package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"statebroker/sbis/internal/adapters"
	"statebroker/sbis/internal/orders"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/id"
	"statebroker/sbis/internal/platform/money"
	"statebroker/sbis/internal/platform/telemetry"
)

type orderReq struct {
	ClientOrderID string `json:"client_order_id"`
	AccountID     string `json:"account_id"`
	InstrumentID  string `json:"instrument_id"`
	Side          string `json:"side"`
	OrderType     string `json:"order_type"`
	TimeInForce   string `json:"time_in_force"`
	Quantity      string `json:"quantity"`
	Price         string `json:"price"`
	StopPrice     string `json:"stop_price"`
}

func (a *App) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req orderReq
	if err := decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r.Context())
	out, existing, err := a.submitOrder(r.Context(), p, req)
	if err != nil {
		if a.Log != nil {
			a.Log.Error("submit_order_failed", zap.Error(err), zap.String("request_id", httpx.RequestIDFrom(r.Context())))
		}
		httpx.WriteError(w, r, dbWriteError(err))
		return
	}
	if existing {
		httpx.WriteJSON(w, http.StatusOK, out)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (a *App) submitOrder(ctx context.Context, p httpx.Principal, req orderReq) (map[string]any, bool, error) {
	if req.ClientOrderID == "" {
		return nil, false, httpx.E(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "client_order_id is required")
	}
	acctID, err := a.resolveOrderAccount(ctx, p, req.AccountID)
	if err != nil {
		return nil, false, err
	}
	instrID, err := uuid.Parse(req.InstrumentID)
	if err != nil {
		return nil, false, httpx.E(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Выберите инструмент")
	}
	qty, err := money.Parse(req.Quantity)
	if err != nil || !qty.IsPositive() {
		return nil, false, httpx.E(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid quantity")
	}
	var px decimal.Decimal
	if req.Price != "" {
		px, err = money.Parse(req.Price)
		if err != nil {
			return nil, false, httpx.ErrValidation
		}
	}
	var stop decimal.Decimal
	if req.StopPrice != "" {
		stop, err = money.Parse(req.StopPrice)
		if err != nil {
			return nil, false, httpx.ErrValidation
		}
	}
	side := strings.ToUpper(req.Side)
	ot := strings.ToUpper(req.OrderType)
	tif := strings.ToUpper(req.TimeInForce)
	if side != "BUY" && side != "SELL" {
		return nil, false, httpx.ErrValidation
	}

	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	var existingID uuid.UUID
	var existingStatus string
	err = tx.QueryRow(ctx, `SELECT id, status FROM orders WHERE broker_id=$1 AND client_order_id=$2`, p.BrokerID, req.ClientOrderID).
		Scan(&existingID, &existingStatus)
	if err == nil {
		_ = tx.Rollback(ctx)
		body, _ := a.orderJSON(ctx, existingID)
		return body, true, nil
	}

	var clientID, brokerID uuid.UUID
	var acctStatus, currency, accountCode, clientCode string
	if err := tx.QueryRow(ctx, `
		SELECT a.client_id, a.broker_id, a.status, a.currency,
		       COALESCE(NULLIF(a.trading_account,''), a.account_number),
		       COALESCE(NULLIF(a.client_code,''), COALESCE(c.client_code,''))
		FROM trading_accounts a
		JOIN clients c ON c.id=a.client_id
		WHERE a.id=$1 FOR UPDATE OF a`, acctID).
		Scan(&clientID, &brokerID, &acctStatus, &currency, &accountCode, &clientCode); err != nil {
		return nil, false, httpx.ErrNotFound
	}
	if acctStatus != "ACTIVE" {
		return nil, false, httpx.ErrAccountBlocked
	}
	if p.ClientID != nil && clientID != *p.ClientID {
		return nil, false, httpx.ErrForbidden
	}

	var instrStatus, quikSymbol, instrBoard string
	if err := tx.QueryRow(ctx, `
		SELECT status, COALESCE(NULLIF(quik_symbol,''), ticker), COALESCE(board,'')
		FROM instruments WHERE id=$1`, instrID).
		Scan(&instrStatus, &quikSymbol, &instrBoard); err != nil {
		return nil, false, httpx.ErrNotFound
	}
	if instrStatus != "ACTIVE" {
		return nil, false, httpx.ErrInstrumentHalted
	}

	var listingID *uuid.UUID
	var supportedTypes, supportedTIF []string
	var venueSymbol, listingBoard string
	var lid uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT l.id, l.venue_symbol, l.supported_types, l.supported_tif, COALESCE(l.board,'')
		FROM listings l WHERE l.instrument_id=$1 AND l.status='ACTIVE' LIMIT 1`, instrID).
		Scan(&lid, &venueSymbol, &supportedTypes, &supportedTIF, &listingBoard)
	if err == nil {
		listingID = &lid
		if !contains(supportedTypes, ot) || !contains(supportedTIF, tif) {
			return nil, false, httpx.ErrUnsupportedType
		}
	}
	if venueSymbol == "" {
		venueSymbol = quikSymbol
	}
	board := firstNonEmpty(listingBoard, instrBoard)

	viaKFB := a.KFB != nil && a.KFB.Configured()

	orderID := id.New()
	remaining := qty
	if _, err := tx.Exec(ctx, `
		INSERT INTO orders (
			id, client_order_id, broker_id, client_id, account_id, instrument_id, listing_id,
			side, order_type, time_in_force, quantity, price, stop_price, remaining_quantity,
			status, submitted_by, client_code, symbol, board)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'NEW',$15,$16,$17,$18)`,
		orderID, req.ClientOrderID, brokerID, clientID, acctID, instrID, listingID,
		side, ot, tif, qty, nilIfZero(px), nilIfZero(stop), remaining, p.UserID,
		clientCode, venueSymbol, board); err != nil {
		return nil, false, fmt.Errorf("insert order: %w", err)
	}
	if err := a.orderEvent(ctx, tx, orderID, "", orders.New, p.UserID); err != nil {
		return nil, false, fmt.Errorf("order event: %w", err)
	}

	if err := a.moveOrder(ctx, tx, orderID, orders.New, orders.Validating, p.UserID); err != nil {
		return nil, false, fmt.Errorf("move NEW->VALIDATING: %w", err)
	}
	if (ot == "LIMIT" || ot == "STOP_LIMIT") && px.IsZero() {
		_ = a.moveOrder(ctx, tx, orderID, orders.Validating, orders.Rejected, p.UserID)
		_, _ = tx.Exec(ctx, `UPDATE orders SET reject_reason='PRICE_REQUIRED' WHERE id=$1`, orderID)
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		body, _ := a.orderJSON(ctx, orderID)
		return body, false, nil
	}
	if err := a.moveOrder(ctx, tx, orderID, orders.Validating, orders.Validated, p.UserID); err != nil {
		return nil, false, err
	}

	notional := qty
	if !px.IsZero() {
		notional = qty.Mul(px)
	} else {
		notional = qty.Mul(decimal.NewFromInt(100))
	}
	if err := a.preTradeRisk(ctx, tx, clientID, acctID, instrID, p.UserID, side, qty, notional, currency); err != nil {
		var api httpx.APIError
		if !errors.As(err, &api) {
			return nil, false, fmt.Errorf("pre-trade risk: %w", err)
		}
		_ = a.moveOrder(ctx, tx, orderID, orders.Validated, orders.Rejected, p.UserID)
		_, _ = tx.Exec(ctx, `UPDATE orders SET reject_reason=$2 WHERE id=$1`, orderID, api.Code)
		if err2 := tx.Commit(ctx); err2 != nil {
			return nil, false, err2
		}
		body, _ := a.orderJSON(ctx, orderID)
		return body, false, nil
	}

	if err := a.moveOrder(ctx, tx, orderID, orders.Validated, orders.Pending, p.UserID); err != nil {
		return nil, false, err
	}

	if !viaKFB {
		if side == "BUY" {
			if err := a.postLedger(ctx, tx, acctID, currency, "ORDER", orderID, "Reserve for buy", "order-reserve-"+orderID.String(), []ledgerLine{
				moneyLine("AVAILABLE", "RESERVE", notional, decimal.Zero),
				moneyLine("BLOCKED", "RESERVE", decimal.Zero, notional),
			}, &p.UserID); err != nil {
				_ = a.moveOrder(ctx, tx, orderID, orders.Pending, orders.Rejected, p.UserID)
				_, _ = tx.Exec(ctx, `UPDATE orders SET reject_reason='INSUFFICIENT_FUNDS' WHERE id=$1`, orderID)
				if err2 := tx.Commit(ctx); err2 != nil {
					return nil, false, err2
				}
				body, _ := a.orderJSON(ctx, orderID)
				return body, false, nil
			}
		} else {
			if err := a.blockPosition(ctx, tx, acctID, instrID, qty); err != nil {
				_ = a.moveOrder(ctx, tx, orderID, orders.Pending, orders.Rejected, p.UserID)
				_, _ = tx.Exec(ctx, `UPDATE orders SET reject_reason='INSUFFICIENT_POSITION' WHERE id=$1`, orderID)
				if err2 := tx.Commit(ctx); err2 != nil {
					return nil, false, err2
				}
				body, _ := a.orderJSON(ctx, orderID)
				return body, false, nil
			}
		}
	}

	if err := a.moveOrder(ctx, tx, orderID, orders.Pending, orders.Routing, p.UserID); err != nil {
		return nil, false, err
	}
	ad, ok := a.Adapters.PickKind("EXCHANGE", "WEIGHTED")
	if !ok && !viaKFB {
		ad, ok = a.Adapters.Pick("WEIGHTED")
	}
	if !ok {
		if !viaKFB && side == "BUY" {
			_ = a.releaseBuyReserve(ctx, tx, acctID, currency, orderID, notional, p.UserID)
		}
		_ = a.moveOrder(ctx, tx, orderID, orders.Routing, orders.Failed, p.UserID)
		_, _ = tx.Exec(ctx, `UPDATE orders SET reject_reason='ADAPTER_UNAVAILABLE' WHERE id=$1`, orderID)
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		body, _ := a.orderJSON(ctx, orderID)
		return body, false, nil
	}
	if err := a.moveOrder(ctx, tx, orderID, orders.Routing, orders.Sent, p.UserID); err != nil {
		return nil, false, err
	}
	ack, err := ad.SendOrder(ctx, adapters.VenueOrder{
		ClientOrderID: req.ClientOrderID,
		VenueSymbol:   venueSymbol,
		Account:       accountCode,
		ClientCode:    clientCode,
		Board:         board,
		Side:          side,
		Type:          ot,
		TIF:           tif,
		Quantity:      qty,
		Price:         px,
		StopPrice:     stop,
	})
	a.Adapters.Release(ad.Code())
	if err != nil {
		_ = a.moveOrder(ctx, tx, orderID, orders.Sent, orders.Failed, p.UserID)
		_, _ = tx.Exec(ctx, `UPDATE orders SET reject_reason='ADAPTER_ERROR' WHERE id=$1`, orderID)
		if err2 := tx.Commit(ctx); err2 != nil {
			return nil, false, err2
		}
		body, _ := a.orderJSON(ctx, orderID)
		return body, false, nil
	}
	_, _ = tx.Exec(ctx, `UPDATE orders SET venue_order_id=$2 WHERE id=$1`, orderID, ack.VenueOrderID)
	if err := a.moveOrder(ctx, tx, orderID, orders.Sent, orders.Acknowledged, p.UserID); err != nil {
		return nil, false, err
	}

	if ad.Kind() == "MOCK" {
		fill := adapters.ImmediateFill(adapters.VenueOrder{ClientOrderID: req.ClientOrderID, Quantity: qty, Price: px})
		if err := a.applyFill(ctx, tx, orderID, acctID, clientID, instrID, side, currency, fill, p.UserID); err != nil {
			return nil, false, err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1,'order',$2,'order.updated', jsonb_build_object('order_id',$3::text,'client_order_id',$4::text))`,
		id.New(), orderID, orderID.String(), req.ClientOrderID); err != nil {
		return nil, false, fmt.Errorf("outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit order: %w", err)
	}
	telemetry.OrdersTotal.Inc()
	a.Audit.Write(ctx, "ORDER_CREATED", "order", &orderID, nil, req)
	body, _ := a.orderJSON(ctx, orderID)
	return body, false, nil
}

func (a *App) releaseBuyReserve(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, currency string, orderID uuid.UUID, notional decimal.Decimal, actor uuid.UUID) error {
	return a.postLedger(ctx, tx, accountID, currency, "ORDER", orderID, "Release unused reserve", "order-release-"+orderID.String(), []ledgerLine{
		moneyLine("BLOCKED", "RELEASE", notional, decimal.Zero),
		moneyLine("AVAILABLE", "RELEASE", decimal.Zero, notional),
	}, &actor)
}

func (a *App) applyFill(ctx context.Context, tx pgx.Tx, orderID, accountID, clientID, instrID uuid.UUID, side, currency string, fill adapters.ExecutionReport, actor uuid.UUID) error {
	if err := a.ensureHouseBuckets(ctx, tx, accountID, currency); err != nil {
		return err
	}
	gross := fill.LastQty.Mul(fill.LastPx)
	commission, tariffID, calc, err := a.calcCommission(ctx, tx, accountID, gross, currency)
	if err != nil {
		return err
	}
	net := gross.Add(commission)
	if side == "SELL" {
		net = gross.Sub(commission)
	}
	execID := id.New()
	settle := time.Now().UTC().Add(48 * time.Hour).Format("2006-01-02")
	if _, err := tx.Exec(ctx, `
		INSERT INTO executions (id, exec_id, order_id, client_id, account_id, instrument_id, side, quantity, price, gross_amount, commission, net_amount, execution_time, settlement_date)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,now(),$13)`,
		execID, fill.ExecID, orderID, clientID, accountID, instrID, side, fill.LastQty, fill.LastPx, gross, commission, net, settle); err != nil {
		return err
	}
	tradeID := id.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO trades (id, execution_id, order_id, client_id, account_id, instrument_id, side, quantity, price, gross_amount, commission, net_amount, trade_date, settlement_date)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,CURRENT_DATE,$13)`,
		tradeID, execID, orderID, clientID, accountID, instrID, side, fill.LastQty, fill.LastPx, gross, commission, net, settle); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO commissions (id, trade_id, tariff_id, amount, currency, calculation)
		VALUES ($1,$2,$3,$4,$5,$6)`, id.New(), tradeID, tariffID, commission, currency, calc); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO settlements (id, trade_id, account_id, instrument_id, trade_date, settlement_date, cash_status, securities_status, cash_amount, quantity)
		VALUES ($1,$2,$3,$4,CURRENT_DATE,$5,'PENDING','PENDING',$6,$7)`,
		id.New(), tradeID, accountID, instrID, settle, net, fill.LastQty); err != nil {
		return err
	}

	if side == "BUY" {
		if err := a.postLedger(ctx, tx, accountID, currency, "TRADE", tradeID, "Buy fill", "trade-"+tradeID.String(), []ledgerLine{
			moneyLine("BLOCKED", "BUY", gross, decimal.Zero),
			moneyLine("CLEARING", "BUY", decimal.Zero, gross),
		}, &actor); err != nil {
			if err := a.ensureHouseBuckets(ctx, tx, accountID, currency); err == nil {
				_ = a.postLedger(ctx, tx, accountID, currency, "TRADE", tradeID, "Buy fill", "trade-"+tradeID.String(), []ledgerLine{
					moneyLine("BLOCKED", "BUY", gross, decimal.Zero),
					moneyLine("SETTLED", "BUY", decimal.Zero, gross),
				}, &actor)
			}
		}
		if commission.IsPositive() {
			_ = a.postLedger(ctx, tx, accountID, currency, "COMMISSION", tradeID, "Commission", "comm-"+tradeID.String(), []ledgerLine{
				moneyLine("AVAILABLE", "COMMISSION", commission, decimal.Zero),
				moneyLine("SETTLED", "COMMISSION", decimal.Zero, commission),
			}, &actor)
		}
		if err := a.applyPositionDelta(ctx, tx, accountID, instrID, fill.LastQty, fill.LastPx, "TRADE", tradeID); err != nil {
			return err
		}
	} else {
		if err := a.postLedger(ctx, tx, accountID, currency, "TRADE", tradeID, "Sell fill", "trade-"+tradeID.String(), []ledgerLine{
			moneyLine("AVAILABLE", "SELL", decimal.Zero, gross),
			moneyLine("SETTLED", "SELL", gross, decimal.Zero),
		}, &actor); err != nil {
			return err
		}
		if commission.IsPositive() {
			_ = a.postLedger(ctx, tx, accountID, currency, "COMMISSION", tradeID, "Commission", "comm-"+tradeID.String(), []ledgerLine{
				moneyLine("AVAILABLE", "COMMISSION", commission, decimal.Zero),
				moneyLine("SETTLED", "COMMISSION", decimal.Zero, commission),
			}, &actor)
		}
		if err := a.applyPositionDelta(ctx, tx, accountID, instrID, fill.LastQty.Neg(), fill.LastPx, "TRADE", tradeID); err != nil {
			return err
		}
	}

	newStatus := orders.Filled
	if !fill.LeavesQty.IsZero() {
		newStatus = orders.PartiallyFilled
	}
	var from string
	_ = tx.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1`, orderID).Scan(&from)
	if err := a.moveOrder(ctx, tx, orderID, orders.Status(from), newStatus, actor); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE orders SET filled_quantity=filled_quantity+$2, remaining_quantity=remaining_quantity-$2, avg_price=$3, version=version+1
		WHERE id=$1`, orderID, fill.LastQty, fill.LastPx)
	return err
}

func (a *App) ensureHouseBuckets(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, currency string) error {
	for _, b := range []string{"CLEARING", "COMMISSION", "TAX"} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO cash_accounts (id, trading_account_id, currency, bucket, owner_type)
			VALUES ($1,$2,$3,$4,'HOUSE')
			ON CONFLICT (trading_account_id, currency, bucket) DO NOTHING`, id.New(), accountID, currency, b); err != nil {
			return err
		}
	}
	return a.ensureCashAccounts(ctx, tx, accountID, currency)
}

