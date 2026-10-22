package app

import (
	"net/http"
	"net/http/httptest"
	"statebroker/sbis/internal/platform/config"
	"testing"
)

func TestExchangeViewingModeBlocksMutations(t *testing.T) {
	app := &App{Cfg: config.Config{ExchangeReadOnly: true}}
	handler := app.exchangeReadOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, test := range []struct {
		method, path string
		status       int
	}{{"POST", "/v1/orders", 503}, {"POST", "/api/v1/orders", 503}, {"POST", "/v1/orders/id/cancel", 503}, {"PUT", "/v1/integrations/kfb/settings", 503}, {"GET", "/v1/market", 204}, {"POST", "/v1/auth/login", 204}, {"POST", "/v1/auth/refresh", 204}, {"POST", "/v1/auth/logout", 204}, {"OPTIONS", "/v1/orders", 204}} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.status {
			t.Errorf("%s %s: %d", test.method, test.path, response.Code)
		}
	}
}
