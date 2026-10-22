package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"statebroker/sbis/internal/adapters/kfb"
	"statebroker/sbis/internal/platform/httpx"
)

func TestOrderListUsesExchangeAndNeverFallsBack(t *testing.T) {
	for _, tc := range []struct {
		name, body              string
		upstream, status, count int
	}{
		{"three QUIK orders", `{"source":"quik_db","orders":[{"id":"quik-1","symbol":"GBA","status":"CANCELLED"},{"id":"quik-2","symbol":"GBA","status":"ACKNOWLEDGED"},{"id":"quik-3","symbol":"GBA","status":"ACKNOWLEDGED"}]}`, 200, 200, 3},
		{"empty exchange", `{"orders":[]}`, 200, 200, 0},
		{"unavailable exchange", `{}`, 502, 503, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.upstream)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			// No DB: the configured exchange route must never query local history.
			a := &App{KFB: kfb.New(server.URL, "test")}
			rec := httptest.NewRecorder()
			a.handleListOrders(rec, httptest.NewRequest("GET", "/v1/orders", nil))
			if rec.Code != tc.status {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
			}
			if rec.Code == 200 {
				var body struct {
					Items  []map[string]any `json:"items"`
					Source string           `json:"source"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Items == nil || len(body.Items) != tc.count || body.Source != "KFB" {
					t.Fatalf("unexpected exchange list: %s err=%v", rec.Body, err)
				}
			}
		})
	}
}

func TestExchangeOrdersEnrichIDsWithoutAddingLocalHistory(t *testing.T) {
	databaseURL := os.Getenv("KFB_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("integration test requires Postgres")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "exchange_orders_" + fmt.Sprintf("%x", uuid.New())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `CREATE TABLE clients(id uuid,client_code text);
	CREATE TABLE trading_accounts(id uuid,client_id uuid,broker_id uuid,client_code text,trading_account text,account_number text,created_at timestamptz DEFAULT now());
	CREATE TABLE instruments(id uuid,quik_symbol text,ticker text,isin text,board text,created_at timestamptz DEFAULT now());
	CREATE TABLE orders(id uuid,broker_id uuid,client_id uuid,account_id uuid,instrument_id uuid,client_order_id text,client_code text,order_type text,status text,venue_order_id text,exchange_order_id text,created_at timestamptz DEFAULT now());`)
	if err != nil {
		t.Fatal(err)
	}
	broker, clientID, account, instrument, localID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, stmt := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO clients VALUES($1,'Kalys')`, []any{clientID}},
		{`INSERT INTO trading_accounts(id,client_id,broker_id,client_code,trading_account,account_number) VALUES($1,$2,$3,'Kalys','shared','shared')`, []any{account, clientID, broker}},
		{`INSERT INTO instruments(id,quik_symbol,ticker,isin,board) VALUES($1,'GBA','GBA','GBA','LIST_GBA')`, []any{instrument}},
		{`INSERT INTO orders(id,broker_id,client_id,account_id,instrument_id,client_order_id,client_code,order_type,status,venue_order_id,exchange_order_id) VALUES($1,$2,$3,$4,$5,'local-client-id','Kalys','LIMIT','ACKNOWLEDGED','gateway-id','261022-LIST_GBA-22457')`, []any{localID, broker, clientID, account, instrument}},
		{`INSERT INTO orders(id,broker_id,client_id,account_id,instrument_id,status,venue_order_id) SELECT gen_random_uuid(),$1,$2,$3,$4,'ACKNOWLEDGED','old-'||n FROM generate_series(1,250) n`, []any{broker, clientID, account, instrument}},
	} {
		if _, err = pool.Exec(ctx, stmt.query, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"source":"quik_db","orders":[
		{"id":"quik-current","quik_order_id":"261022-LIST_GBA-22457","account_id":"shared","client_code":"Kalys","symbol":"GBA","board":"LIST_GBA","status":"CANCELLED","quantity":"1","filled_quantity":"0","price":"130"},
		{"id":"quik-external","account_id":"shared","client_code":"Kalys","symbol":"GBA","board":"LIST_GBA","status":"ACKNOWLEDGED","quantity":"2","filled_quantity":"0"},
		{"id":"other-client","account_id":"shared","client_code":"Other","symbol":"GBA","status":"ACKNOWLEDGED"}]}`)
	}))
	defer server.Close()
	a := &App{DB: pool, KFB: kfb.New(server.URL, "test")}
	p := httpx.Principal{BrokerID: broker, ClientID: &clientID}
	req := httptest.NewRequest("GET", "/v1/orders", nil).WithContext(httpx.WithPrincipal(ctx, p))
	rec := httptest.NewRecorder()
	a.handleListOrders(rec, req)
	var result struct {
		Items []map[string]any `json:"items"`
	}
	if err = json.Unmarshal(rec.Body.Bytes(), &result); err != nil || rec.Code != 200 || len(result.Items) != 2 {
		t.Fatalf("status=%d body=%s err=%v", rec.Code, rec.Body, err)
	}
	first, external := result.Items[0], result.Items[1]
	if first["id"] != localID.String() || first["status"] != "CANCELLED" || first["can_cancel"] != false || first["price"] != "130" {
		t.Fatalf("local history overrode exchange: %+v", first)
	}
	if external["id"] != "quik-external" || external["account_id"] != account.String() || external["instrument_id"] != instrument.String() || external["can_cancel"] != false {
		t.Fatalf("external order lost: %+v", external)
	}
}
