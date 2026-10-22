package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"statebroker/sbis/internal/adapters/kfb"
	"statebroker/sbis/internal/orders"
	"strings"
	"time"
)

// Snapshot, original stream payload, lifecycle and notification commit together.
func (a *App) applyKFBOrder(ctx context.Context, ref kfbOrderRef, remote kfb.RemoteOrder, raw ...json.RawMessage) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var localStatus string
	var priorTimestamp *string
	if err = tx.QueryRow(ctx, `SELECT status,venue_details->'order'->>'updated_at' FROM orders WHERE id=$1 FOR UPDATE`, ref.id).Scan(&localStatus, &priorTimestamp); err != nil {
		return err
	}
	if priorTimestamp != nil && !remote.UpdatedAt.IsZero() {
		if prior, err := time.Parse(time.RFC3339Nano, *priorTimestamp); err == nil && remote.UpdatedAt.Before(prior) {
			return nil
		}
	}
	status := orders.Status(strings.ToUpper(strings.TrimSpace(remote.Status)))
	payload, err := json.Marshal(remote)
	if err != nil {
		return err
	}
	if len(raw) > 0 {
		payload = raw[0]
	}
	if _, err = tx.Exec(ctx, `UPDATE orders SET venue_status=$2,venue_checked_at=now(),venue_check_error=NULL,exchange_order_id=COALESCE(NULLIF($3,''),exchange_order_id),venue_details=jsonb_set(COALESCE(venue_details,'{"events":[],"trades":[]}'::jsonb),'{order}',$4::jsonb) WHERE id=$1`, ref.id, status, remote.QuikOrderID, payload); err != nil {
		return err
	}
	if len(raw) > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO venue_order_updates(order_id,payload) VALUES($1,$2)`, ref.id, payload); err != nil {
			return err
		}
	}
	from := orders.Status(localStatus)
	if !orders.IsTerminal(from) && (from == status || orders.CanTransition(from, status)) {
		filled := decimal.Zero
		if remote.FilledQuantity != "" {
			filled, err = decimal.NewFromString(remote.FilledQuantity)
			if err != nil || filled.IsNegative() {
				return fmt.Errorf("invalid venue filled quantity")
			}
		}
		var avg any
		if remote.AveragePrice != "" {
			parsed, e := decimal.NewFromString(remote.AveragePrice)
			if e == nil && parsed.IsPositive() {
				avg = parsed
			}
		}
		if from != status {
			if err = a.moveOrder(ctx, tx, ref.id, from, status, uuid.Nil); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE orders SET filled_quantity=$2,remaining_quantity=GREATEST(quantity-$2,0),avg_price=COALESCE($3,avg_price),reject_reason=COALESCE(NULLIF($4,''),reject_reason),updated_at=now() WHERE id=$1`, ref.id, filled, avg, remote.RejectReason); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'order',$2::uuid,'order.venue_updated',jsonb_build_object('order_id',$2::uuid::text))`, uuid.New(), ref.id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
