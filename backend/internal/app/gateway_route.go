package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"statebroker/sbis/internal/adapters"
	"statebroker/sbis/internal/orders"
)

func (a *App) StartGatewayLoop(ctx context.Context) {
	a.startKFBRealtime(ctx)
	if a.Cfg.ExchangeReadOnly {
		return
	}
	t := time.NewTicker(250 * time.Millisecond)
	tradeTicker := time.NewTicker(5 * time.Second)
	go func() {
		defer t.Stop()
		defer tradeTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := a.ProcessGatewayQueue(ctx); err != nil && a.Log != nil {
					a.Log.Warn("gateway_queue", zap.Error(err))
				}
			case <-tradeTicker.C:
				if err := a.syncKFBTrades(ctx); err != nil && a.Log != nil {
					a.Log.Warn("kfb_trades", zap.Error(err))
				}
			}
		}
	}()
}

func (a *App) ProcessGatewayQueue(ctx context.Context) error {
	if err := a.routeQueuedOrders(ctx); err != nil {
		return err
	}
	if err := a.completeCancels(ctx); err != nil {
		return err
	}
	return a.syncKFBOrderStatuses(ctx)
}

func (a *App) routeQueuedOrders(ctx context.Context) error {
	rows, err := a.DB.Query(ctx, `
		SELECT id FROM orders
		WHERE status='QUEUED'
		ORDER BY created_at
		LIMIT 50`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var idv uuid.UUID
		if rows.Scan(&idv) == nil {
			ids = append(ids, idv)
		}
	}
	rows.Close()
	for _, idv := range ids {
		if err := a.routeOneGatewayOrder(ctx, idv); err != nil && a.Log != nil {
			a.Log.Warn("gateway_route", zap.String("order_id", idv.String()), zap.Error(err))
		}
	}
	return nil
}

