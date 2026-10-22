package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"statebroker/sbis/internal/gateway"
	"statebroker/sbis/internal/orders"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/id"
)

type gwOrderReq struct {
	AccountID     string         `json:"account_id"`
	ClientCode    string         `json:"client_code"`
	ClientOrderID string         `json:"client_order_id"`
	Symbol        string         `json:"symbol"`
	Board         string         `json:"board"`
	Side          string         `json:"side"`
	OrderType     string         `json:"order_type"`
	TimeInForce   string         `json:"time_in_force"`
	Quantity      gateway.Number `json:"quantity"`
	Price         gateway.Number `json:"price"`
}

func decodeGateway[T any](r *http.Request, dst *T) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func (a *App) handleGatewayFirm(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	gateway.JSON(w, 200, map[string]any{"quik_firm": p.QuikFirm, "firm_name": p.FirmName})
}

func (a *App) handleGatewayInstruments(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(r.Context(), `
		SELECT public_id, COALESCE(internal_symbol, isin, ticker), COALESCE(quik_symbol, ticker),
		       COALESCE(board, ''), currency, security_type, status
		FROM instruments
		ORDER BY public_id`)
	if err != nil {
		gateway.WriteErr(w, r, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var pid int64
		var isym, qsym, board, cur, stype, st string
		if err := rows.Scan(&pid, &isym, &qsym, &board, &cur, &stype, &st); err != nil {
			gateway.WriteErr(w, r, err)
			return
		}
		items = append(items, map[string]any{
			"id": pid, "internal_symbol": isym, "quik_symbol": qsym,
			"board": board, "currency": cur, "security_type": stype, "status": st,
		})
	}
	gateway.JSON(w, 200, map[string]any{"instruments": items})
}

func (a *App) handleGatewayAccounts(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	rows, err := a.DB.Query(r.Context(), `
		SELECT COALESCE(a.trading_account, a.account_number),
		       COALESCE(NULLIF(a.client_code,''), COALESCE(c.client_code, '')),
		       a.currency, a.status
		FROM trading_accounts a
		JOIN clients c ON c.id=a.client_id
		WHERE a.broker_id=$1 ORDER BY a.created_at DESC LIMIT 100`, p.BrokerID)
	if err != nil {
		gateway.WriteErr(w, r, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var ta, cc, cur, st string
		if err := rows.Scan(&ta, &cc, &cur, &st); err != nil {
			gateway.WriteErr(w, r, err)
			return
		}
		items = append(items, map[string]any{
			"trading_account": ta, "account_id": ta, "client_code": cc, "currency": cur, "status": st,
		})
	}
	gateway.JSON(w, 200, map[string]any{"accounts": items})
}

func (a *App) handleGatewayCreateOrder(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	if strings.TrimSpace(p.QuikFirm) == "" {
		gateway.Write(w, http.StatusBadRequest, "quik_firm is not set on token", map[string]any{})
		return
	}
	var req gwOrderReq
	if err := decodeGateway(r, &req); err != nil {
		gateway.Write(w, http.StatusBadRequest, "invalid JSON or unknown field", map[string]any{"details": err.Error()})
		return
	}
	reqID := httpx.RequestIDFrom(r.Context())
	out, created, err := a.gatewaySubmit(r.Context(), p, req, reqID)
	if err != nil {
		gateway.WriteErr(w, r, err)
		return
	}
	if created {
		gateway.JSON(w, http.StatusCreated, map[string]any{"order": out, "idempotent": false})
		return
	}
	gateway.JSON(w, http.StatusOK, map[string]any{"order": out, "idempotent": true})
}

func (a *App) gatewaySubmit(ctx context.Context, p httpx.Principal, req gwOrderReq, reqID string) (map[string]any, bool, error) {
	side := strings.ToUpper(strings.TrimSpace(req.Side))
	ot := strings.ToUpper(strings.TrimSpace(req.OrderType))
	tif := strings.ToUpper(strings.TrimSpace(req.TimeInForce))
	if tif == "" {
		tif = "DAY"
	}
	if req.AccountID == "" || req.ClientCode == "" || req.ClientOrderID == "" || req.Symbol == "" || req.Board == "" {
		return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "missing required field")
	}
	if len(req.ClientOrderID) > 128 {
		return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "client_order_id too long")
	}
	if side != "BUY" && side != "SELL" {
		return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "side must be BUY or SELL")
	}
	if ot != "LIMIT" && ot != "MARKET" {
		return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "order_type must be LIMIT or MARKET")
	}
	if tif != "DAY" && tif != "IOC" && tif != "FOK" && tif != "GTC" {
		return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "invalid time_in_force")
	}
	qty, err := req.Quantity.Decimal()
	if err != nil || !qty.IsPositive() {
		return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "quantity must be > 0")
	}
	var px decimal.Decimal
	if ot == "LIMIT" {
		px, err = req.Price.Decimal()
		if err != nil || !px.IsPositive() {
			return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "price is required for LIMIT and must be > 0")
		}
	} else if strings.TrimSpace(req.Price.String()) != "" {
		px, err = req.Price.Decimal()
		if err != nil {
			return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "invalid price")
		}
	}

	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	var existing uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM orders WHERE broker_id=$1 AND client_order_id=$2`, p.BrokerID, req.ClientOrderID).Scan(&existing)
	if err == nil {
		_ = tx.Rollback(ctx)
		body, e := a.gatewayOrderJSON(ctx, p.BrokerID, existing)
		return body, false, e
	}
	if err != pgx.ErrNoRows {
		return nil, false, err
	}

	var acctID, clientID uuid.UUID
	var acctStatus, currency, clientCode string
	err = tx.QueryRow(ctx, `
		SELECT a.id, a.client_id, a.status, a.currency,
		       COALESCE(NULLIF(a.client_code,''), COALESCE(c.client_code,''))
		FROM trading_accounts a
		JOIN clients c ON c.id=a.client_id
		WHERE a.broker_id=$1 AND (a.trading_account=$2 OR a.account_number=$2)
		FOR UPDATE OF a`, p.BrokerID, req.AccountID).
		Scan(&acctID, &clientID, &acctStatus, &currency, &clientCode)
	if err != nil {
		return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "unknown account_id")
	}
	if acctStatus != "ACTIVE" {
		return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "account is not active")
	}
	if clientCode != "" && clientCode != req.ClientCode {
		approved, approvalErr := a.approvedLoadParticipant(ctx, tx, p, req)
		if approvalErr != nil {
			return nil, false, approvalErr
		}
		if !approved {
			return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "client_code does not match account")
		}
	}

	var instrID uuid.UUID
	var instrStatus string
	err = tx.QueryRow(ctx, `
		SELECT i.id, i.status
		FROM instruments i
		LEFT JOIN listings l ON l.instrument_id = i.id
		WHERE i.status='ACTIVE'
		  AND (i.quik_symbol=$1 OR i.internal_symbol=$1 OR i.ticker=$1 OR i.isin=$1)
		  AND (i.board=$2 OR l.board=$2)
		LIMIT 1`, req.Symbol, req.Board).
		Scan(&instrID, &instrStatus)
	if err != nil {
		return nil, false, httpx.E(http.StatusBadRequest, "VALIDATION_ERROR", "unknown symbol+board")
	}

	var listingID *uuid.UUID
	var lid uuid.UUID
	var venueSymbol string
	if err := tx.QueryRow(ctx, `
		SELECT id, venue_symbol FROM listings
		WHERE instrument_id=$1 AND status='ACTIVE' AND (board=$2 OR board IS NULL OR board='')
		LIMIT 1`, instrID, req.Board).Scan(&lid, &venueSymbol); err == nil {
		listingID = &lid
	}
	if venueSymbol == "" {
		venueSymbol = req.Symbol
	}

	orderID := id.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO orders (
			id, client_order_id, broker_id, client_id, account_id, instrument_id, listing_id,
			side, order_type, time_in_force, quantity, price, remaining_quantity, status,
			client_code, symbol, board, quik_firm, request_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'QUEUED',$14,$15,$16,$17,$18)`,
		orderID, req.ClientOrderID, p.BrokerID, clientID, acctID, instrID, listingID,
		side, ot, tif, qty, nilIfZero(px), qty,
		req.ClientCode, req.Symbol, req.Board, p.QuikFirm, reqID); err != nil {
		return nil, false, err
	}
	if err := a.orderEvent(ctx, tx, orderID, "", orders.Queued, uuid.Nil); err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1,'order',$2,'gateway.order.route', jsonb_build_object('order_id',$3::text,'client_order_id',$4::text))`,
		id.New(), orderID, orderID.String(), req.ClientOrderID); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	a.Audit.Write(ctx, "ORDER_CREATED", "order", &orderID, nil, map[string]any{
		"client_order_id": req.ClientOrderID, "source": "broker_api",
	})
	body, err := a.gatewayOrderJSON(ctx, p.BrokerID, orderID)
	return body, true, err
}

func (a *App) handleGatewayListOrders(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	rows, err := a.DB.Query(r.Context(), `
		SELECT id, client_order_id, status, symbol, board, side, quantity, price, filled_quantity, quik_firm, request_id
		FROM orders WHERE broker_id=$1 ORDER BY created_at DESC LIMIT 100`, p.BrokerID)
	if err != nil {
		gateway.WriteErr(w, r, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var idv uuid.UUID
		var coid, status, symbol, board, side string
		var qty, filled decimal.Decimal
		var price *decimal.Decimal
		var quikFirm, reqID *string
		if err := rows.Scan(&idv, &coid, &status, &symbol, &board, &side, &qty, &price, &filled, &quikFirm, &reqID); err != nil {
			gateway.WriteErr(w, r, err)
			return
		}
		item := map[string]any{
			"id": idv, "client_order_id": coid, "status": gateway.PublicStatus(status),
			"symbol": symbol, "board": board, "side": side, "quantity": qty.String(),
			"filled_quantity": filled.String(),
		}
		if price != nil {
			item["price"] = price.String()
		}
		if quikFirm != nil {
			item["quik_firm"] = *quikFirm
		}
		if reqID != nil {
			item["request_id"] = *reqID
		}
		items = append(items, item)
	}
	gateway.JSON(w, 200, map[string]any{"orders": items})
}

func (a *App) handleGatewayGetOrder(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	idv, err := chiID(r, "id")
	if err != nil {
		gateway.Write(w, http.StatusBadRequest, "invalid id", map[string]any{})
		return
	}
	order, err := a.gatewayOrderJSON(r.Context(), p.BrokerID, idv)
	if err != nil {
		gateway.Write(w, http.StatusNotFound, "order not found", map[string]any{})
		return
	}
	events := []map[string]any{}
	erows, err := a.DB.Query(r.Context(), `
		SELECT from_status, to_status, created_at FROM order_events WHERE order_id=$1 ORDER BY created_at`, idv)
	if err == nil {
		defer erows.Close()
		for erows.Next() {
			var from *string
			var to string
			var ts time.Time
			if erows.Scan(&from, &to, &ts) == nil {
				events = append(events, map[string]any{"from": from, "to": gateway.PublicStatus(to), "at": ts.UTC()})
			}
		}
	}
	trades := a.gatewayTrades(r.Context(), p.BrokerID, &idv)
	gateway.JSON(w, 200, map[string]any{"order": order, "events": events, "trades": trades})
}

func (a *App) handleGatewayCancelOrder(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	idv, err := chiID(r, "id")
	if err != nil {
		gateway.Write(w, http.StatusBadRequest, "invalid id", map[string]any{})
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		gateway.WriteErr(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var status string
	var broker uuid.UUID
	err = tx.QueryRow(r.Context(), `SELECT status, broker_id FROM orders WHERE id=$1 FOR UPDATE`, idv).Scan(&status, &broker)
	if err != nil || broker != p.BrokerID {
		gateway.Write(w, http.StatusNotFound, "order not found", map[string]any{})
		return
	}
	if !gateway.CanCancel(status) {
		gateway.Write(w, http.StatusBadRequest, "cannot cancel from status "+status, map[string]any{"status": status})
		return
	}
	if err := a.moveOrder(r.Context(), tx, idv, orders.Status(status), orders.CancelPending, uuid.Nil); err != nil {
		gateway.WriteErr(w, r, err)
		return
	}
	if _, err := tx.Exec(r.Context(), `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1,'order',$2,'gateway.order.cancel', jsonb_build_object('order_id',$3::text))`, id.New(), idv, idv.String()); err != nil {
		gateway.WriteErr(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		gateway.WriteErr(w, r, err)
		return
	}
	a.Audit.Write(r.Context(), "ORDER_CANCEL_REQUESTED", "order", &idv, nil, nil)
	body, _ := a.gatewayOrderJSON(r.Context(), p.BrokerID, idv)
	gateway.JSON(w, 200, map[string]any{"order": body})
}

func (a *App) handleGatewayTrades(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	gateway.JSON(w, 200, map[string]any{"trades": a.gatewayTrades(r.Context(), p.BrokerID, nil)})
}

func (a *App) handleGatewayPositions(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	var taken *time.Time
	var note *string
	var payload []byte
	err := a.DB.QueryRow(r.Context(), `
		SELECT taken_at, note, payload FROM position_snapshots
		WHERE broker_id=$1 ORDER BY taken_at DESC LIMIT 1`, p.BrokerID).Scan(&taken, &note, &payload)
	if err == nil && payload != nil {
		var items any
		_ = json.Unmarshal(payload, &items)
		gateway.JSON(w, 200, map[string]any{"positions": items, "taken_at": taken, "note": note})
		return
	}
	rows, err := a.DB.Query(r.Context(), `
		SELECT COALESCE(a.trading_account, a.account_number),
		       COALESCE(NULLIF(a.client_code,''), COALESCE(c.client_code,'')),
		       COALESCE(i.quik_symbol, i.ticker), COALESCE(i.board,''),
		       p.quantity, p.avg_price, p.available_qty, p.blocked_qty
		FROM positions p
		JOIN trading_accounts a ON a.id=p.account_id
		JOIN clients c ON c.id=a.client_id
		JOIN instruments i ON i.id=p.instrument_id
		WHERE a.broker_id=$1
		ORDER BY a.trading_account, i.ticker
		LIMIT 100`, p.BrokerID)
	if err != nil {
		gateway.WriteErr(w, r, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var acct, cc, sym, board string
		var qty, avg, avail, blocked decimal.Decimal
		if err := rows.Scan(&acct, &cc, &sym, &board, &qty, &avg, &avail, &blocked); err != nil {
			gateway.WriteErr(w, r, err)
			return
		}
		items = append(items, map[string]any{
			"account_id": acct, "client_code": cc, "symbol": sym, "board": board,
			"quantity": qty.String(), "avg_price": avg.String(),
			"available_qty": avail.String(), "blocked_qty": blocked.String(),
		})
	}
	msg := ""
	if len(items) == 0 {
		msg = "position snapshot is empty; this is normal until the operator runs the first sync"
	}
	gateway.JSON(w, 200, map[string]any{"positions": items, "note": msg})
}

func (a *App) gatewayOrderJSON(ctx context.Context, brokerID, orderID uuid.UUID) (map[string]any, error) {
	row := a.DB.QueryRow(ctx, `
		SELECT id, client_order_id, status, COALESCE(symbol,''), COALESCE(board,''), side, quantity, price,
		       filled_quantity, COALESCE(quik_firm,''), COALESCE(request_id,''), avg_price, venue_order_id, reject_reason,
		       COALESCE(client_code,''), order_type, time_in_force
		FROM orders WHERE id=$1 AND broker_id=$2`, orderID, brokerID)
	return scanGatewayOrder(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanGatewayOrder(row scanner) (map[string]any, error) {
	var (
		idv                                                                     uuid.UUID
		coid, status, symbol, board, side, quikFirm, reqID, clientCode, ot, tif string
		qty, filled                                                             decimal.Decimal
		price, avg                                                              *decimal.Decimal
		venue, reason                                                           *string
	)
	if err := row.Scan(&idv, &coid, &status, &symbol, &board, &side, &qty, &price, &filled, &quikFirm, &reqID, &avg, &venue, &reason, &clientCode, &ot, &tif); err != nil {
		return nil, err
	}
	m := map[string]any{
		"id": idv, "client_order_id": coid, "status": gateway.PublicStatus(status),
		"symbol": symbol, "board": board, "side": side,
		"quantity": qty.String(), "filled_quantity": filled.String(),
		"quik_firm": quikFirm, "request_id": reqID,
		"client_code": clientCode, "order_type": ot, "time_in_force": tif,
	}
	if price != nil {
		m["price"] = price.String()
	}
	if avg != nil {
		m["avg_price"] = avg.String()
	}
	if venue != nil {
		m["quik_order_id"] = *venue
	}
	if reason != nil {
		m["reject_reason"] = *reason
	}
	return m, nil
}

func (a *App) gatewayTrades(ctx context.Context, brokerID uuid.UUID, orderID *uuid.UUID) []map[string]any {
	q := `
		SELECT t.id, t.order_id, o.client_order_id, COALESCE(a.trading_account, a.account_number),
		       COALESCE(i.quik_symbol, i.ticker), t.side, t.quantity, t.price, t.gross_amount, t.created_at
		FROM trades t
		JOIN orders o ON o.id=t.order_id
		JOIN trading_accounts a ON a.id=t.account_id
		JOIN instruments i ON i.id=t.instrument_id
		WHERE o.broker_id=$1`
	args := []any{brokerID}
	if orderID != nil {
		q += ` AND t.order_id=$2`
		args = append(args, *orderID)
	}
	q += ` ORDER BY t.created_at DESC LIMIT 100`
	rows, err := a.DB.Query(ctx, q, args...)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var idv, oid uuid.UUID
		var coid, acct, sym, side string
		var qty, px, gross decimal.Decimal
		var ts time.Time
		if rows.Scan(&idv, &oid, &coid, &acct, &sym, &side, &qty, &px, &gross, &ts) != nil {
			continue
		}
		items = append(items, map[string]any{
			"id": idv, "order_id": oid, "client_order_id": coid, "account_id": acct,
			"symbol": sym, "side": side, "quantity": qty.String(), "price": px.String(),
			"gross_amount": gross.String(), "created_at": ts.UTC(),
		})
	}
	return items
}
