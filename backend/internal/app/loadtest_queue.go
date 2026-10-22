package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type loadQueueStats struct {
	Received              int     `json:"received"`
	Waiting               int     `json:"waiting"`
	Sent                  int     `json:"sent_to_gateway"`
	Failed                int     `json:"failed"`
	Rejected              int     `json:"rejected"`
	OldestWaitSeconds     float64 `json:"oldest_wait_seconds"`
	EstimatedDrainSeconds int     `json:"estimated_drain_seconds"`
	TradingRPS            int     `json:"trading_rps"`
	ReadRPS               int     `json:"read_rps"`
}

// Called under LoadMu; orders are the durable waiting room, not goroutines
// blocked in an HTTP client. Results and run metadata also survive restarts.
func (a *App) restoreLoadRun(ctx context.Context) error {
	if a.LoadRun != nil {
		return nil
	}
	var raw []byte
	err := a.DB.QueryRow(ctx, `SELECT payload FROM load_test_runs ORDER BY started_at DESC LIMIT 1`).Scan(&raw)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var run loadTestRun
	if err := json.Unmarshal(raw, &run); err != nil {
		return err
	}
	run.requests = make(map[int]loadTestRequestResult)
	rows, err := a.DB.Query(ctx, `SELECT payload FROM load_test_results WHERE run_id=$1`, run.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return err
		}
		var item loadTestRequestResult
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		run.requests[item.Sequence] = item
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if run.Status == "RUNNING" {
		run.Status = "DRAINING"
		run.Error = "Генератор прерван перезапуском. Уже принятые заявки продолжают обрабатываться из сохранённой очереди."
	}
	a.LoadRun = &run
	return nil
}

func (a *App) saveLoadRun(ctx context.Context, run *loadTestRun) error {
	raw, err := json.Marshal(run)
	if err != nil {
		return err
	}
	_, err = a.DB.Exec(ctx, `INSERT INTO load_test_runs(id,started_at,payload) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET payload=EXCLUDED.payload`, run.ID, run.StartedAt, raw)
	return err
}

func (a *App) refreshLoadQueue(ctx context.Context, run *loadTestRun) error {
	stats := loadQueueStats{TradingRPS: a.Cfg.KFBTradingRPS, ReadRPS: a.Cfg.KFBReadRPS}
	if stats.TradingRPS < 1 {
		stats.TradingRPS = 20
	}
	if stats.ReadRPS < 1 {
		stats.ReadRPS = 100
	}
	err := a.DB.QueryRow(ctx, `SELECT count(*), count(*) FILTER(WHERE status IN ('QUEUED','ROUTING','SENT')),
 count(*) FILTER(WHERE venue_order_id IS NOT NULL), count(*) FILTER(WHERE status='FAILED'),
 count(*) FILTER(WHERE status='REJECTED'), COALESCE(EXTRACT(EPOCH FROM now()-min(created_at) FILTER(WHERE status='QUEUED')),0)
 FROM orders WHERE client_order_id LIKE $1`, "LOAD-"+run.ID+"-%").Scan(&stats.Received, &stats.Waiting, &stats.Sent, &stats.Failed, &stats.Rejected, &stats.OldestWaitSeconds)
	if err != nil {
		return err
	}
	stats.EstimatedDrainSeconds = (stats.Waiting + stats.TradingRPS - 1) / stats.TradingRPS
	run.Queue = &stats
	if run.Status == "DRAINING" && stats.Waiting == 0 {
		now := time.Now().UTC()
		run.FinishedAt = &now
		run.Status = "PASSED"
		if run.Error != "" || stats.Failed > 0 || stats.Rejected > 0 || stats.Received < run.Planned {
			run.Status = "FAILED"
		}
	}
	return a.saveLoadRun(ctx, run)
}

func (a *App) refreshLoadResults(ctx context.Context, run *loadTestRun) error {
	rows, err := a.DB.Query(ctx, `SELECT o.client_order_id,o.id::text,o.status,COALESCE(o.reject_reason,''),o.side,COALESCE(o.price::text,''),COALESCE(o.client_code,''),COALESCE(a.trading_account,a.account_number),o.quantity::text,COALESCE(o.venue_order_id,''),COALESCE(o.exchange_order_id,''),COALESCE(o.venue_status,''),o.venue_checked_at,COALESCE(o.venue_check_error,''),o.remaining_quantity::text FROM orders o JOIN trading_accounts a ON a.id=o.account_id WHERE o.client_order_id LIKE $1`, "LOAD-"+run.ID+"-%")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var clientID, orderID, status, reason, side, price, code, account, quantity, venueID, exchangeID, venueStatus, venueError, remaining string
		var checkedAt *time.Time
		if err := rows.Scan(&clientID, &orderID, &status, &reason, &side, &price, &code, &account, &quantity, &venueID, &exchangeID, &venueStatus, &checkedAt, &venueError, &remaining); err != nil {
			return err
		}
		var sequence int
		if _, err := fmt.Sscanf(strings.TrimPrefix(clientID, "LOAD-"+run.ID+"-"), "%d", &sequence); err != nil {
			continue
		}
		item, ok := run.requests[sequence]
		if !ok {
			item = loadTestRequestResult{Sequence: sequence, ClientOrderID: clientID, HTTPCode: 201}
		}
		item.ServerOrderID, item.Status, item.RejectReason = orderID, status, reason
		item.Side, item.Price = side, price
		item.ClientCode, item.AccountID = code, account
		item.Quantity = quantity
		item.VenueOrderID, item.ExchangeOrderID, item.VenueStatus, item.VenueCheckedAt, item.VenueCheckError, item.RemainingQuantity = venueID, exchangeID, venueStatus, checkedAt, venueError, remaining
		item.Verified = true
		item.ExchangeReached = status == "ACKNOWLEDGED" || status == "PARTIALLY_FILLED" || status == "FILLED" || status == "CANCELLED" || status == "EXPIRED"
		run.requests[sequence] = item
	}
	return rows.Err()
}
