package kfb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGatewayFailureEvidence(t *testing.T) {
	body := `{"error":"rate limit exceeded","detail":"` + strings.Repeat("x", 500) + `"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") == "" {
			t.Error("missing request ID")
		}
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	c := New(server.URL, "secret-token")
	_, _, err := c.do(context.Background(), http.MethodPost, "/api/v1/orders", map[string]string{"client_order_id": "test"})
	var diagnostic *RequestError
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected diagnostic, got %v", err)
	}
	if diagnostic.Source != "GATEWAY" || diagnostic.HTTPStatus != 429 || diagnostic.Response != body || diagnostic.RequestID == "" || diagnostic.Request == nil {
		t.Fatalf("incomplete evidence: %v", err)
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatal("authentication leaked")
	}
}

func TestTransportFailureDoesNotBlameExchange(t *testing.T) {
	c := New("http://127.0.0.1:1", "secret-token")
	_, _, err := c.do(context.Background(), http.MethodPost, "/api/v1/orders", nil)
	var diagnostic *RequestError
	if !errors.As(err, &diagnostic) || diagnostic.Source != "UNKNOWN" || diagnostic.Stage != "transport" {
		t.Fatalf("incorrect attribution: %v", err)
	}
}