func (a *App) moveOrder(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, from, to orders.Status, actor uuid.UUID) error {
	if err := orders.Transition(from, to); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE orders SET status=$2, version=version+1, updated_at=now() WHERE id=$1 AND status=$3`, orderID, to, from)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return httpx.ErrInvalidTransition
	}
	return a.orderEvent(ctx, tx, orderID, string(from), to, actor)
}

func (a *App) orderEvent(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, from string, to orders.Status, actor uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO order_events (id, order_id, from_status, to_status, actor_id) VALUES ($1,$2,$3,$4,$5)`,
		id.New(), orderID, nilIfEmpty(from), to, nilIfUUID(actor))
	return err
}

func nilIfUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func (a *App) orderJSON(ctx context.Context, orderID uuid.UUID) (map[string]any, error) {
	row := a.DB.QueryRow(ctx, `
		SELECT o.id, o.client_order_id, o.account_id, o.instrument_id, o.side, o.order_type, o.time_in_force,
		       o.quantity, o.price, o.filled_quantity, o.remaining_quantity, o.status, o.reject_reason, o.created_at,
		       COALESCE(o.symbol, i.ticker), COALESCE(o.board,'')
		FROM orders o JOIN instruments i ON i.id=o.instrument_id WHERE o.id=$1`, orderID)
	var (
		idv, acct, instr            uuid.UUID
		coid, side, ot, tif, status string
		qty, filled, remain         decimal.Decimal
		price                       *decimal.Decimal
		reason                      *string
		created                     time.Time
		symbol, board               string
	)
	if err := row.Scan(&idv, &coid, &acct, &instr, &side, &ot, &tif, &qty, &price, &filled, &remain, &status, &reason, &created, &symbol, &board); err != nil {
		return nil, err
	}
	m := map[string]any{
		"id": idv, "client_order_id": coid, "account_id": acct, "instrument_id": instr,
		"side": side, "order_type": ot, "time_in_force": tif, "quantity": qty.String(),
		"filled_quantity": filled.String(), "remaining_quantity": remain.String(),
		"status": status, "created_at": created.UTC(), "symbol": symbol, "board": board,
		"can_cancel": orderCanCancel(status),
	}
	if price != nil {
		m["price"] = price.String()
	}
	if reason != nil {
		m["reject_reason"] = *reason
	}
	return m, nil
}

