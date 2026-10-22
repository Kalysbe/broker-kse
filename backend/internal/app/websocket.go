package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/metadata"
	"statebroker/sbis/internal/outbox"
	"statebroker/sbis/internal/platform/httpx"
)

type wsCommand struct {
	Type    string            `json:"type"`
	ID      string            `json:"id"`
	Token   string            `json:"token"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Body    json.RawMessage   `json:"body"`
	Headers map[string]string `json:"headers"`
	Watch   bool              `json:"watch"`
}

func wsPathAllowed(path string) bool {
	u, err := url.ParseRequestURI(path)
	return err == nil && u.Host == "" && strings.HasPrefix(u.Path, "/v1/") &&
		!strings.Contains(u.Path, "..") && !strings.HasPrefix(u.Path, "/v1/realtime/") &&
		!strings.HasPrefix(u.Path, "/v1/auth/login") && !strings.HasPrefix(u.Path, "/v1/auth/refresh") &&
		!strings.HasPrefix(u.Path, "/v1/auth/password-reset")
}

func wsWatchAllowed(path string) bool {
	return path == "/v1/positions" || path == "/v1/market" || path == "/v1/orders" ||
		(strings.HasPrefix(path, "/v1/market/") && strings.HasSuffix(path, "/book"))
}

func (a *App) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{CheckOrigin: func(req *http.Request) bool {
		origin := req.Header.Get("Origin")
		if origin == "" {
			return true
		}
		for _, allowed := range a.Cfg.CORSOrigins {
			if origin == strings.TrimSpace(allowed) {
				return true
			}
		}
		u, err := url.Parse(origin)
		return err == nil && u.Host == req.Host
	}}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1 << 20)
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var auth wsCommand
	if conn.ReadJSON(&auth) != nil || auth.Type != "auth" {
		return
	}
	ctx, cancel := context.WithCancel(metadata.NewIncomingContext(r.Context(), metadata.Pairs("authorization", "Bearer "+auth.Token)))
	defer cancel()
	server := &realtimeServer{app: a}
	p, err := server.authenticate(ctx)
	write := func(value any) error {
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		return conn.WriteJSON(value)
	}
	if err != nil {
		_ = write(map[string]any{"type": "unauthorized"})
		return
	}
	release, err := a.admitRealtime(p, "Connect", httpx.RequestIDFrom(r.Context()))
	if err != nil {
		_ = write(map[string]any{"type": "error", "message": "Connection limit exceeded"})
		return
	}
	defer release()
	_ = conn.SetReadDeadline(time.Time{})
	if write(map[string]any{"type": "ready", "upstream_connected": a.KFBStreamConnected.Load()}) != nil {
		return
	}
	commands := make(chan wsCommand, 8)
	go func() {
		defer cancel()
		for {
			var cmd wsCommand
			if conn.ReadJSON(&cmd) != nil {
				return
			}
			select {
			case commands <- cmd:
			case <-ctx.Done():
				return
			}
		}
	}()
	events := make(chan redis.XMessage, 32)
	errs := make(chan error, 1)
	if a.Redis != nil {
		lastID := "0-0"
		if tail, e := a.Redis.XRevRangeN(ctx, outbox.DefaultStream, "+", "-", 1).Result(); e == nil && len(tail) > 0 {
			lastID = tail[0].ID
		}
		go server.readEvents(ctx, events, errs, lastID)
	}
	// Existing routing retains permission checks, validation and client filters.
	router := a.Router()
	invoke := func(cmd wsCommand) (int, json.RawMessage) {
		if !wsPathAllowed(cmd.Path) {
			return 400, json.RawMessage(`{"error":{"code":"INVALID_PATH","message":"Invalid WebSocket request path"}}`)
		}
		reqCtx, stop := context.WithTimeout(ctx, 30*time.Second)
		defer stop()
		reqCtx = context.WithValue(reqCtx, chi.RouteCtxKey, chi.NewRouteContext())
		req, e := http.NewRequestWithContext(reqCtx, cmd.Method, cmd.Path, bytes.NewReader(cmd.Body))
		if e != nil {
			return 400, json.RawMessage(`{"error":{"code":"INVALID_REQUEST"}}`)
		}
		req.Header.Set("Authorization", "Bearer "+auth.Token)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = r.RemoteAddr
		for _, name := range []string{"Idempotency-Key", "X-Request-ID", "X-Trace-ID"} {
			if v := cmd.Headers[name]; v != "" {
				req.Header.Set(name, v)
			}
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		body := rec.Body.Bytes()
		if len(body) == 0 {
			body = []byte("null")
		}
		if !json.Valid(body) {
			body = []byte(`{"error":{"code":"INVALID_RESPONSE"}}`)
		}
		return rec.Code, body
	}
	watches := map[string]bool{}
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	flush := time.NewTicker(250 * time.Millisecond)
	defer flush.Stop()
	dirty := false
	dirtyPaths := map[string]bool{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-errs:
			return
		case <-ticker.C:
			current, err := server.authenticate(ctx)
			if err != nil {
				_ = write(map[string]any{"type": "unauthorized"})
				return
			}
			p = current
			if write(map[string]any{"type": "connection", "upstream_connected": a.KFBStreamConnected.Load()}) != nil {
				return
			}
			dirty = true
			for path := range watches {
				dirtyPaths[path] = true
			}
		case <-flush.C:
			if !dirty {
				continue
			}
			dirty = false
			current, err := server.authenticate(ctx)
			if err != nil {
				_ = write(map[string]any{"type": "unauthorized"})
				return
			}
			p = current
			for path := range dirtyPaths {
				delete(dirtyPaths, path)
				if !watches[path] {
					continue
				}
				status, body := invoke(wsCommand{Method: "GET", Path: path})
				if write(map[string]any{"type": "snapshot", "path": path, "status": status, "data": body}) != nil {
					return
				}
			}
		case raw := <-events:
			event := redisEvent(raw)
			if !topicAllowed(p, event.Topic) {
				continue
			}
			dirty = true
			for path := range watches {
				if (strings.HasPrefix(event.Type, "marketdata.") && strings.HasPrefix(path, "/v1/market")) ||
					(strings.HasPrefix(event.Type, "positions.") && path == "/v1/positions") ||
					(strings.HasPrefix(event.Type, "order") && (path == "/v1/orders" || path == "/v1/positions")) {
					dirtyPaths[path] = true
				}
			}
			// Snapshot routes apply client filters; event payloads need their own scope check.
			if !server.eventAllowed(ctx, p, raw) {
				continue
			}
			if write(map[string]any{"type": "event", "event_type": event.Type, "order_id": event.AggregateId}) != nil {
				return
			}
		case cmd := <-commands:
			if cmd.Type == "unwatch" {
				delete(watches, cmd.Path)
				continue
			}
			if cmd.Type != "request" {
				continue
			}
			current, e := server.authenticate(ctx)
			if e != nil {
				_ = write(map[string]any{"type": "unauthorized"})
				return
			}
			p = current
			status, body := invoke(cmd)
			if cmd.Watch && cmd.Method == "GET" && status == 200 && wsWatchAllowed(cmd.Path) && len(watches) < 32 {
				if strings.HasSuffix(cmd.Path, "/book") {
					for path := range watches {
						if strings.HasSuffix(path, "/book") {
							delete(watches, path)
						}
					}
				}
				watches[cmd.Path] = true
			}
			if write(map[string]any{"type": "response", "id": cmd.ID, "path": cmd.Path, "status": status, "data": body}) != nil {
				return
			}
		}
	}
}
