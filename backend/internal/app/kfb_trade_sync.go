package app

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"statebroker/sbis/internal/platform/id"
)

// syncKFBTrades imports exchange executions into the local books. exec_id is
// unique in executions, making the operation idempotent across polls/restarts.
func (a *App) syncKFBTrades(ctx context.Context) error {
	if a.KFB == nil || !a.KFB.Configured() {
		return nil
	}
	remote, err := a.KFB.FetchTrades(ctx)
	if err != nil {
		return err
	}
	for _, item := range remote {
		if err := a.importKFBTrade(ctx, item); err != nil && a.Log != nil {
			a.Log.Warn("kfb_trade_import", zap.String("exec_id", mapString(item, "exec_id")), zap.Error(err))
		}
	}
	return nil
}

func (a *App) importKFBTrade(ctx context.Context, item map[string]any) error {
	execCode := strings.TrimSpace(mapString(item, "exec_id"))
	qty, qtyErr := decimal.NewFromString(mapString(item, "last_qty"))
	price, priceErr := decimal.NewFromString(mapString(item, "last_px"))
	if execCode == "" || qtyErr != nil || priceErr != nil || !qty.IsPositive() || !price.IsPositive() {
		return nil
	}

	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM executions WHERE exec_id=$1)`, execCode).Scan(&exists); err != nil || exists {
		return err
	}

	var orderID, clientID, accountID, instrumentID uuid.UUID
	var side, currency string
	err = tx.QueryRow(ctx, `
		SELECT o.id, o.client_id, o.account_id, o.instrument_id, o.side, a.currency
		FROM orders o JOIN trading_accounts a ON a.id=o.account_id
		WHERE o.venue_order_id=$1 OR o.client_order_id=$2
		ORDER BY CASE WHEN o.venue_order_id=$1 THEN 0 ELSE 1 END LIMIT 1 FOR UPDATE OF o`,
		mapString(item, "order_id"), mapString(item, "client_order_id")).
		Scan(&orderID, &clientID, &accountID, &instrumentID, &side, &currency)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	remoteSide := strings.ToUpper(strings.TrimSpace(mapString(item, "side")))
	if remoteSide == "BUY" || remoteSide == "SELL" {
		side = remoteSide
	}

	executedAt := time.Now().UTC()
	if raw := mapString(item, "created_at"); raw != "" {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, raw); parseErr == nil {
			executedAt = parsed
		}
	}
	settlementDate := executedAt.AddDate(0, 0, 2)
	gross := qty.Mul(price)
	executionID, tradeID := id.New(), id.New()
	if _, err := tx.Exec(ctx, `INSERT INTO executions
		(id,exec_id,order_id,client_id,account_id,instrument_id,side,quantity,price,gross_amount,commission,net_amount,execution_time,settlement_date)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,0,$10,$11,$12)`,
		executionID, execCode, orderID, clientID, accountID, instrumentID, side, qty, price, gross, executedAt, settlementDate); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO trades
		(id,execution_id,order_id,client_id,account_id,instrument_id,side,quantity,price,gross_amount,commission,net_amount,trade_date,settlement_date,created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,0,$10,$11,$12,$13)`,
		tradeID, executionID, orderID, clientID, accountID, instrumentID, side, qty, price, gross, executedAt, settlementDate, executedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO settlements
		(id,trade_id,account_id,instrument_id,trade_date,settlement_date,cash_status,securities_status,cash_amount,quantity)
		VALUES ($1,$2,$3,$4,$5,$6,'PENDING','PENDING',$7,$8)`, id.New(), tradeID, accountID, instrumentID, executedAt, settlementDate, gross, qty); err != nil {
		return err
	}

	if side == "BUY" {
		if err := a.postReconciliationLedger(ctx, tx, accountID, currency, "TRADE", tradeID, "Покупка на КФБ", "kfb-trade-"+execCode, []ledgerLine{
			moneyLine("AVAILABLE", "BUY", gross, decimal.Zero), moneyLine("SETTLED", "BUY", decimal.Zero, gross),
		}); err != nil {
			return err
		}
		if err := a.applyPositionDelta(ctx, tx, accountID, instrumentID, qty, price, "KFB_TRADE", tradeID); err != nil {
			return err
		}
	} else {
		if err := a.postReconciliationLedger(ctx, tx, accountID, currency, "TRADE", tradeID, "Продажа на КФБ", "kfb-trade-"+execCode, []ledgerLine{
			moneyLine("SETTLED", "SELL", gross, decimal.Zero), moneyLine("AVAILABLE", "SELL", decimal.Zero, gross),
		}); err != nil {
			return err
		}
		if err := a.applyExternalSale(ctx, tx, accountID, instrumentID, qty, price, tradeID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (a *App) applyExternalSale(ctx context.Context, tx pgx.Tx, accountID, instrumentID uuid.UUID, qty, price decimal.Decimal, tradeID uuid.UUID) error {
	var positionID uuid.UUID
	var current decimal.Decimal
	err := tx.QueryRow(ctx, `SELECT id,quantity FROM positions WHERE account_id=$1 AND instrument_id=$2 FOR UPDATE`, accountID, instrumentID).Scan(&positionID, &current)
	if err == pgx.ErrNoRows {
		positionID = id.New()
		if _, err = tx.Exec(ctx, `INSERT INTO positions (id,account_id,instrument_id,quantity,available_qty,blocked_qty,avg_price) VALUES ($1,$2,$3,0,0,0,0)`, positionID, accountID, instrumentID); err != nil {
			return err
		}
		current = decimal.Zero
	} else if err != nil {
		return err
	}
	newQty := current.Sub(qty)
	avgExpr := "avg_price"
	if newQty.IsZero() {
		avgExpr = "0"
	}
	if _, err = tx.Exec(ctx, `UPDATE positions SET quantity=$2,available_qty=available_qty-$3,avg_price=`+avgExpr+`,version=version+1,updated_at=now() WHERE id=$1`, positionID, newQty, qty); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO position_events (id,position_id,account_id,instrument_id,delta_qty,price,reference_type,reference_id) VALUES ($1,$2,$3,$4,$5,$6,'KFB_TRADE',$7)`, id.New(), positionID, accountID, instrumentID, qty.Neg(), price, tradeID)
	return err
}
