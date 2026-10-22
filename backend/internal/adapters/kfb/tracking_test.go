package kfb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
)

func TestMockOrderIDNeverReachesGateway(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	c := New(server.URL, "test")
	id := "VENUE-LOAD-1790940089294981207-00008973"
	if _, err := c.FetchOrder(context.Background(), id); err == nil {
		t.Fatal("mock status lookup must fail locally")
	}
	if _, err := c.FetchOrderDetails(context.Background(), id); err == nil {
		t.Fatal("mock details lookup must fail locally")
	}
	if err := c.CancelOrder(context.Background(), id, "LOAD-test", decimal.NewFromInt(1)); err == nil {
		t.Fatal("mock cancellation must fail locally")
	}
	if requests != 0 {
		t.Fatalf("sent %d mock requests to gateway", requests)
	}
}

func TestOrderTrackingPreservesExchangeIDAndEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("missing authorization")
		}
		_, _ = w.Write([]byte(`{"order":{"id":"gateway-id","status":"ACKNOWLEDGED","quik_order_id":"EXCHANGE-123"},"events":[{"event_type":"ORDER_EXECUTION_REPORT"}]}`))
	}))
	defer server.Close()
	c := New(server.URL, "test")
	order, err := c.FetchOrder(context.Background(), "gateway-id")
	if err != nil {
		t.Fatal(err)
	}
	if order.QuikOrderID != "EXCHANGE-123" {
		t.Fatal("exchange ID lost")
	}
	raw, err := c.FetchOrderDetails(context.Background(), "gateway-id")
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	if len(response["events"]) == 0 {
		t.Fatal("tracking events lost")
	}
}
