package kfb

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
)

func TestRefreshConnectsWithBearerToken(t *testing.T) {
	want := "Bearer sk_test_secretTokenValue"
	auths := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths[r.URL.Path] = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]any{"ready": true})
		case "/api/v1/firm":
			_ = json.NewEncoder(w).Encode(map[string]string{"quik_firm": "MC0031400000", "firm_name": "ТОО Example"})
		case "/api/v1/instruments":
			_ = json.NewEncoder(w).Encode(map[string]any{"instruments": []map[string]any{{"quik_symbol": "KG0105166315", "board": "LISTEDSTOCKK"}}})
		case "/api/v1/accounts":
			_ = json.NewEncoder(w).Encode(map[string]any{"accounts": []map[string]any{{"account_id": "1-3101-006"}}})
		case "/api/v1/positions":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"positions": []map[string]any{{
					"account_id": "1-3101-006", "client_code": "006", "symbol": "KG0105166315",
					"board": "LISTEDSTOCKK", "quantity": "100", "avg_price": "12.5",
					"available_qty": "100", "blocked_qty": "0",
				}},
				"note": "ok",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "sk_test_secretTokenValue")
	snap := c.Refresh(t.Context())
	if !snap.Connected {
		t.Fatalf("not connected: %s", snap.Error)
	}
	for _, p := range []string{"/health", "/ready", "/api/v1/firm", "/api/v1/instruments", "/api/v1/accounts", "/api/v1/positions"} {
		if auths[p] != want {
			t.Fatalf("%s auth %q", p, auths[p])
		}
	}
	if snap.TokenPrefix != "sk_test_secr…" {
		t.Fatalf("prefix %s", snap.TokenPrefix)
	}
	if snap.Firm["quik_firm"] != "MC0031400000" {
		t.Fatalf("firm %+v", snap.Firm)
	}
	if len(snap.Positions) != 1 || snap.Positions[0]["symbol"] != "KG0105166315" {
		t.Fatalf("positions %+v", snap.Positions)
	}
}

func TestDoRequiresTokenHeader(t *testing.T) {
	c := New("http://127.0.0.1:9", "")
	if _, err := c.get(t.Context(), "/health"); err == nil {
		t.Fatal("expected error without token")
	}
}

func TestUnconfiguredIsOffline(t *testing.T) {
	c := New("", "")
	if c.HealthCheck(t.Context()) != "OFFLINE" {
		t.Fatal("expected offline")
	}
}

func TestParsePositionsWrapAndArray(t *testing.T) {
	wrap := parsePositions([]byte(`{"positions":[{"symbol":"ABC","quantity":"10"}],"note":"sync","taken_at":"2026-08-17T00:00:00Z"}`))
	if len(wrap.Positions) != 1 || wrap.Positions[0]["symbol"] != "ABC" || wrap.Note != "sync" {
		t.Fatalf("wrap %+v", wrap)
	}
	arr := parsePositions([]byte(`[{"symbol":"XYZ"}]`))
	if len(arr.Positions) != 1 || arr.Positions[0]["symbol"] != "XYZ" {
		t.Fatalf("array %+v", arr)
	}
	empty := parsePositions([]byte(`{"positions":[]}`))
	if empty.Positions == nil || len(empty.Positions) != 0 {
		t.Fatalf("empty %+v", empty)
	}
}

func TestParseKFBFixSnapshot(t *testing.T) {
	raw := []byte(`{
		"cash":[{"kind":"CASH","client_code":"kalys","account_code":"","symbol":"EQTV","currency":"KGS","net_qty":"10000000","items":{"CUR_BAL":"10000000.00","LOCKED":"0.00"}}],
		"instruments":[{"kind":"INSTRUMENT","client_code":"kalys","account_code":"1-3101-2040","symbol":"KG0105166315","security_id":"KG0105166315","net_qty":"1000000","items":{"AVG_POS_PRICE":"0.000000","CUR_BAL":"1000000.00","LOCKED":"0.00"}}],
		"snapshot":{"status":"OK","completed_at":"2026-08-17T16:43:47+06:00"},
		"source":"fix"
	}`)
	got := parsePositions(raw)
	if got.Note != "OK" {
		t.Fatalf("note %q", got.Note)
	}
	if got.TakenAt == nil {
		t.Fatal("missing taken_at")
	}
	if len(got.Positions) != 2 {
		t.Fatalf("rows %+v", got.Positions)
	}
	cash := got.Positions[0]
	if cash["kind"] != "CASH" || cash["quantity"] != "10000000" || cash["available_qty"] != "10000000.00" {
		t.Fatalf("cash %+v", cash)
	}
	sec := got.Positions[1]
	if sec["kind"] != "INSTRUMENT" || sec["account_id"] != "1-3101-2040" || sec["symbol"] != "KG0105166315" || sec["quantity"] != "1000000" {
		t.Fatalf("instrument %+v", sec)
	}
}

func TestCancelOrderSendsOriginalOrderQty(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/api/v1/firm" {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "quik_firm": "MC0031400000"})
			return
		}
		b, _ := io.ReadAll(r.Body)
		gotPath = r.URL.Path
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "CANCEL_PENDING"})
	}))
	defer srv.Close()

	c := New(srv.URL, "sk_test_secretTokenValue")
	qty := decimal.RequireFromString("1000000")
	if err := c.CancelOrder(t.Context(), "venue-1", "UI-abc", qty); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/orders/venue-1/cancel" {
		t.Fatalf("path %s", gotPath)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("body %s: %v", gotBody, err)
	}
	if body["OrderQty"] != "1000000" || body["quantity"] != "1000000" {
		t.Fatalf("expected original order qty, got %+v", body)
	}
	if body["client_order_id"] != "UI-abc" {
		t.Fatalf("client_order_id %+v", body)
	}
}
