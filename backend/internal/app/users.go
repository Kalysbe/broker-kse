package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"statebroker/sbis/internal/platform/hash"
	"statebroker/sbis/internal/platform/httpx"
)

func (a *App) handleGetUser(w http.ResponseWriter, r *http.Request) {
	idv, err := chiID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	item, err := a.loadUser(r.Context(), idv)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	httpx.WriteJSON(w, 200, item)
}

type userUpdate struct {
	Email     string   `json:"email"`
	Phone     string   `json:"phone"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Password  string   `json:"password"`
	Status    string   `json:"status"`
	RoleCodes []string `json:"roles"`
	ClientID  string   `json:"client_id"`
}

func (a *App) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	idv, err := chiID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req userUpdate
	if err := decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r.Context())
	cid, err := parseOptionalUUID(req.ClientID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrValidation)
		return
	}
	if err := a.validateClientLink(req.RoleCodes, cid); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.guardRoleAssign(p, req.RoleCodes); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if status == "" {
		status = "ACTIVE"
	}
	if status != "ACTIVE" && status != "BLOCKED" && status != "SUSPENDED" {
		httpx.WriteError(w, r, httpx.E(422, "VALIDATION_ERROR", "Invalid status"))
		return
	}
	var current string
	if err := a.DB.QueryRow(r.Context(), `SELECT status FROM users WHERE id=$1 AND status <> 'DELETED'`, idv).Scan(&current); err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	if req.Password != "" {
		ph, err := hash.Password(req.Password)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if _, err := a.DB.Exec(r.Context(), `
			UPDATE users SET email=$2, phone=$3, first_name=$4, last_name=$5, status=$6, client_id=$7, password_hash=$8, version=version+1
			WHERE id=$1`, idv, req.Email, nullIfEmpty(req.Phone), req.FirstName, req.LastName, status, cid, ph); err != nil {
			httpx.WriteError(w, r, httpx.ErrConflict)
			return
		}
		_, _ = a.DB.Exec(r.Context(), `UPDATE sessions SET revoked_at=now(), revoke_reason='password_change' WHERE user_id=$1 AND revoked_at IS NULL`, idv)
	} else {
		if _, err := a.DB.Exec(r.Context(), `
			UPDATE users SET email=$2, phone=$3, first_name=$4, last_name=$5, status=$6, client_id=$7, version=version+1
			WHERE id=$1`, idv, req.Email, nullIfEmpty(req.Phone), req.FirstName, req.LastName, status, cid); err != nil {
			httpx.WriteError(w, r, httpx.ErrConflict)
			return
		}
	}
	if err := a.replaceUserRoles(r.Context(), idv, req.RoleCodes); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if status != "ACTIVE" {
		_, _ = a.DB.Exec(r.Context(), `UPDATE sessions SET revoked_at=now(), revoke_reason='status_'||$2 WHERE user_id=$1 AND revoked_at IS NULL`, idv, status)
	}
	a.Audit.Write(r.Context(), "USER_UPDATED", "user", &idv, nil, map[string]any{"roles": req.RoleCodes, "status": status})
	item, err := a.loadUser(r.Context(), idv)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, item)
}

func (a *App) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	idv, err := chiID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r.Context())
	if p.UserID == idv {
		httpx.WriteError(w, r, httpx.E(422, "VALIDATION_ERROR", "Cannot delete the current user"))
		return
	}
	tag, err := a.DB.Exec(r.Context(), `UPDATE users SET status='DELETED', version=version+1 WHERE id=$1 AND status <> 'DELETED'`, idv)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	_, _ = a.DB.Exec(r.Context(), `UPDATE sessions SET revoked_at=now(), revoke_reason='deleted' WHERE user_id=$1 AND revoked_at IS NULL`, idv)
	a.Audit.Write(r.Context(), "USER_DELETED", "user", &idv, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) loadUser(ctx context.Context, userID uuid.UUID) (map[string]any, error) {
	var (
		idv                      uuid.UUID
		login, email, fn, ln, st string
		phone                    *string
		totp                     bool
		lastAt                   *time.Time
		lastIP                   *string
		clientID                 *uuid.UUID
		clientNo                 *string
		roles                    []string
	)
	err := a.DB.QueryRow(ctx, `
		SELECT u.id, u.login, u.email, u.phone, u.first_name, u.last_name, u.status, u.totp_enabled,
		       u.last_login_at, u.last_login_ip::text, u.client_id, c.client_no,
		       COALESCE(array_agg(r.code) FILTER (WHERE r.code IS NOT NULL), '{}')
		FROM users u
		LEFT JOIN clients c ON c.id = u.client_id
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE u.id=$1 AND u.status <> 'DELETED'
		GROUP BY u.id, c.client_no`, userID).
		Scan(&idv, &login, &email, &phone, &fn, &ln, &st, &totp, &lastAt, &lastIP, &clientID, &clientNo, &roles)
	if err != nil {
		return nil, err
	}
	if roles == nil {
		roles = []string{}
	}
	return map[string]any{
		"id": idv, "login": login, "email": email, "phone": phone,
		"first_name": fn, "last_name": ln, "status": st, "totp_enabled": totp,
		"last_login_at": lastAt, "last_login_ip": lastIP,
		"client_id": clientID, "client_no": clientNo, "roles": roles,
	}, nil
}

func (a *App) replaceUserRoles(ctx context.Context, userID uuid.UUID, codes []string) error {
	if _, err := a.DB.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		if _, err := a.DB.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE code=$2`, userID, code); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) guardRoleAssign(actor httpx.Principal, codes []string) error {
	for _, c := range codes {
		if strings.EqualFold(strings.TrimSpace(c), "SUPER_ADMIN") && !actor.HasRole("SUPER_ADMIN") {
			return httpx.ErrForbidden
		}
	}
	return nil
}

func (a *App) validateClientLink(roles []string, clientID *uuid.UUID) error {
	if hasCode(roles, "CLIENT") && clientID == nil {
		return httpx.E(422, "VALIDATION_ERROR", "CLIENT role requires a linked client")
	}
	return nil
}

func hasCode(codes []string, want string) bool {
	for _, c := range codes {
		if strings.EqualFold(strings.TrimSpace(c), want) {
			return true
		}
	}
	return false
}

func parseOptionalUUID(s string) (*uuid.UUID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	idv, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &idv, nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func (a *App) requireAccountAccess(ctx context.Context, p httpx.Principal, accountID uuid.UUID) error {
	if p.ClientID == nil {
		return nil
	}
	var cid uuid.UUID
	if err := a.DB.QueryRow(ctx, `SELECT client_id FROM trading_accounts WHERE id=$1`, accountID).Scan(&cid); err != nil {
		return httpx.ErrNotFound
	}
	if cid != *p.ClientID {
		return httpx.ErrForbidden
	}
	return nil
}

func (a *App) requireOrderAccess(ctx context.Context, p httpx.Principal, orderID uuid.UUID) error {
	if p.ClientID == nil {
		return nil
	}
	var cid uuid.UUID
	if err := a.DB.QueryRow(ctx, `SELECT client_id FROM orders WHERE id=$1`, orderID).Scan(&cid); err != nil {
		return httpx.ErrNotFound
	}
	if cid != *p.ClientID {
		return httpx.ErrForbidden
	}
	return nil
}
