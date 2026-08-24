package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"statebroker/sbis/internal/platform/authn"
	"statebroker/sbis/internal/platform/hash"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/id"
)

type loginReq struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	TOTP     string `json:"totp"`
}

type tokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	User         any    `json:"user"`
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var (
		userID                     uuid.UUID
		status                     string
		phash                      string
		totpOn                     bool
		fname, lname, email, login string
	)
	err := a.DB.QueryRow(r.Context(), `
		SELECT id, status, password_hash, totp_enabled, first_name, last_name, email, login
		FROM users WHERE login=$1`, strings.TrimSpace(req.Login)).
		Scan(&userID, &status, &phash, &totpOn, &fname, &lname, &email, &login)
	if err != nil || !hash.VerifyPassword(req.Password, phash) {
		httpx.WriteError(w, r, httpx.E(http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid login or password"))
		a.securityEvent(r.Context(), nil, "LOGIN_FAILED", httpx.ClientIP(r), r.UserAgent())
		return
	}
	if status != "ACTIVE" {
		httpx.WriteError(w, r, httpx.E(http.StatusForbidden, "USER_INACTIVE", "User is not active"))
		return
	}
	if totpOn {
		var secret []byte
		_ = a.DB.QueryRow(r.Context(), `SELECT secret_enc FROM totp_secrets WHERE user_id=$1 AND confirmed_at IS NOT NULL`, userID).Scan(&secret)
		if req.TOTP == "" || !totp.Validate(req.TOTP, string(secret)) {
			httpx.WriteError(w, r, httpx.E(http.StatusUnauthorized, "TOTP_REQUIRED", "Valid TOTP code required"))
			return
		}
	}
	pair, err := a.issueSession(r.Context(), userID, httpx.ClientIP(r), r.UserAgent())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	_, _ = a.DB.Exec(r.Context(), `UPDATE users SET last_login_at=now(), last_login_ip=$2, failed_logins=0 WHERE id=$1`, userID, httpx.ClientIP(r))
	a.Audit.Write(r.Context(), "USER_LOGIN", "user", &userID, nil, map[string]string{"login": login})
	pair.User = map[string]any{"id": userID, "login": login, "email": email, "first_name": fname, "last_name": lname}
	httpx.WriteJSON(w, http.StatusOK, pair)
}

func (a *App) issueSession(ctx context.Context, userID uuid.UUID, ip, ua string) (*tokenPair, error) {
	roles, _, err := a.permissionsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	sid := id.New()
	if _, err := a.DB.Exec(ctx, `INSERT INTO sessions (id, user_id, ip, user_agent) VALUES ($1,$2,$3,$4)`, sid, userID, ip, ua); err != nil {
		return nil, err
	}
	raw, err := hash.RandomToken(32)
	if err != nil {
		return nil, err
	}
	exp := time.Now().UTC().Add(a.Cfg.RefreshTTL)
	if _, err := a.DB.Exec(ctx, `INSERT INTO refresh_tokens (id, session_id, user_id, token_hash, expires_at) VALUES ($1,$2,$3,$4,$5)`,
		id.New(), sid, userID, hash.SHA256Hex(raw), exp); err != nil {
		return nil, err
	}
	access, err := authn.IssueAccess(a.Cfg.JWTSecret, userID, sid, a.HouseID, roles, a.Cfg.AccessTTL)
	if err != nil {
		return nil, err
	}
	return &tokenPair{AccessToken: access, RefreshToken: raw, ExpiresIn: int(a.Cfg.AccessTTL.Seconds()), TokenType: "Bearer"}, nil
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (a *App) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h := hash.SHA256Hex(req.RefreshToken)
	var userID, sid, tid uuid.UUID
	var exp time.Time
	var revoked *time.Time
	err := a.DB.QueryRow(r.Context(), `
		SELECT id, user_id, session_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash=$1`, h).
		Scan(&tid, &userID, &sid, &exp, &revoked)
	if err != nil || revoked != nil || time.Now().After(exp) {
		httpx.WriteError(w, r, httpx.ErrUnauthorized)
		return
	}
	_, _ = a.DB.Exec(r.Context(), `UPDATE refresh_tokens SET revoked_at=now() WHERE id=$1`, tid)
	pair, err := a.issueSession(r.Context(), userID, httpx.ClientIP(r), r.UserAgent())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, pair)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	_, _ = a.DB.Exec(r.Context(), `UPDATE sessions SET revoked_at=now(), revoke_reason='logout' WHERE id=$1`, p.SessionID)
	_, _ = a.DB.Exec(r.Context(), `UPDATE refresh_tokens SET revoked_at=now() WHERE session_id=$1`, p.SessionID)
	a.Audit.Write(r.Context(), "USER_LOGOUT", "session", &p.SessionID, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	var login, email, fname, lname, status string
	var totpOn bool
	var phone, lastIP *string
	var lastLogin *time.Time
	if err := a.DB.QueryRow(r.Context(), `
		SELECT login, email, phone, first_name, last_name, status, totp_enabled, last_login_at, last_login_ip::text
		FROM users WHERE id=$1`, p.UserID).
		Scan(&login, &email, &phone, &fname, &lname, &status, &totpOn, &lastLogin, &lastIP); err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	perms := make([]string, 0, len(p.Permissions))
	for k := range p.Permissions {
		perms = append(perms, k)
	}
	var broker any
	if p.BrokerID != uuid.Nil {
		var code, name, st string
		var license, firm, quik *string
		if err := a.DB.QueryRow(r.Context(), `
			SELECT code, name, status, license_no, firm_name, NULLIF(quik_firm, '')
			FROM brokers WHERE id=$1`, p.BrokerID).
			Scan(&code, &name, &st, &license, &firm, &quik); err == nil {
			broker = map[string]any{
				"id": p.BrokerID, "code": code, "name": name, "status": st,
				"license_no": license, "firm_name": firm, "quik_firm": quik,
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id": p.UserID, "login": login, "email": email, "phone": phone,
		"first_name": fname, "last_name": lname,
		"status": status, "totp_enabled": totpOn, "roles": p.Roles, "permissions": perms,
		"client_id": p.ClientID, "last_login_at": lastLogin, "last_login_ip": lastIP,
		"broker": broker,
	})
}

type changePasswordReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	var req changePasswordReq
	if err := decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := requirePassword(req.NewPassword); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.CurrentPassword == req.NewPassword {
		httpx.WriteError(w, r, httpx.E(http.StatusUnprocessableEntity, "SAME_PASSWORD", "New password must differ from the current one"))
		return
	}
	var phash string
	if err := a.DB.QueryRow(r.Context(), `SELECT password_hash FROM users WHERE id=$1`, p.UserID).Scan(&phash); err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	if !hash.VerifyPassword(req.CurrentPassword, phash) {
		httpx.WriteError(w, r, httpx.E(http.StatusUnprocessableEntity, "INVALID_CURRENT_PASSWORD", "Current password is incorrect"))
		return
	}
	ph, err := hash.Password(req.NewPassword)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := a.DB.Exec(r.Context(), `UPDATE users SET password_hash=$2, version=version+1 WHERE id=$1`, p.UserID, ph); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	_, _ = a.DB.Exec(r.Context(), `
		UPDATE sessions SET revoked_at=now(), revoke_reason='password_change'
		WHERE user_id=$1 AND id<>$2 AND revoked_at IS NULL`, p.UserID, p.SessionID)
	_, _ = a.DB.Exec(r.Context(), `
		UPDATE refresh_tokens SET revoked_at=now()
		WHERE user_id=$1 AND session_id<>$2 AND revoked_at IS NULL`, p.UserID, p.SessionID)
	a.Audit.Write(r.Context(), "PASSWORD_CHANGED", "user", &p.UserID, nil, nil)
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type passwordResetReq struct {
	Email string `json:"email"`
}

func (a *App) handlePasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	var req passwordResetReq
	_ = decode(r, &req)
	var userID uuid.UUID
	err := a.DB.QueryRow(r.Context(), `SELECT id FROM users WHERE email=$1 AND status='ACTIVE'`, req.Email).Scan(&userID)
	if err == nil {
		raw, _ := hash.RandomToken(32)
		_, _ = a.DB.Exec(r.Context(), `INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at) VALUES ($1,$2,$3,now()+interval '30 minutes')`,
			id.New(), userID, hash.SHA256Hex(raw))
		a.Log.Info("password_reset_issued")
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

type passwordResetConfirm struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (a *App) handlePasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	var req passwordResetConfirm
	if err := decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := requirePassword(req.NewPassword); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var userID, tid uuid.UUID
	var used *time.Time
	var exp time.Time
	err := a.DB.QueryRow(r.Context(), `SELECT id, user_id, used_at, expires_at FROM password_reset_tokens WHERE token_hash=$1`, hash.SHA256Hex(req.Token)).
		Scan(&tid, &userID, &used, &exp)
	if err != nil || used != nil || time.Now().After(exp) {
		httpx.WriteError(w, r, httpx.ErrUnauthorized)
		return
	}
	ph, err := hash.Password(req.NewPassword)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	_, _ = a.DB.Exec(r.Context(), `UPDATE password_reset_tokens SET used_at=now() WHERE id=$1`, tid)
	_, _ = a.DB.Exec(r.Context(), `UPDATE users SET password_hash=$2, version=version+1 WHERE id=$1`, userID, ph)
	_, _ = a.DB.Exec(r.Context(), `UPDATE sessions SET revoked_at=now(), revoke_reason='password_reset' WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	a.Audit.Write(r.Context(), "PASSWORD_RESET", "user", &userID, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			httpx.WriteError(w, r, httpx.ErrUnauthorized)
			return
		}
		claims, err := authn.ParseAccess(a.Cfg.JWTSecret, strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrUnauthorized)
			return
		}
		uid, _ := uuid.Parse(claims.Subject)
		sid, _ := uuid.Parse(claims.SessionID)
		bid, _ := uuid.Parse(claims.BrokerID)
		var revoked *time.Time
		_ = a.DB.QueryRow(r.Context(), `SELECT revoked_at FROM sessions WHERE id=$1`, sid).Scan(&revoked)
		if revoked != nil {
			httpx.WriteError(w, r, httpx.ErrUnauthorized)
			return
		}
		_, perms, err := a.permissionsForUser(r.Context(), uid)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrUnauthorized)
			return
		}
		var clientID *uuid.UUID
		_ = a.DB.QueryRow(r.Context(), `SELECT client_id FROM users WHERE id=$1`, uid).Scan(&clientID)
		p := httpx.Principal{
			UserID: uid, SessionID: sid, BrokerID: bid, ClientID: clientID, Roles: claims.Roles,
			Permissions: perms, IP: httpx.ClientIP(r),
		}
		next.ServeHTTP(w, r.WithContext(httpx.WithPrincipal(r.Context(), p)))
	})
}

func (a *App) securityEvent(ctx context.Context, userID *uuid.UUID, typ, ip, ua string) {
	_, _ = a.DB.Exec(ctx, `INSERT INTO security_events (id, user_id, event_type, ip, user_agent) VALUES ($1,$2,$3,$4,$5)`,
		id.New(), userID, typ, ip, ua)
}
