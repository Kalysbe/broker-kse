package app

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"statebroker/sbis/internal/gateway"
	"statebroker/sbis/internal/platform/hash"
	"statebroker/sbis/internal/platform/httpx"
)

func (a *App) GatewayAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			gateway.Write(w, http.StatusBadRequest, "invalid body", map[string]any{})
			return
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(rawBody))

		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			gateway.Write(w, http.StatusUnauthorized, "missing or invalid token", map[string]any{})
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		if !gateway.IsAPIToken(token) {
			gateway.Write(w, http.StatusUnauthorized, "missing or invalid token", map[string]any{})
			return
		}

		var (
			tokenID, brokerID                uuid.UUID
			perms, whitelist                 []string
			rps, orderRPS                    int
			hmacReq                          bool
			expires, revoked                 *time.Time
			quikFirm, firmName, brokerStatus string
		)
		err = a.DB.QueryRow(r.Context(), `
			SELECT t.id, t.broker_id, t.permissions, COALESCE(t.ip_whitelist::text[], '{}'),
			       t.rate_limit_rps, t.order_rate_limit_rps,
			       t.hmac_required, t.expires_at, t.revoked_at,
			       COALESCE(NULLIF(t.quik_firm,''), b.quik_firm),
			       COALESCE(NULLIF(b.firm_name,''), b.name),
			       b.status
			FROM api_tokens t
			JOIN brokers b ON b.id = t.broker_id
			WHERE t.token_hash=$1`, hash.SHA256Hex(token)).
			Scan(&tokenID, &brokerID, &perms, &whitelist, &rps, &orderRPS, &hmacReq, &expires, &revoked, &quikFirm, &firmName, &brokerStatus)
		if err != nil {
			gateway.Write(w, http.StatusUnauthorized, "missing or invalid token", map[string]any{})
			return
		}
		if revoked != nil || (expires != nil && time.Now().After(*expires)) {
			gateway.Write(w, http.StatusUnauthorized, "expired or revoked token", map[string]any{})
			return
		}
		if brokerStatus != "ACTIVE" {
			gateway.Write(w, http.StatusUnauthorized, "expired or revoked token", map[string]any{})
			return
		}

		ip := httpx.ClientIP(r)
		if len(whitelist) > 0 && !ipAllowed(ip, whitelist) {
			gateway.Write(w, http.StatusForbidden, "ip not allowed", map[string]any{})
			return
		}

		if a.Limiter == nil {
			a.Limiter = gateway.NewLimiter()
		}
		if !a.Limiter.Allow(tokenID, "all", rps) {
			gateway.Write(w, http.StatusTooManyRequests, "rate limit exceeded", map[string]any{})
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/orders") && !strings.Contains(r.URL.Path, "/cancel") {
			if !a.Limiter.Allow(tokenID, "orders", orderRPS) {
				gateway.Write(w, http.StatusTooManyRequests, "order create rate limit exceeded", map[string]any{})
				return
			}
		}

		needHMAC := a.Cfg.HMACRequired || hmacReq || r.Header.Get("X-Signature") != ""
		if needHMAC {
			if err := gateway.Verify(token, r.Header.Get("X-Timestamp"), r.Header.Get("X-Nonce"),
				r.Method, r.URL.Path, string(rawBody), r.Header.Get("X-Signature"), time.Now()); err != nil {
				gateway.Write(w, http.StatusUnauthorized, "bad HMAC signature", map[string]any{})
				return
			}
			tag, err := a.DB.Exec(r.Context(), `INSERT INTO hmac_nonces (token_id, nonce) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
				tokenID, r.Header.Get("X-Nonce"))
			if err != nil || tag.RowsAffected() == 0 {
				gateway.Write(w, http.StatusUnauthorized, "bad HMAC signature", map[string]any{})
				return
			}
		}

		scope := gateway.ScopeFor(r)
		permSet := map[string]struct{}{}
		for _, p := range perms {
			permSet[p] = struct{}{}
		}
		if scope != "" {
			if _, ok := permSet[scope]; !ok {
				gateway.Write(w, http.StatusForbidden, "insufficient permissions", map[string]any{"scope": scope})
				return
			}
		}

		_, _ = a.DB.Exec(r.Context(), `UPDATE api_tokens SET last_used_at=now() WHERE id=$1`, tokenID)
		p := httpx.Principal{
			BrokerID: brokerID, TokenID: &tokenID, Permissions: permSet,
			IP: ip, QuikFirm: quikFirm, FirmName: firmName, IsAPIToken: true, HMACRequired: hmacReq,
		}
		next.ServeHTTP(w, r.WithContext(httpx.WithPrincipal(r.Context(), p)))
	})
}

func ipAllowed(ip string, cidrs []string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if !strings.Contains(c, "/") {
			if other := net.ParseIP(strings.Trim(c, "{}")); other != nil && other.Equal(parsed) {
				return true
			}
			continue
		}
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			continue
		}
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

func (a *App) handleSwagger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com; style-src 'self' 'unsafe-inline' https://unpkg.com; img-src 'self' data:; connect-src 'self'")
	_, _ = w.Write([]byte(gateway.SwaggerHTML))
}

func (a *App) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(gateway.OpenAPI)
}

func (a *App) handleGatewayReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ready := true
	checks := map[string]any{}
	if err := a.DB.Ping(ctx); err != nil {
		ready = false
		checks["database"] = "down"
	} else {
		checks["database"] = "up"
	}
	fixOK := a.Adapters.HasHealthyKind("FIX")
	checks["fix_adapter"] = fixOK
	if !fixOK {
		ready = false
	}
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	gateway.JSON(w, status, map[string]any{"ready": ready, "checks": checks})
}
