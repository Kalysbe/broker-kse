package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"statebroker/sbis/internal/adapters"
	"statebroker/sbis/internal/adapters/kfb"
	"statebroker/sbis/internal/audit"
	"statebroker/sbis/internal/loadtest"
	"statebroker/sbis/internal/platform/config"
	"statebroker/sbis/internal/platform/httpx"
)

func TestWaitingRoomSurvivesRestartAnd429(t *testing.T) {
	url := os.Getenv("KFB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("integration test requires Postgres")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "queue_test_" + fmt.Sprintf("%x", uuid.New())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `CREATE TABLE load_test_runs(id uuid PRIMARY KEY,started_at timestamptz,payload jsonb);
 CREATE TABLE load_test_results(run_id uuid,sequence int,payload jsonb,PRIMARY KEY(run_id,sequence));
 CREATE TABLE trading_accounts(id uuid PRIMARY KEY,trading_account text,account_number text);
 CREATE TABLE orders(id uuid PRIMARY KEY,account_id uuid,broker_id uuid,status text,version bigint DEFAULT 0,client_order_id text,side text,order_type text,time_in_force text,symbol text,board text,client_code text,quik_firm text,quantity numeric,remaining_quantity numeric DEFAULT 1,filled_quantity numeric DEFAULT 0,avg_price numeric,venue_details jsonb,price numeric,venue_order_id text,exchange_order_id text,venue_status text,venue_checked_at timestamptz,venue_check_error text,reject_reason text,created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now());
 CREATE TABLE venue_order_updates(id bigserial PRIMARY KEY,order_id uuid,received_at timestamptz DEFAULT now(),payload jsonb);
 CREATE TABLE outbox_events(id uuid,aggregate_type text,aggregate_id uuid,event_type text,payload jsonb);
 CREATE TABLE order_events(id uuid,order_id uuid,from_status text,to_status text,actor_id uuid);
 CREATE TABLE audit_logs(id uuid,action text,actor_id uuid,entity_type text,entity_id uuid,request_id text,ip inet,before_json jsonb,after_json jsonb);`)
	if err != nil {
		t.Fatal(err)
	}
	var allow atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/orders" {
			t.Error("unexpected health or read request")
		}
		if !allow.Load() {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{"order": map[string]any{"id": uuid.NewString(), "status": "ACKNOWLEDGED"}})
	}))
	defer server.Close()
	client := kfb.New(server.URL, "sk_test_mock")
	a := &App{DB: pool, KFB: client, Adapters: adapters.NewManager(nil), Audit: audit.Logger{DB: pool}, Cfg: config.Config{KFBTradingRPS: 20, KFBReadRPS: 100}}
	run := &loadTestRun{ID: uuid.NewString(), StartedAt: time.Now().UTC(), Status: "DRAINING", Planned: 2, requests: make(map[int]loadTestRequestResult)}
	if err := a.saveLoadRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	account := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO trading_accounts VALUES($1,'test-account','test-account')`, account); err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	for seq, id := range ids {
		coid := fmt.Sprintf("LOAD-%s-%08d", run.ID, seq)
		if _, err := pool.Exec(ctx, `INSERT INTO orders(id,account_id,status,client_order_id,side,order_type,time_in_force,symbol,board,client_code,quik_firm,quantity,price) VALUES($1,$2,'QUEUED',$3,'SELL','LIMIT','DAY','TEST','TEST','client','firm',1,100)`, id, account, coid); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.routeOneGatewayOrder(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}
	var status, reason string
	if err := pool.QueryRow(ctx, `SELECT status,reject_reason FROM orders WHERE id=$1`, ids[0]).Scan(&status, &reason); err != nil {
		t.Fatal(err)
	}
	if status != "QUEUED" {
		t.Fatalf("429 lost queued order: %s", status)
	}
	var diagnostic map[string]any
	if err := json.Unmarshal([]byte(reason), &diagnostic); err != nil {
		t.Fatal(err)
	}
	if diagnostic["http_status"] != float64(429) {
		t.Fatalf("missing gateway evidence: %s", reason)
	}
	if err := a.restoreLoadRun(ctx); err != nil {
		t.Fatal(err)
	}
	if err := a.refreshLoadQueue(ctx, a.LoadRun); err != nil {
		t.Fatal(err)
	}
	if a.LoadRun.Status != "DRAINING" || a.LoadRun.Queue.Waiting != 2 {
		t.Fatalf("queue completed too early: %+v", a.LoadRun)
	}
	allow.Store(true)
	for _, id := range ids {
		if err := a.routeOneGatewayOrder(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.refreshLoadQueue(ctx, a.LoadRun); err != nil {
		t.Fatal(err)
	}
	if a.LoadRun.Status != "PASSED" || a.LoadRun.Queue.Sent != 2 || a.LoadRun.Queue.Waiting != 0 {
		t.Fatalf("queue did not drain: %+v", a.LoadRun)
	}
	a.LoadRun = nil
	if err := a.restoreLoadRun(ctx); err != nil {
		t.Fatal(err)
	}
	if err := a.refreshLoadResults(ctx, a.LoadRun); err != nil {
		t.Fatal(err)
	}
	if a.LoadRun.Status != "PASSED" || len(a.LoadRun.requests) != 2 {
		t.Fatal("restart lost run/results")
	}
	// The shared account exception is bound to a persisted run and its token.
	tokenID, brokerID := uuid.New(), uuid.New()
	a.LoadRun.TokenID, a.LoadRun.BrokerID, a.LoadRun.Status = tokenID, brokerID, "RUNNING"
	a.LoadRun.Participants = []loadtest.Participant{{AccountID: "test-account", ClientCode: "Emil"}}
	if err := a.saveLoadRun(ctx, a.LoadRun); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	p := httpx.Principal{TokenID: &tokenID, BrokerID: brokerID}
	req := gwOrderReq{ClientOrderID: "LOAD-" + run.ID + "-00000000", AccountID: "test-account", ClientCode: "Emil"}
	approved, err := a.approvedLoadParticipant(ctx, tx, p, req)
	if err != nil || !approved {
		t.Fatalf("verified shared client rejected: %v", err)
	}
	otherToken := uuid.New()
	p.TokenID = &otherToken
	approved, err = a.approvedLoadParticipant(ctx, tx, p, req)
	if err != nil || approved {
		t.Fatal("another token used the test exception")
	}
	p.TokenID = &tokenID
	req.ClientCode = "unverified"
	approved, err = a.approvedLoadParticipant(ctx, tx, p, req)
	if err != nil || approved {
		t.Fatal("unverified client used the test exception")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE orders SET broker_id=$1`, brokerID); err != nil {
		t.Fatal(err)
	}
	checkActive := func(want int) {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/v1/load-tests/active?page=1&page_size=50", nil).WithContext(httpx.WithPrincipal(ctx, p))
		a.handleActiveLoadOrders(rec, req)
		if rec.Code != 200 {
			t.Fatalf("active list: %s", rec.Body.String())
		}
		var result struct {
			Total int                     `json:"total"`
			Items []loadTestRequestResult `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Total != want || len(result.Items) != want {
			t.Fatalf("active orders: got %d, want %d", result.Total, want)
		}
		if want > 0 && result.Items[0].ExchangeOrderID != "QUIK-TEST-123" {
			t.Fatal("lost exchange ID")
		}
	}
	checkActive(0) // HTTP acceptance alone does not prove an active exchange order.
	if _, err := pool.Exec(ctx, `UPDATE orders SET venue_status='ACKNOWLEDGED',exchange_order_id='QUIK-TEST-123',venue_checked_at=now() WHERE id=$1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	checkActive(1)
	if _, err := pool.Exec(ctx, `UPDATE orders SET venue_check_error='lookup failed' WHERE id=$1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	checkActive(0)
	if _, err := pool.Exec(ctx, `UPDATE orders SET venue_check_error=NULL,venue_status='FILLED' WHERE id=$1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	checkActive(0)

	// Native stream DTO must update the persisted snapshot, preserve the original
	// payload and publish only after the database transaction succeeds.
	var venueID string
	if err := pool.QueryRow(ctx, `SELECT venue_order_id FROM orders WHERE id=$1`, ids[0]).Scan(&venueID); err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(fmt.Sprintf(`{"id":%q,"status":"PARTIALLY_FILLED","quik_order_id":"QUIK-TEST-123","filled_quantity":"0.5","quantity":"1","client_code":"Kalys","extra_diagnostic":"original"}`, venueID))
	if err := a.receiveKFBOrder(ctx, raw); err != nil {
		t.Fatal(err)
	}
	var remaining, original string
	var updates, notifications int
	if err := pool.QueryRow(ctx, `SELECT status,remaining_quantity::text,venue_details->'order'->>'extra_diagnostic' FROM orders WHERE id=$1`, ids[0]).Scan(&status, &remaining, &original); err != nil {
		t.Fatal(err)
	}
	if status != "PARTIALLY_FILLED" || remaining != "0.5" || original != "original" {
		t.Fatalf("stream snapshot: %s %s %s", status, remaining, original)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM venue_order_updates WHERE order_id=$1`, ids[0]).Scan(&updates); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id=$1`, ids[0]).Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if updates != 1 || notifications != 1 {
		t.Fatalf("stream history=%d notifications=%d", updates, notifications)
	}
	invalid := json.RawMessage(fmt.Sprintf(`{"id":%q,"status":"FILLED","filled_quantity":"bad"}`, venueID))
	if err := a.receiveKFBOrder(ctx, invalid); err == nil {
		t.Fatal("invalid event committed")
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id=$1`, ids[0]).Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatal("failed transaction published a notification")
	}
}

func TestWaiting429IsNotAnError(t *testing.T) {
	if isLoadRequestError(loadTestRequestResult{Status: "QUEUED", HTTPCode: 201, RejectReason: `{"code":"RATE_LIMITED"}`}) {
		t.Fatal("normal waiting counted as failure")
	}
	if !isLoadRequestError(loadTestRequestResult{Status: "FAILED", HTTPCode: 201, RejectReason: "failure"}) {
		t.Fatal("actual failure hidden")
	}
}
