package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/id"
)

type marketItem struct {
	ID            string `json:"id"`
	Ticker        string `json:"ticker"`
	ISIN          string `json:"isin"`
	Name          string `json:"name"`
	Board         string `json:"board"`
	Currency      string `json:"currency"`
	Status        string `json:"status"`
	SecurityType  string `json:"security_type"`
	Last          string `json:"last,omitempty"`
	Bid           string `json:"bid,omitempty"`
	Ask           string `json:"ask,omitempty"`
	Volume        string `json:"volume,omitempty"`
	Source        string `json:"source"`
	VenueSymbol   string `json:"venue_symbol"`
}

func (a *App) handleMarket(w http.ResponseWriter, r *http.Request) {
	items, err := a.marketBoard(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"items": items})
}

func (a *App) handleMarketBook(w http.ResponseWriter, r *http.Request) {
	idv, err := chiID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	bids, asks, err := a.orderBook(r.Context(), idv)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"instrument_id": idv, "bids": bids, "asks": asks})
}

func (a *App) marketBoard(ctx context.Context) ([]marketItem, error) {
	if a.KFB != nil && a.KFB.Configured() {
		snap := a.KFB.Snapshot()
		stale := snap.LastCheckAt == nil || time.Since(*snap.LastCheckAt) > 12*time.Second
		if stale || len(snap.Instruments) == 0 {
			snap = a.KFB.Refresh(ctx)
			a.syncAdapterRow(ctx, "KFB", "EXCHANGE", "Кыргызская фондовая биржа", snap.Health, 20)
		}
		if len(snap.Instruments) > 0 {
			out := make([]marketItem, 0, len(snap.Instruments))
			for _, row := range snap.Instruments {
				item, err := a.marketItemFromKFB(ctx, row)
				if err != nil {
					continue
				}
				out = append(out, item)
			}
			if len(out) > 0 {
				return out, nil
			}
		}
	}
	return a.localMarketBoard(ctx)
}

func (a *App) marketItemFromKFB(ctx context.Context, row map[string]any) (marketItem, error) {
	fields := parseKFBInstrument(row)
	if fields.Symbol == "" {
		return marketItem{}, fmt.Errorf("missing symbol")
	}
	instrID, err := a.ensureKFBInstrument(ctx, fields)
	if err != nil {
		return marketItem{}, err
	}
	item := marketItem{
		ID:           instrID.String(),
		Ticker:       fields.Symbol,
		ISIN:         fields.ISIN,
		Name:         fields.Name,
		Board:        fields.Board,
		Currency:     fields.Currency,
		Status:       fields.Status,
		SecurityType: fields.SecurityType,
		Source:       "KFB",
		VenueSymbol:  fields.Symbol,
	}
	a.attachSnapshot(ctx, instrID, &item)
	return item, nil
}

type kfbInstrument struct {
	Symbol       string
	Board        string
	Currency     string
	Status       string
	Name         string
	ISIN         string
	SecurityType string
}

func parseKFBInstrument(row map[string]any) kfbInstrument {
	symbol := firstNonEmpty(
		mapString(row, "quik_symbol"),
		mapString(row, "ticker"),
		mapString(row, "symbol"),
		mapString(row, "internal_symbol"),
		mapString(row, "isin"),
	)
	if i := strings.LastIndex(symbol, "/"); i >= 0 && i+1 < len(symbol) {
		symbol = symbol[i+1:]
	}
	isin := mapString(row, "isin")
	if isin == "" || strings.Contains(isin, "/") {
		isin = symbol
	}
	name := firstNonEmpty(mapString(row, "name"), mapString(row, "short_name"), symbol)
	cur := strings.ToUpper(firstNonEmpty(mapString(row, "currency"), "KGS"))
	if len(cur) != 3 {
		cur = "KGS"
	}
	st := mapInstrumentStatus(mapString(row, "status"))
	return kfbInstrument{
		Symbol:       symbol,
		Board:        mapString(row, "board"),
		Currency:     cur,
		Status:       st,
		Name:         name,
		ISIN:         isin,
		SecurityType: firstNonEmpty(mapString(row, "security_type"), "CS"),
	}
}

func mapInstrumentStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "SUSPENDED":
		return "SUSPENDED"
	case "HALTED":
		return "HALTED"
	case "DELISTED":
		return "DELISTED"
	case "ACTIVE", "ONLINE", "TRADING", "OPEN", "":
		return "ACTIVE"
	default:
		if s == "" {
			return "ACTIVE"
		}
		return "ACTIVE"
	}
}

func (a *App) ensureKFBInstrument(ctx context.Context, f kfbInstrument) (uuid.UUID, error) {
	if a.DB == nil {
		return uuid.Nil, httpx.ErrNotFound
	}
	var curOK bool
	_ = a.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM currencies WHERE code=$1)`, f.Currency).Scan(&curOK)
	if !curOK {
		f.Currency = "KGS"
	}
	var idv uuid.UUID
	err := a.DB.QueryRow(ctx, `
		SELECT i.id
		FROM instruments i
		LEFT JOIN listings l ON l.instrument_id = i.id
		WHERE (i.quik_symbol=$1 OR i.internal_symbol=$1 OR i.ticker=$1 OR i.isin=$1)
		  AND ($2 = '' OR i.board=$2 OR l.board=$2)
		ORDER BY i.created_at
		LIMIT 1`, f.Symbol, f.Board).Scan(&idv)
	if err == nil {
		_, _ = a.DB.Exec(ctx, `
			UPDATE instruments
			SET quik_symbol=COALESCE(NULLIF(quik_symbol,''), $2),
			    board=COALESCE(NULLIF(board,''), NULLIF($3,'')),
			    status=$4
			WHERE id=$1`, idv, f.Symbol, f.Board, f.Status)
		return idv, nil
	}

	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	issuerID := id.New()
	short := f.Symbol
	if len(short) > 16 {
		short = short[:16]
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO issuers (id, name, short_name, country, status)
		VALUES ($1,$2,$3,'KG','ACTIVE')`, issuerID, f.Name, short); err != nil {
		return uuid.Nil, err
	}
	secID := id.New()
	secType := "EQUITY"
	if strings.Contains(strings.ToUpper(f.SecurityType), "BOND") {
		secType = "BOND"
	}
	isin := f.ISIN
	if isin == "" {
		isin = f.Symbol
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO securities (id, issuer_id, isin, ticker, name, type, currency, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'ACTIVE')
		ON CONFLICT (isin) DO NOTHING`, secID, issuerID, isin, f.Symbol, f.Name, secType, f.Currency); err != nil {
		return uuid.Nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT id FROM securities WHERE isin=$1`, isin).Scan(&secID); err != nil {
		return uuid.Nil, err
	}
	instrID := id.New()
	err = tx.QueryRow(ctx, `
		INSERT INTO instruments (id, security_id, ticker, isin, name, type, currency, lot_size, tick_size, status,
		                         internal_symbol, quik_symbol, board, security_type)
		VALUES ($1,$2,$3,$4,$5,$6,$7,1,0.01,$8,$3,$3,$9,$10)
		ON CONFLICT (security_id) DO UPDATE
		  SET quik_symbol=EXCLUDED.quik_symbol, board=EXCLUDED.board, status=EXCLUDED.status
		RETURNING id`,
		instrID, secID, f.Symbol, isin, f.Name, secType, f.Currency, f.Status, f.Board, f.SecurityType).Scan(&instrID)
	if err != nil {
		var existing uuid.UUID
		if err2 := tx.QueryRow(ctx, `SELECT id FROM instruments WHERE security_id=$1`, secID).Scan(&existing); err2 != nil {
			return uuid.Nil, err
		}
		instrID = existing
	}
	var exID uuid.UUID
	_ = tx.QueryRow(ctx, `SELECT id FROM exchanges WHERE code='KSE'`).Scan(&exID)
	if exID != uuid.Nil {
		_, _ = tx.Exec(ctx, `
			INSERT INTO listings (id, instrument_id, exchange_id, venue_symbol, status, supported_types, supported_tif, board)
			VALUES ($1,$2,$3,$4,'ACTIVE', ARRAY['MARKET','LIMIT'], ARRAY['DAY','GTC','IOC','FOK'], $5)
			ON CONFLICT (instrument_id, exchange_id) DO UPDATE
			  SET venue_symbol=EXCLUDED.venue_symbol, board=EXCLUDED.board, status='ACTIVE'`,
			id.New(), instrID, exID, f.Symbol, f.Board)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return instrID, nil
}

