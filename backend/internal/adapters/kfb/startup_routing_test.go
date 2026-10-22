package kfb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
	"statebroker/sbis/internal/adapters"
)

func TestManagerRoutesWithProfileLoadedAfterConstruction(t *testing.T) {
	orders := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer saved-profile-token" {
			t.Error("saved profile token was not used")
		}
		if r.URL.Path != "/api/v1/orders" {
			w.Write([]byte(`{}`))
			return
		}
		orders++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["client_code"] != "test-client" || body["price"] != "1000" || body["symbol"] != "test-symbol" {
			t.Errorf("unexpected order parameters: %v", body)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"order":{"id":"test-gateway-id","status":"ACKNOWLEDGED"}}`))
	}))
	defer server.Close()

	client := New("", "")
	manager := adapters.Default(client)
	if _, ok := manager.PickKind("EXCHANGE", "WEIGHTED"); ok {
		t.Fatal("unconfigured exchange must not be selected")
	}
	// The same client gets the persisted settings after the manager was built.
	client.ApplyConnection(ConnectionSettings{RESTURL: server.URL, APIToken: "saved-profile-token", Environment: "test"})
	adapter, ok := manager.PickKind("EXCHANGE", "WEIGHTED")
	if !ok || adapter != client {
		t.Fatal("saved KFB profile is unavailable for routing")
	}
	defer manager.Release(adapter.Code())
	ack, err := adapter.SendOrder(context.Background(), adapters.VenueOrder{
		ClientOrderID: "test-order", ClientCode: "test-client", Account: "test-account",
		VenueSymbol: "test-symbol", Board: "test-board", Side: "BUY", Type: "LIMIT", TIF: "DAY",
		Quantity: decimal.NewFromInt(1), Price: decimal.NewFromInt(1000),
	})
	if err != nil || ack.VenueOrderID != "test-gateway-id" || orders != 1 {
		t.Fatalf("saved profile routing failed: ack=%+v err=%v requests=%d", ack, err, orders)
	}
}
