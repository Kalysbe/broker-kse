package app

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"net/http"
	"statebroker/sbis/internal/platform/httpx"
	"strings"
)

// Account-scoped exchange balances never fall back to the broker's local ledger.
func (a *App) handleExchangePortfolio(w http.ResponseWriter, r *http.Request) {
	accountID, err := chiID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	principal, _ := httpx.PrincipalFrom(r.Context())
	if err := a.requireAccountAccess(r.Context(), principal, accountID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if a.KFB == nil || !a.KFB.Configured() {
		httpx.WriteError(w, r, httpx.E(503, "KFB_UNAVAILABLE", "Exchange connection is not configured"))
		return
	}
	var code, venue, currency string
	err = a.DB.QueryRow(r.Context(), `SELECT COALESCE(NULLIF(a.client_code,''),c.client_code,''),COALESCE(NULLIF(a.trading_account,''),a.account_number),a.currency FROM trading_accounts a JOIN clients c ON c.id=a.client_id WHERE a.id=$1`, accountID).Scan(&code, &venue, &currency)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(code) == "" {
		httpx.WriteError(w, r, httpx.E(422, "CLIENT_CODE_MISSING", "Exchange client code is not configured"))
		return
	}
	snapshot, err := a.kfbPositions(r.Context())
	if err != nil {
		httpx.WriteError(w, r, httpx.E(503, "KFB_UNAVAILABLE", "Could not refresh exchange balances"))
		return
	}
	buckets := map[string]string{}
	positions := []map[string]any{}
	securities := decimal.Zero
	valuationComplete := true
	for _, row := range snapshot.Positions {
		if !exchangeRowMatches(row, code, venue, currency) {
			continue
		}
		if strings.EqualFold(mapString(row, "kind"), "CASH") {
			// Missing balances stay absent instead of being presented as zero.
			for bucket, field := range map[string]string{"AVAILABLE": "available_qty", "BLOCKED": "blocked_qty"} {
				raw := mapString(row, field)
				if bucket == "AVAILABLE" && raw == "" {
					raw = mapString(row, "quantity")
				}
				if raw == "" {
					continue
				}
				value, parseErr := decimal.NewFromString(raw)
				if parseErr != nil {
					httpx.WriteError(w, r, httpx.E(502, "INVALID_EXCHANGE_DATA", "Invalid exchange balance"))
					return
				}
				previous, _ := decimal.NewFromString(buckets[bucket])
				buckets[bucket] = previous.Add(value).String()
			}
			continue
		}
		symbol := mapString(row, "symbol")
		qty, parseErr := decimal.NewFromString(mapString(row, "quantity"))
		if parseErr != nil {
			httpx.WriteError(w, r, httpx.E(502, "INVALID_EXCHANGE_DATA", "Invalid exchange position"))
			return
		}
		if qty.IsZero() {
			continue
		}
		var iid uuid.UUID
		var ticker string
		err = a.DB.QueryRow(r.Context(), `SELECT id,ticker FROM instruments WHERE (quik_symbol=$1 OR ticker=$1 OR isin=$1) AND ($2='' OR board=$2) ORDER BY created_at LIMIT 1`, symbol, mapString(row, "board")).Scan(&iid, &ticker)
		if err != nil {
			httpx.WriteError(w, r, httpx.E(502, "INSTRUMENT_NOT_SYNCED", "Exchange instrument is not in the broker catalogue"))
			return
		}
		avg, avgErr := decimal.NewFromString(mapString(row, "avg_price"))
		var price, value any
		if avgErr == nil {
			price = avg.String()
			value = qty.Mul(avg).String()
			securities = securities.Add(qty.Mul(avg))
		} else {
			valuationComplete = false
		}
		positions = append(positions, map[string]any{"instrument_id": iid, "ticker": ticker, "quantity": qty.String(), "average_price": price, "market_value": value})
	}
	var available, securityValue any
	if raw, ok := buckets["AVAILABLE"]; ok {
		available = raw
	}
	if valuationComplete {
		securityValue = securities.String()
	}
	httpx.WriteJSON(w, 200, map[string]any{"account_id": accountID, "cash": buckets, "available_funds": available, "securities_value": securityValue, "positions": positions, "source": "KFB", "taken_at": snapshot.TakenAt, "note": snapshot.Note, "valuation": "average_price", "cash_reported": available != nil})
}

func exchangeRowMatches(row map[string]any, clientCode, account, currency string) bool {
	if strings.TrimSpace(clientCode) == "" || !strings.EqualFold(positionClientCode(row), strings.TrimSpace(clientCode)) {
		return false
	}
	if value := mapString(row, "currency"); value != "" && !strings.EqualFold(value, currency) {
		return false
	}
	// A shared venue account never grants access to a different client code.
	if value := mapString(row, "account_id"); value != "" && account != "" && !strings.EqualFold(value, account) {
		return false
	}
	return true
}