func (a *App) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	idv, err := chiID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r.Context())
	if err := a.requireOrderAccess(r.Context(), p, idv); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	body, err := a.orderJSON(r.Context(), idv)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	httpx.WriteJSON(w, 200, body)
}

func (a *App) handleListOrders(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	var scope any
	if p.ClientID != nil {
		scope = *p.ClientID
	}
	rows, err := a.DB.Query(r.Context(), `
		SELECT o.id, o.client_order_id, o.account_id, o.instrument_id, o.side, o.order_type, o.quantity, o.price,
		       o.status, o.created_at, o.filled_quantity, COALESCE(o.symbol, i.ticker), COALESCE(o.board,''), o.reject_reason
		FROM orders o
		JOIN instruments i ON i.id=o.instrument_id
		WHERE ($1::uuid IS NULL OR o.client_id=$1)
		ORDER BY o.created_at DESC LIMIT 200`, scope)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	var items []map[string]any
	for rows.Next() {
		var idv, acct, instr uuid.UUID
		var coid, side, ot, status, symbol, board string
		var qty, filled decimal.Decimal
		var price *decimal.Decimal
		var reason *string
		var created time.Time
		if err := rows.Scan(&idv, &coid, &acct, &instr, &side, &ot, &qty, &price, &status, &created, &filled, &symbol, &board, &reason); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		item := map[string]any{"id": idv, "client_order_id": coid, "account_id": acct, "instrument_id": instr,
			"side": side, "order_type": ot, "quantity": qty.String(), "filled_quantity": filled.String(),
			"status": status, "created_at": created.UTC(), "symbol": symbol, "board": board,
			"can_cancel": orderCanCancel(status)}
		if price != nil {
			item["price"] = price.String()
		}
		if reason != nil && *reason != "" {
			item["reject_reason"] = *reason
		}
		items = append(items, item)
	}
	if items == nil {
		items = []map[string]any{}
	}
	httpx.WriteJSON(w, 200, map[string]any{"items": items})
}

