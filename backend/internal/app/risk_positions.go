package app

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"statebroker/sbis/internal/platform/httpx"
)

func (a *App) preTradeRisk(ctx context.Context, tx pgx.Tx, clientID, accountID, instrumentID, traderID uuid.UUID, side string, qty, notional decimal.Decimal, currency string) error {
	var sessionOK bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM trading_calendars tc
			JOIN exchanges e ON e.id = tc.exchange_id
			JOIN listings l ON l.exchange_id = e.id
			WHERE l.instrument_id=$1 AND e.status='ACTIVE'
			  AND tc.weekday = EXTRACT(DOW FROM (now() AT TIME ZONE e.timezone))::int
			  AND (now() AT TIME ZONE e.timezone)::time BETWEEN tc.open_time AND tc.close_time
		)`, instrumentID).Scan(&sessionOK)
	if err != nil {
		return err
	}
	var enforce bool
	_ = tx.QueryRow(ctx, `SELECT COALESCE((value_json)::text, 'false') = 'true' FROM system_settings WHERE key='trading.enforce_hours'`).Scan(&enforce)
	if enforce && !sessionOK {
		return httpx.E(422, "MARKET_CLOSED", "Instrument is outside trading hours")
	}

	type lim struct {
		Scope string
		ID    uuid.UUID
	}
	scopes := []lim{{"ACCOUNT", accountID}, {"CLIENT", clientID}, {"INSTRUMENT", instrumentID}, {"TRADER", traderID}}
	for _, s := range scopes {
		rows, err := tx.Query(ctx, `
			SELECT limit_type, amount FROM risk_limits
			WHERE scope_type=$1 AND scope_id=$2 AND status='ACTIVE'`, s.Scope, s.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var lt string
			var amt decimal.Decimal
			if err := rows.Scan(&lt, &amt); err != nil {
				rows.Close()
				return err
			}
			switch lt {
			case "MAX_ORDER_QTY":
				if qty.GreaterThan(amt) {
					rows.Close()
					return httpx.E(422, "LIMIT_EXCEEDED", "Order quantity exceeds limit")
				}
			case "MAX_ORDER_NOTIONAL":
				if notional.GreaterThan(amt) {
					rows.Close()
					return httpx.E(422, "LIMIT_EXCEEDED", "Order notional exceeds limit")
				}
			}
		}
		rows.Close()
	}
	return nil
}

func (a *App) calcCommission(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, gross decimal.Decimal, currency string) (decimal.Decimal, uuid.UUID, []byte, error) {
	var tariffID uuid.UUID
	var ctype string
	var pct, fixed, min, maxn decimal.Decimal
	var maxPtr *decimal.Decimal
	err := tx.QueryRow(ctx, `
		SELECT tp.id, tp.commission_type, tp.percentage, tp.fixed_fee, tp.minimum_fee, tp.maximum_fee
		FROM trading_accounts a
		JOIN contracts c ON c.id = a.contract_id
		JOIN tariff_plans tp ON tp.id = c.tariff_plan_id
		WHERE a.id=$1`, accountID).Scan(&tariffID, &ctype, &pct, &fixed, &min, &maxPtr)
	if err != nil {
		return decimal.Zero, uuid.Nil, nil, err
	}
	if maxPtr != nil {
		maxn = *maxPtr
	}
	fee := decimal.Zero
	switch ctype {
	case "FIXED", "PER_TRANSACTION":
		fee = fixed
	case "PERCENTAGE":
		fee = gross.Mul(pct)
	case "TIERED", "PER_VOLUME":
		var from, perc decimal.Decimal
		var to *decimal.Decimal
		_ = tx.QueryRow(ctx, `
			SELECT from_amount, to_amount, percentage FROM commission_tiers
			WHERE tariff_id=$1 AND from_amount <= $2 AND (to_amount IS NULL OR $2 < to_amount)
			ORDER BY from_amount DESC LIMIT 1`, tariffID, gross).Scan(&from, &to, &perc)
		fee = gross.Mul(perc)
	}
	if fee.LessThan(min) {
		fee = min
	}
	if maxPtr != nil && fee.GreaterThan(maxn) {
		fee = maxn
	}
	calc, _ := json.Marshal(map[string]any{"type": ctype, "gross": gross.String(), "fee": fee.String(), "currency": currency})
	return fee.RoundBank(2), tariffID, calc, nil
}

func (a *App) blockPosition(ctx context.Context, tx pgx.Tx, accountID, instrumentID uuid.UUID, qty decimal.Decimal) error {
	var avail decimal.Decimal
	err := tx.QueryRow(ctx, `
		SELECT available_qty FROM positions WHERE account_id=$1 AND instrument_id=$2 FOR UPDATE`, accountID, instrumentID).Scan(&avail)
	if err != nil {
		return httpx.ErrInsufficientPos
	}
	if avail.LessThan(qty) {
		return httpx.ErrInsufficientPos
	}
	_, err = tx.Exec(ctx, `
		UPDATE positions SET available_qty=available_qty-$3, blocked_qty=blocked_qty+$3, version=version+1, updated_at=now()
		WHERE account_id=$1 AND instrument_id=$2`, accountID, instrumentID, qty)
	return err
}

func (a *App) applyPositionDelta(ctx context.Context, tx pgx.Tx, accountID, instrumentID uuid.UUID, delta, price decimal.Decimal, refType string, refID uuid.UUID) error {
	var posID uuid.UUID
	var qty, avg decimal.Decimal
	err := tx.QueryRow(ctx, `
		SELECT id, quantity, avg_price FROM positions WHERE account_id=$1 AND instrument_id=$2 FOR UPDATE`,
		accountID, instrumentID).Scan(&posID, &qty, &avg)
	if err != nil {
		posID = uuid.Must(uuid.NewV7())
		if _, err := tx.Exec(ctx, `
			INSERT INTO positions (id, account_id, instrument_id, quantity, available_qty, blocked_qty, avg_price)
			VALUES ($1,$2,$3,0,0,0,0)`, posID, accountID, instrumentID); err != nil {
			return err
		}
		qty, avg = decimal.Zero, decimal.Zero
	}
	newQty := qty.Add(delta)
	newAvg := avg
	if delta.IsPositive() && newQty.IsPositive() {
		newAvg = qty.Mul(avg).Add(delta.Mul(price)).Div(newQty)
	}
	if newQty.IsZero() {
		newAvg = decimal.Zero
	}
	availAdj := delta
	if delta.IsNegative() {
		if _, err := tx.Exec(ctx, `
			UPDATE positions SET quantity=$3, available_qty=available_qty, blocked_qty=blocked_qty+$4,
			avg_price=$5, version=version+1, updated_at=now() WHERE id=$1`,
			posID, accountID, newQty, delta, newAvg); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(ctx, `
			UPDATE positions SET quantity=$2, available_qty=available_qty+$3, avg_price=$4, version=version+1, updated_at=now()
			WHERE id=$1`, posID, newQty, availAdj, newAvg); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO position_events (id, position_id, account_id, instrument_id, delta_qty, price, reference_type, reference_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, uuid.Must(uuid.NewV7()), posID, accountID, instrumentID, delta, price, refType, refID)
	return err
}