func (a *App) routeOneGatewayOrder(ctx context.Context, orderID uuid.UUID) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var status, side, ot, tif, coid, symbol, board, clientCode, quikFirm, accountCode string
	var qty decimal.Decimal
	var price *decimal.Decimal
	err = tx.QueryRow(ctx, `
		SELECT o.status, o.side, o.order_type, o.time_in_force, o.client_order_id, COALESCE(o.symbol,''), COALESCE(o.board,''),
		       COALESCE(o.client_code,''), COALESCE(o.quik_firm,''), o.quantity, o.price,
		       COALESCE(a.trading_account, a.account_number)
		FROM orders o
		JOIN trading_accounts a ON a.id=o.account_id
		WHERE o.id=$1 FOR UPDATE OF o SKIP LOCKED`, orderID).
		Scan(&status, &side, &ot, &tif, &coid, &symbol, &board, &clientCode, &quikFirm, &qty, &price, &accountCode)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	if status != string(orders.Queued) {
		return nil
	}
	if err := a.moveOrder(ctx, tx, orderID, orders.Queued, orders.Routing, uuid.Nil); err != nil {
		return err
	}
	var ad adapters.ExchangeAdapter
	var ok bool
	if a.KFB != nil && a.KFB.Configured() {
		ad, ok = a.KFB, true
	} else {
		ad, ok = a.Adapters.PickKind("EXCHANGE", "WEIGHTED")
	}
	if !ok {
		ad, ok = a.Adapters.PickKind("FIX", "WEIGHTED")
	}
	if !ok {
		ad, ok = a.Adapters.Pick("WEIGHTED")
	}
	if !ok {
		return nil // rollback to QUEUED until an adapter is available
	}
	if err := a.moveOrder(ctx, tx, orderID, orders.Routing, orders.Sent, uuid.Nil); err != nil {
		return err
	}
	px := decimal.Zero
	if price != nil {
		px = *price
	}
	ack, err := ad.SendOrder(ctx, adapters.VenueOrder{
		ClientOrderID: coid,
		VenueSymbol:   symbol,
		Account:       accountCode,
		ClientCode:    clientCode,
		QuikFirm:      quikFirm,
		Board:         board,
		Side:          side,
		Type:          ot,
		TIF:           tif,
		Quantity:      qty,
		Price:         px,
	})
	a.Adapters.Release(ad.Code())
	if err != nil {
		if isGatewayRateLimit(err) {
			if queueErr := a.queueRateLimitedOrder(ctx, tx, orderID, ad.Code(), err, uuid.Nil); queueErr != nil {
				return queueErr
			}
			return tx.Commit(ctx)
		}
		reason := adapterFailureReason(ad.Code(), err)
		if failErr := a.failOrder(ctx, tx, orderID, orders.Sent, reason, uuid.Nil); failErr != nil {
			return failErr
		}
		if auditErr := a.Audit.WriteTx(ctx, tx, "ORDER_ADAPTER_FAILED", "order", &orderID, nil, json.RawMessage(reason)); auditErr != nil {
			return auditErr
		}
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `UPDATE orders SET venue_order_id=$2, reject_reason=NULL WHERE id=$1`, orderID, ack.VenueOrderID); err != nil {
		return err
	}
	if err := a.moveOrder(ctx, tx, orderID, orders.Sent, orders.Acknowledged, uuid.Nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (a *App) completeCancels(ctx context.Context) error {
	rows, err := a.DB.Query(ctx, `
		SELECT o.id, COALESCE(o.venue_order_id,''), o.client_order_id, o.side, o.account_id,
		       o.remaining_quantity, o.quantity, o.price, a.currency
		FROM orders o
		JOIN trading_accounts a ON a.id=o.account_id
		WHERE o.status='CANCEL_PENDING'
		ORDER BY o.updated_at
		LIMIT 50`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		id, acct                    uuid.UUID
		venue, coid, side, currency string
		remain, qty                 decimal.Decimal
		price                       *decimal.Decimal
	}
	var list []row
	for rows.Next() {
		var r row
		if rows.Scan(&r.id, &r.venue, &r.coid, &r.side, &r.acct, &r.remain, &r.qty, &r.price, &r.currency) == nil {
			list = append(list, r)
		}
	}
	rows.Close()
	for _, item := range list {
		ad, ok := a.Adapters.PickKind("EXCHANGE", "WEIGHTED")
		if !ok {
			ad, ok = a.Adapters.PickKind("FIX", "WEIGHTED")
		}
		if !ok {
			ad, ok = a.Adapters.Pick("WEIGHTED")
		}
		if ok {
			cancelErr := ad.CancelOrder(ctx, item.venue, item.coid, item.qty)
			a.Adapters.Release(ad.Code())
			if cancelErr != nil {
				if auditErr := a.Audit.Write(ctx, "ORDER_CANCEL_ADAPTER_ERROR", "order", &item.id, nil, json.RawMessage(adapterFailureReason(ad.Code(), cancelErr))); auditErr != nil {
					return auditErr
				}
				if a.Log != nil {
					a.Log.Warn("gateway_cancel", zap.String("order_id", item.id.String()), zap.Error(cancelErr))
				}
				continue
			}
		} else {
			continue
		}
		tx, err := a.DB.Begin(ctx)
		if err != nil {
			continue
		}
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1 FOR UPDATE`, item.id).Scan(&status); err != nil {
			_ = tx.Rollback(ctx)
			continue
		}
		if status != string(orders.CancelPending) {
			_ = tx.Rollback(ctx)
			continue
		}
		if err := a.moveOrder(ctx, tx, item.id, orders.CancelPending, orders.Cancelled, uuid.Nil); err != nil {
			_ = tx.Rollback(ctx)
			continue
		}
		if item.side == "BUY" && item.remain.IsPositive() && item.price != nil {
			_ = a.releaseBuyReserve(ctx, tx, item.acct, item.currency, item.id, item.remain.Mul(*item.price), uuid.Nil)
		}
		_ = tx.Commit(ctx)
	}
	return nil
}
