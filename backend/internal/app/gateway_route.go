package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"statebroker/sbis/internal/adapters"
	"statebroker/sbis/internal/orders"
)

func (a *App) StartGatewayLoop(ctx context.Context) {
	t := time.NewTicker(250 * time.Millisecond)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := a.ProcessGatewayQueue(ctx); err != nil && a.Log != nil {
					a.Log.Warn("gateway_queue", zap.Error(err))
				}
			}
		}
	}()
}

func (a *App) ProcessGatewayQueue(ctx context.Context) error {
	if err := a.routeQueuedOrders(ctx); err != nil {
		return err
	}
	return a.completeCancels(ctx)
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
		WHERE o.id=$1 FOR UPDATE`, orderID).
		Scan(&status, &side, &ot, &tif, &coid, &symbol, &board, &clientCode, &quikFirm, &qty, &price, &accountCode)
	if err != nil {
		return err
	}
	if status != string(orders.Queued) {
		return nil
	}
	if err := a.moveOrder(ctx, tx, orderID, orders.Queued, orders.Routing, uuid.Nil); err != nil {
		return err
	}
	ad, ok := a.Adapters.PickKind("EXCHANGE", "WEIGHTED")
	if !ok {
		ad, ok = a.Adapters.PickKind("FIX", "WEIGHTED")
	}
	if !ok {
		ad, ok = a.Adapters.Pick("WEIGHTED")
	}
	if !ok {
		_ = a.moveOrder(ctx, tx, orderID, orders.Routing, orders.Failed, uuid.Nil)
		_, _ = tx.Exec(ctx, `UPDATE orders SET reject_reason='ADAPTER_UNAVAILABLE' WHERE id=$1`, orderID)
		return tx.Commit(ctx)
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
		_ = a.moveOrder(ctx, tx, orderID, orders.Sent, orders.Failed, uuid.Nil)
		_, _ = tx.Exec(ctx, `UPDATE orders SET reject_reason='ADAPTER_ERROR' WHERE id=$1`, orderID)
		return tx.Commit(ctx)
	}
	_, _ = tx.Exec(ctx, `UPDATE orders SET venue_order_id=$2 WHERE id=$1`, orderID, ack.VenueOrderID)
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
			_ = ad.CancelOrder(ctx, item.venue, item.coid, item.qty)
			a.Adapters.Release(ad.Code())
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
