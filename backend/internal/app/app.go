package app

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"statebroker/sbis/internal/adapters"
	"statebroker/sbis/internal/adapters/kfb"
	"statebroker/sbis/internal/audit"
	"statebroker/sbis/internal/gateway"
	"statebroker/sbis/internal/platform/config"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/storage"
	"statebroker/sbis/internal/ratelimit"
)

type App struct {
	Cfg                config.Config
	DB                 *pgxpool.Pool
	Redis              *redis.Client
	Log                *zap.Logger
	Audit              audit.Logger
	Adapters           *adapters.Manager
	KFB                *kfb.Client
	Store              storage.Store
	HouseID            uuid.UUID
	Limiter            *gateway.Limiter
	LoadMu             sync.Mutex
	LoadRun            *loadTestRun
	KFBSettings        kfbSettingsState
	KFBStreamError     atomic.Pointer[string]
	KFBStreamConnected atomic.Bool
	KFBLastReconcile   atomic.Int64
	KFBSyncMu          sync.Mutex
	RealtimeAdmission  *ratelimit.Admission
	RealtimePolicyMu   sync.Mutex
}

func (a *App) perm(code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := httpx.PrincipalFrom(r.Context())
			if !ok {
				httpx.WriteError(w, r, httpx.ErrUnauthorized)
				return
			}
			if !p.Has(code) {
				httpx.WriteError(w, r, httpx.ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (a *App) loadHouse(ctx context.Context) error {
	return a.DB.QueryRow(ctx, `SELECT id FROM brokers WHERE code='HOUSE'`).Scan(&a.HouseID)
}

func decode[T any](r *http.Request, dst *T) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return httpx.DecodeJSONError(err)
	}
	return nil
}

func chiID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, httpx.E(http.StatusBadRequest, "INVALID_ID", "Invalid identifier")
	}
	return id, nil
}

func (a *App) permissionsForUser(ctx context.Context, userID uuid.UUID) (roles []string, perms map[string]struct{}, err error) {
	rows, err := a.DB.Query(ctx, `
		SELECT r.code, p.permission
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		JOIN role_permissions p ON p.role_id = r.id
		WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	roleSet := map[string]struct{}{}
	perms = map[string]struct{}{}
	for rows.Next() {
		var rc, pc string
		if err := rows.Scan(&rc, &pc); err != nil {
			return nil, nil, err
		}
		roleSet[rc] = struct{}{}
		perms[pc] = struct{}{}
	}
	for r := range roleSet {
		roles = append(roles, r)
	}
	return roles, perms, rows.Err()
}

func (a *App) heartbeat(ctx context.Context, name, status string) {
	_, _ = a.DB.Exec(ctx, `
		INSERT INTO worker_heartbeats (worker_name, status, last_seen_at)
		VALUES ($1,$2,now())
		ON CONFLICT (worker_name) DO UPDATE SET status=EXCLUDED.status, last_seen_at=now()`, name, status)
}
