package app

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"statebroker/sbis/internal/platform/httpx"
)

func (a *App) handleExchangeOrders(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	code := ""
	if p.ClientID != nil {
		var err error
		code, err = a.clientCodeForUser(r.Context(), *p.ClientID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	remote, err := a.KFB.FetchOrders(r.Context())
	if err != nil {
		httpx.WriteError(w, r, httpx.E(503, "KFB_UNAVAILABLE", "Не удалось загрузить заявки с биржи"))
		return
	}
	items := []map[string]any{}
	for _, row := range remote.Orders {
		if p.ClientID != nil && (code == "" || !strings.EqualFold(positionClientCode(row), code)) {
			continue
		}
		item := exchangeOrderItem(row)
		if err := a.enrichExchangeOrder(r.Context(), p, row, item); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		items = append(items, item)
	}
	httpx.WriteJSON(w, 200, map[string]any{"items": items, "source": "KFB", "exchange_source": remote.Source})
}

// The exchange determines membership, values and status. Local records provide
// application IDs for account selection and existing cancellation routes only.
func exchangeOrderItem(row map[string]any) map[string]any {
	item := make(map[string]any, len(row)+6)
	for key, value := range row {
		item[key] = value
	}
	item["source"] = "KFB"
	item["venue_order_id"] = mapString(row, "id")
	item["client_order_id"] = firstNonEmpty(mapString(row, "client_order_id"), mapString(row, "quik_order_id"), mapString(row, "id"))
	item["instrument_id"] = mapString(row, "symbol")
	item["order_type"] = firstNonEmpty(mapString(row, "order_type"), mapString(row, "quik_type"), "UNKNOWN")
	item["status"] = strings.ToUpper(strings.TrimSpace(mapString(row, "status")))
	item["can_cancel"] = false
	if mapString(row, "filled_quantity") == "" {
		item["filled_quantity"] = "0"
	}
	if mapString(row, "remaining_quantity") == "" {
		if balance := mapString(row, "balance"); balance != "" {
			item["remaining_quantity"] = balance
		} else if qty, err := decimal.NewFromString(mapString(row, "quantity")); err == nil {
			if filled, err := decimal.NewFromString(mapString(item, "filled_quantity")); err == nil {
				item["remaining_quantity"] = decimal.Max(decimal.Zero, qty.Sub(filled)).String()
			}
		}
	}
	return item
}

func (a *App) enrichExchangeOrder(ctx context.Context, p httpx.Principal, remote, item map[string]any) error {
	if a.DB == nil {
		return nil
	}
	var scope any
	if p.ClientID != nil {
		scope = *p.ClientID
	}
	var id, clientOrderID, accountID, instrumentID, orderType, status, venueID string
	err := a.DB.QueryRow(ctx, `
		SELECT o.id::text,o.client_order_id,o.account_id::text,o.instrument_id::text,o.order_type,o.status,COALESCE(o.venue_order_id,'')
		FROM orders o JOIN trading_accounts a ON a.id=o.account_id JOIN clients c ON c.id=o.client_id
		WHERE o.broker_id=$1 AND ($2::uuid IS NULL OR o.client_id=$2)
		  AND (($3<>'' AND o.venue_order_id=$3) OR ($4<>'' AND o.exchange_order_id=$4))
		  AND ($5='' OR LOWER(TRIM(COALESCE(NULLIF(o.client_code,''),NULLIF(a.client_code,''),c.client_code,'')))=LOWER($5))
		  AND ($6='' OR COALESCE(NULLIF(a.trading_account,''),a.account_number)=$6)
		ORDER BY o.created_at DESC LIMIT 1`, p.BrokerID, scope, mapString(remote, "id"), mapString(remote, "quik_order_id"), positionClientCode(remote), mapString(remote, "account_id")).
		Scan(&id, &clientOrderID, &accountID, &instrumentID, &orderType, &status, &venueID)
	if err == nil {
		item["id"], item["client_order_id"] = id, clientOrderID
		item["account_id"], item["instrument_id"], item["order_type"] = accountID, instrumentID, orderType
		item["can_cancel"] = !a.Cfg.ExchangeReadOnly && venueID != "" && orderCanCancel(status) && orderCanCancel(mapString(item, "status"))
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	// Orders entered in QUIK remain visible even without a local OMS record.
	err = a.DB.QueryRow(ctx, `SELECT a.id::text FROM trading_accounts a JOIN clients c ON c.id=a.client_id
		WHERE a.broker_id=$1 AND ($2::uuid IS NULL OR a.client_id=$2)
		  AND COALESCE(NULLIF(a.trading_account,''),a.account_number)=$3
		  AND LOWER(TRIM(COALESCE(NULLIF(a.client_code,''),c.client_code,'')))=LOWER($4)
		ORDER BY a.created_at LIMIT 1`, p.BrokerID, scope, mapString(remote, "account_id"), positionClientCode(remote)).Scan(&accountID)
	if err == nil {
		item["account_id"] = accountID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	err = a.DB.QueryRow(ctx, `SELECT id::text FROM instruments WHERE (quik_symbol=$1 OR ticker=$1 OR isin=$1)
		AND ($2='' OR board=$2) ORDER BY created_at LIMIT 1`, mapString(remote, "symbol"), mapString(remote, "board")).Scan(&instrumentID)
	if err == nil {
		item["instrument_id"] = instrumentID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return nil
}
