package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"statebroker/sbis/internal/loadtest"
	"strconv"
	"testing"

	"github.com/shopspring/decimal"
)

func TestSubmitUsesAssignedClientAndRecordsIt(t *testing.T) {
	var submittedQuantity string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		submittedQuantity, _ = body["quantity"].(string)
		quantity, err := strconv.Atoi(submittedQuantity)
		if err != nil || quantity < 100 || quantity > 1000 {
			t.Errorf("quantity outside 100..1000: %v", body["quantity"])
		}
		if body["client_code"] != "imar" || body["account_id"] != "shared" {
			t.Errorf("wrong participant: %v", body)
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"order":{"id":"test-order","status":"QUEUED"}}`))
	}))
	defer server.Close()
	c := config{baseURL: server.URL, token: "test", price: "100", participants: []loadtest.Participant{{AccountID: "shared", ClientCode: "Kalys"}, {AccountID: "shared", ClientCode: "imar"}}, schedule: []int{1}}
	result := submit(context.Background(), server.Client(), c, 0)
	if result.Quantity != submittedQuantity {
		t.Fatal("recorded quantity differs from submitted quantity")
	}
	if result.ClientCode != "imar" || result.AccountID != "shared" || result.Error != "" {
		t.Fatalf("missing participant in audit result: %+v", result)
	}
}

func TestVariedPriceStaysWithinFivePercent(t *testing.T) {
	seen := map[string]bool{}
	for sequence := 0; sequence < 110; sequence++ {
		price, err := decimal.NewFromString(variedPrice("100", sequence))
		if err != nil {
			t.Fatal(err)
		}
		if price.LessThan(decimal.NewFromInt(95)) || price.GreaterThan(decimal.NewFromInt(105)) {
			t.Fatalf("price %s is outside 95..105", price)
		}
		seen[price.String()] = true
	}
	if len(seen) != 11 {
		t.Fatalf("expected 11 price levels, got %d", len(seen))
	}
}

func TestQueuedRequestsAreExpectedInWaitingRoom(t *testing.T) {
	queued := []result{{HTTPCode: 201, ServerID: "saved", Status: "QUEUED", Verified: true}}
	if !printSummary(queued, true) {
		t.Fatal("durably queued request counted as a failed test")
	}
	if printSummary(queued) {
		t.Fatal("direct mode falsely confirmed exchange receipt")
	}
	queued[0].Status = "FAILED"
	if printSummary(queued, true) {
		t.Fatal("adapter failure hidden by waiting room")
	}
}
