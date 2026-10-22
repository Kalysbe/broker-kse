package app

import (
	"net/http"
	"statebroker/sbis/internal/platform/httpx"
)

// Optional viewing mode also stops the background routing/cancellation loop.
// Authentication remains available; all financial mutations are rejected here.
func (a *App) exchangeReadOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Method == http.MethodPost && (r.URL.Path == "/v1/auth/login" || r.URL.Path == "/v1/auth/refresh" || r.URL.Path == "/v1/auth/logout")
		if a.Cfg.ExchangeReadOnly && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !auth {
			httpx.WriteError(w, r, httpx.E(503, "EXCHANGE_READ_ONLY", "Подключение работает в режиме просмотра данных КФБ"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