func (a *App) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	idv, err := chiID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r.Context())
	if err := a.requireOrderAccess(r.Context(), p, idv); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var status, side, currency, venueID string
	var acct uuid.UUID
	var remain decimal.Decimal
	var price *decimal.Decimal
	if err := tx.QueryRow(r.Context(), `
		SELECT o.status, o.side, o.account_id, o.remaining_quantity, o.price, a.currency, COALESCE(o.venue_order_id,'')
		FROM orders o JOIN trading_accounts a ON a.id=o.account_id WHERE o.id=$1 FOR UPDATE`, idv).
		Scan(&status, &side, &acct, &remain, &price, &currency, &venueID); err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	from := orders.Status(status)
	if from == orders.Sent || from == orders.Acknowledged || from == orders.PartiallyFilled || venueID != "" {
		if err := a.moveOrder(r.Context(), tx, idv, from, orders.CancelPending, p.UserID); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		a.Audit.Write(r.Context(), "ORDER_CANCEL_REQUESTED", "order", &idv, nil, nil)
		body, _ := a.orderJSON(r.Context(), idv)
		httpx.WriteJSON(w, 200, body)
		return
	}
	if err := a.moveOrder(r.Context(), tx, idv, from, orders.Cancelled, p.UserID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if side == "BUY" && remain.IsPositive() {
		px := decimal.NewFromInt(100)
		if price != nil {
			px = *price
		}
		_ = a.releaseBuyReserve(r.Context(), tx, acct, currency, idv, remain.Mul(px), p.UserID)
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.Audit.Write(r.Context(), "ORDER_CANCELLED", "order", &idv, nil, nil)
	body, _ := a.orderJSON(r.Context(), idv)
	httpx.WriteJSON(w, 200, body)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func nilIfZero(d decimal.Decimal) any {
	if d.IsZero() {
		return nil
	}
	return d
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
