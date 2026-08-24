package telemetry

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "sbis_http_requests_total",
		Help: "HTTP requests",
	}, []string{"method", "code"})
	HTTPLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "sbis_http_request_duration_seconds",
		Help: "HTTP latency",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})
	OrdersTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "sbis_orders_total",
		Help: "Orders submitted",
	})
	TradesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "sbis_trades_total",
		Help: "Trades captured",
	})
	AdapterUp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sbis_adapter_up",
		Help: "Adapter health 1=up",
	}, []string{"code"})
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &writer{ResponseWriter: w, code: 200}
		next.ServeHTTP(sw, r)
		HTTPRequests.WithLabelValues(r.Method, http.StatusText(sw.code)).Inc()
		HTTPLatency.WithLabelValues(r.Method).Observe(time.Since(start).Seconds())
	})
}

type writer struct {
	http.ResponseWriter
	code int
}

func (w *writer) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func Shutdown(ctx context.Context) error {
	_ = ctx
	return nil
}

// OTLP tracing is enabled when OTEL_EXPORTER_OTLP_ENDPOINT is set at the collector.
// Metrics are always exposed on /metrics for Prometheus.
