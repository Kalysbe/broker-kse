package httpx

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ctxKey string

const (
	ctxRequestID ctxKey = "request_id"
	ctxPrincipal ctxKey = "principal"
	ctxStart     ctxKey = "start"
)

type Principal struct {
	UserID       uuid.UUID
	SessionID    uuid.UUID
	BrokerID     uuid.UUID
	ClientID     *uuid.UUID
	TokenID      *uuid.UUID
	Roles        []string
	Permissions  map[string]struct{}
	IP           string
	QuikFirm     string
	FirmName     string
	IsAPIToken   bool
	HMACRequired bool
}

func (p Principal) Has(perm string) bool {
	if p.Permissions == nil {
		return false
	}
	_, ok := p.Permissions[perm]
	if ok {
		return true
	}
	_, ok = p.Permissions["*"]
	return ok
}

func (p Principal) HasRole(code string) bool {
	for _, r := range p.Roles {
		if r == code {
			return true
		}
	}
	return false
}

func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxRequestID).(string)
	return v
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxPrincipal).(Principal)
	return p, ok
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxPrincipal, p)
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 12)
			_, _ = rand.Read(b)
			id = "req_" + hex.EncodeToString(b)
		}
		trace := r.Header.Get("X-Trace-ID")
		if trace == "" {
			trace = id
		}
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Trace-ID", trace)
		ctx := context.WithValue(r.Context(), ctxRequestID, id)
		ctx = context.WithValue(ctx, ctxStart, time.Now())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func AccessLog(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w, code: 200}
			next.ServeHTTP(sw, r)
			start, _ := r.Context().Value(ctxStart).(time.Time)
			dur := time.Since(start).Milliseconds()
			p, _ := PrincipalFrom(r.Context())
			fields := []zap.Field{
				zap.String("request_id", RequestIDFrom(r.Context())),
				zap.String("ip", ClientIP(r)),
				zap.String("endpoint", r.URL.Path),
				zap.String("method", r.Method),
				zap.Int("status", sw.code),
				zap.Int64("duration_ms", dur),
			}
			if p.UserID != uuid.Nil {
				fields = append(fields, zap.String("user_id", p.UserID.String()))
			}
			if p.BrokerID != uuid.Nil {
				fields = append(fields, zap.String("broker_id", p.BrokerID.String()))
			}
			log.Info("http_request", fields...)
		})
	}
}

func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func Recoverer(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic", zap.Any("recover", rec), zap.String("request_id", RequestIDFrom(r.Context())))
					WriteError(w, r, E(http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}
