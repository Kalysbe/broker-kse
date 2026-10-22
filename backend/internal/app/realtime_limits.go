package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"net/http"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/telemetry"
	"statebroker/sbis/internal/ratelimit"
	"strings"
	"time"
)

func (a *App) InitRealtimeAdmission(ctx context.Context) error {
	a.RealtimeAdmission = ratelimit.NewAdmissionWithCapacity(int64(a.Cfg.RealtimeLimitMaxStates))
	if err := a.refreshRealtimeLimits(ctx); err != nil {
		return err
	}
	telemetry.RegisterRealtimeAdmission(a.RealtimeAdmission)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				check, cancel := context.WithTimeout(ctx, 2*time.Second)
				err := a.refreshRealtimeLimits(check)
				cancel()
				if err != nil && a.Log != nil {
					a.Log.Warn("realtime_policy_refresh_failed", zap.Error(err))
				}
			}
		}
	}()
	return nil
}
func (a *App) loadRealtimePolicies(ctx context.Context) ([]ratelimit.Policy, error) {
	rows, err := a.DB.Query(ctx, `SELECT scope,subject,operation,rate,burst,concurrent,enabled FROM realtime_rate_limits ORDER BY scope,subject,operation`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ratelimit.Policy{}
	for rows.Next() {
		var p ratelimit.Policy
		if err = rows.Scan(&p.Scope, &p.Subject, &p.Operation, &p.Rate, &p.Burst, &p.Concurrent, &p.Enabled); err != nil {
			return nil, err
		}
		if err = p.Validate(); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (a *App) refreshRealtimeLimits(ctx context.Context) error {
	a.RealtimePolicyMu.Lock()
	defer a.RealtimePolicyMu.Unlock()
	policies, err := a.loadRealtimePolicies(ctx)
	if err != nil {
		return err
	}
	if len(policies) == 0 {
		return fmt.Errorf("realtime rate limit configuration empty")
	}
	a.RealtimeAdmission.Replace(policies)
	return nil
}
func (a *App) admitRealtime(p httpx.Principal, method, requestID string) (func(), error) {
	if p.BrokerID == uuid.Nil {
		return nil, status.Error(codes.Unauthenticated, "authenticated broker identity required")
	}
	if a.RealtimeAdmission == nil {
		return nil, status.Error(codes.Unavailable, "realtime admission unavailable")
	}
	release, denied := a.RealtimeAdmission.Acquire("broker", p.BrokerID.String(), "System", method)
	if denied == nil {
		return release, nil
	}
	if denied.Sample && a.Log != nil {
		a.Log.Warn("RATE_LIMIT_EXCEEDED", zap.String("broker_id", p.BrokerID.String()), zap.String("method", method), zap.Int("limit", denied.Policy.Rate), zap.Int("burst", denied.Policy.Burst), zap.Uint64("current_rate", denied.CurrentRate), zap.String("reason", denied.Reason), zap.String("request_id", requestID))
	}
	st := status.New(codes.ResourceExhausted, denied.Error())
	withRetry, err := st.WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(denied.RetryAfter)})
	if err != nil {
		return nil, st.Err()
	}
	return nil, withRetry.Err()
}
func realtimeLimitsAdministrator(p httpx.Principal) bool { return p.HasRole("SUPER_ADMIN") }
func (a *App) handleRealtimeLimits(w http.ResponseWriter, r *http.Request) {
	p, ok := httpx.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized)
		return
	}
	if !realtimeLimitsAdministrator(p) {
		httpx.WriteError(w, r, httpx.ErrForbidden)
		return
	}
	if a.RealtimeAdmission == nil {
		httpx.WriteError(w, r, httpx.E(503, "LIMITER_UNAVAILABLE", "Limiter is unavailable"))
		return
	}
	if r.Method == http.MethodGet {
		policies, err := a.loadRealtimePolicies(r.Context())
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, 200, map[string]any{"policies": policies, "usage": a.RealtimeAdmission.Snapshot()})
		return
	}
	var body struct {
		ratelimit.Policy
		Reason string `json:"reason"`
	}
	if err := decode(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := body.Policy.Validate(); err != nil {
		httpx.WriteError(w, r, httpx.E(400, "INVALID_POLICY", err.Error()))
		return
	}
	allowed := map[string]bool{"*": true, "System": true, "Connect": true, "Subscribe": true, "Unsubscribe": true, "Ping": true, "UnknownCommand": true}
	if (body.Scope != "broker" && body.Scope != "global") || !allowed[body.Operation] || (body.Scope == "global" && body.Operation != "*" && body.Operation != "System") {
		httpx.WriteError(w, r, httpx.E(400, "INVALID_POLICY", "Invalid scope/operation"))
		return
	}
	if len(strings.TrimSpace(body.Reason)) < 3 || len(body.Reason) > 1000 {
		httpx.WriteError(w, r, httpx.E(400, "INVALID_REASON", "Change reason required (3..1000 characters)"))
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(72914517)`); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var broker *uuid.UUID
	if body.Scope == "broker" && body.Subject != "*" {
		bid, e := uuid.Parse(body.Subject)
		if e != nil {
			httpx.WriteError(w, r, httpx.E(400, "INVALID_BROKER", "Broker UUID required"))
			return
		}
		var exists bool
		if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM brokers WHERE id=$1)`, bid).Scan(&exists); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if !exists {
			httpx.WriteError(w, r, httpx.E(400, "INVALID_BROKER", "Unknown broker"))
			return
		}
		broker = &bid
		body.Subject = bid.String()
	}
	var before *string
	if err = tx.QueryRow(r.Context(), `SELECT (SELECT row_to_json(p)::text FROM realtime_rate_limits p WHERE scope=$1 AND subject=$2 AND operation=$3)`, body.Scope, body.Subject, body.Operation).Scan(&before); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO realtime_rate_limits(scope,subject,operation,rate,burst,concurrent,enabled) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(scope,subject,operation) DO UPDATE SET rate=EXCLUDED.rate,burst=EXCLUDED.burst,concurrent=EXCLUDED.concurrent,enabled=EXCLUDED.enabled,updated_at=now()`, body.Scope, body.Subject, body.Operation, body.Rate, body.Burst, body.Concurrent, body.Enabled); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var old any
	if before != nil {
		old = json.RawMessage(*before)
	}
	if err = a.Audit.WriteTx(r.Context(), tx, "REALTIME_RATE_LIMIT_UPDATED", "realtime_rate_limit", broker, old, map[string]any{"policy": body.Policy, "reason": body.Reason}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = a.refreshRealtimeLimits(r.Context()); err != nil {
		httpx.WriteError(w, r, httpx.E(503, "REFRESH_PENDING", "Policy saved; refresh pending"))
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"policy": body.Policy})
}