func (a *App) localMarketBoard(ctx context.Context) ([]marketItem, error) {
	if a.DB == nil {
		return []marketItem{}, nil
	}
	rows, err := a.DB.Query(ctx, `
		SELECT i.id, i.ticker, i.isin, i.name, COALESCE(i.board,''), i.currency, i.status,
		       COALESCE(i.security_type,'CS'), COALESCE(i.quik_symbol, i.ticker)
		FROM instruments i
		WHERE i.status IN ('ACTIVE','SUSPENDED','HALTED')
		ORDER BY i.ticker`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []marketItem{}
	for rows.Next() {
		var idv uuid.UUID
		var ticker, isin, name, board, cur, st, stype, venue string
		if err := rows.Scan(&idv, &ticker, &isin, &name, &board, &cur, &st, &stype, &venue); err != nil {
			return nil, err
		}
		item := marketItem{
			ID: idv.String(), Ticker: ticker, ISIN: isin, Name: name, Board: board,
			Currency: cur, Status: st, SecurityType: stype, Source: "LOCAL", VenueSymbol: venue,
		}
		a.attachSnapshot(ctx, idv, &item)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *App) attachSnapshot(ctx context.Context, instrID uuid.UUID, item *marketItem) {
	if a.DB == nil {
		return
	}
	var last, bid, ask, vol *decimal.Decimal
	err := a.DB.QueryRow(ctx, `
		SELECT last_price, bid, ask, volume
		FROM market_snapshots WHERE instrument_id=$1 ORDER BY ts DESC LIMIT 1`, instrID).
		Scan(&last, &bid, &ask, &vol)
	if err != nil {
		return
	}
	item.Last = decStr(last)
	item.Bid = decStr(bid)
	item.Ask = decStr(ask)
	item.Volume = decStr(vol)
}

func (a *App) orderBook(ctx context.Context, instrID uuid.UUID) ([]map[string]any, []map[string]any, error) {
	bids := []map[string]any{}
	asks := []map[string]any{}
	if a.DB == nil {
		return bids, asks, nil
	}
	rows, err := a.DB.Query(ctx, `
		SELECT side, price, quantity FROM order_book_levels
		WHERE instrument_id=$1
		ORDER BY CASE WHEN side='BID' THEN price END DESC,
		         CASE WHEN side='ASK' THEN price END ASC
		LIMIT 40`, instrID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var side string
			var px, qty decimal.Decimal
			if rows.Scan(&side, &px, &qty) != nil {
				continue
			}
			lvl := map[string]any{"price": px.String(), "quantity": qty.String()}
			if side == "ASK" {
				asks = append(asks, lvl)
			} else {
				bids = append(bids, lvl)
			}
		}
	}
	if len(bids) == 0 && len(asks) == 0 {
		var bid, ask, last *decimal.Decimal
		_ = a.DB.QueryRow(ctx, `
			SELECT bid, ask, last_price FROM market_snapshots
			WHERE instrument_id=$1 ORDER BY ts DESC LIMIT 1`, instrID).Scan(&bid, &ask, &last)
		if bid != nil && bid.IsPositive() {
			bids = append(bids, map[string]any{"price": bid.String(), "quantity": "—"})
		}
		if ask != nil && ask.IsPositive() {
			asks = append(asks, map[string]any{"price": ask.String(), "quantity": "—"})
		}
	}
	return bids, asks, nil
}

func mapString(row map[string]any, key string) string {
	if row == nil {
		return ""
	}
	v, ok := row[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func decStr(d *decimal.Decimal) string {
	if d == nil {
		return ""
	}
	return d.String()
}

func orderCanCancel(status string) bool {
	switch strings.ToUpper(status) {
	case "ACKNOWLEDGED", "PARTIALLY_FILLED", "SENT", "PENDING":
		return true
	default:
		return false
	}
}
