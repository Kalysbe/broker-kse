package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"statebroker/sbis/internal/adapters"
	"statebroker/sbis/internal/platform/httpx"
)

func (a *App) handleKFBStatus(w http.ResponseWriter, r *http.Request) {
	if a.KFB == nil {
		httpx.WriteJSON(w, 200, map[string]any{
			"connected": false, "health": "OFFLINE",
			"error":       "KFB adapter is not configured",
			"instruments": []any{}, "accounts": []any{}, "positions": []any{}, "firm": map[string]any{},
		})
		return
	}
	if r.URL.Query().Get("refresh") == "1" || r.Method == http.MethodPost {
		snap := a.KFB.Refresh(r.Context())
		a.syncAdapterRow(r.Context(), "KFB", "EXCHANGE", "Кыргызская фондовая биржа", snap.Health, 20)
		httpx.WriteJSON(w, 200, snap)
		return
	}
	snap := a.KFB.Snapshot()
	if snap.LastCheckAt == nil {
		snap = a.KFB.Refresh(r.Context())
		a.syncAdapterRow(r.Context(), "KFB", "EXCHANGE", "Кыргызская фондовая биржа", snap.Health, 20)
	}
	httpx.WriteJSON(w, 200, snap)
}

func (a *App) handleKFBHealth(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, 200, a.kfbHealth(r.Context()))
}

func (a *App) kfbHealth(ctx context.Context) map[string]any {
	if a.KFB == nil {
		return map[string]any{"connected": false, "health": "OFFLINE", "error": "KFB adapter is not configured"}
	}
	snap := a.KFB.Snapshot()
	stale := snap.LastCheckAt == nil || time.Since(*snap.LastCheckAt) > 12*time.Second
	if stale && a.KFB.Configured() {
		_ = a.KFB.HealthCheck(ctx)
		snap = a.KFB.Snapshot()
	}
	out := map[string]any{
		"connected":          snap.Connected,
		"health":             snap.Health,
		"environment":        connectionEnvironment(a.KFB.ConnectionSettings()),
		"read_only":          a.Cfg.ExchangeReadOnly,
		"realtime_connected": a.KFBStreamConnected.Load(),
	}
	if snap.Health == "" {
		out["health"] = "OFFLINE"
	}
	if snap.Error != "" {
		out["error"] = snap.Error
	}
	if snap.LastCheckAt != nil {
		out["last_check_at"] = snap.LastCheckAt.UTC()
	}
	return out
}

func (a *App) handleListPositions(w http.ResponseWriter, r *http.Request) {
	empty := []map[string]any{}
	if a.KFB == nil || !a.KFB.Configured() {
		httpx.WriteJSON(w, 200, map[string]any{
			"items": empty, "error": "KFB gateway URL or token is not configured",
		})
		return
	}
	pos, err := a.kfbPositions(r.Context())
	if err != nil {
		httpx.WriteJSON(w, 200, map[string]any{"items": empty, "error": err.Error()})
		return
	}
	items := pos.Positions
	if items == nil {
		items = empty
	}
	p, _ := httpx.PrincipalFrom(r.Context())
	if p.ClientID != nil {
		code, err := a.clientCodeForUser(r.Context(), *p.ClientID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		items = filterPositionRows(items, code)
	}
	httpx.WriteJSON(w, 200, map[string]any{
		"items":    items,
		"taken_at": pos.TakenAt,
		"note":     pos.Note,
	})
}

func (a *App) clientCodeForUser(ctx context.Context, clientID uuid.UUID) (string, error) {
	if a.DB == nil {
		return "", nil
	}
	var code string
	err := a.DB.QueryRow(ctx, `SELECT COALESCE(NULLIF(TRIM(client_code), ''), '') FROM clients WHERE id=$1`, clientID).Scan(&code)
	if err != nil {
		return "", httpx.ErrNotFound
	}
	return strings.TrimSpace(code), nil
}

func positionClientCode(row map[string]any) string {
	if row == nil {
		return ""
	}
	v, ok := row["client_code"]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func filterPositionRows(items []map[string]any, clientCode string) []map[string]any {
	want := strings.TrimSpace(clientCode)
	if want == "" {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(items))
	for _, row := range items {
		if strings.EqualFold(positionClientCode(row), want) {
			out = append(out, row)
		}
	}
	return out
}

func (a *App) handleDepositoryStatus(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, 200, map[string]any{
		"connected": false,
		"health":    "OFFLINE",
		"name":      "Центральный депозитарий",
		"note":      "Подключение депозитария будет на следующем этапе. Сейчас активна только интеграция с КФБ.",
	})
}

func (a *App) syncAdapterRow(ctx context.Context, code, kind, name, status string, weight int) {
	if a.DB == nil {
		return
	}
	if status == "" {
		status = string(adapters.Offline)
	}
	_, _ = a.DB.Exec(ctx, `
		INSERT INTO adapters (code, kind, name, status, weight, last_health_at)
		VALUES ($1,$2,$3,$4,$5,now())
		ON CONFLICT (code) DO UPDATE SET
		  kind=EXCLUDED.kind, name=EXCLUDED.name, status=EXCLUDED.status,
		  weight=EXCLUDED.weight, last_health_at=now()`,
		code, kind, name, status, weight)
}
