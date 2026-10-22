package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"statebroker/sbis/internal/adapters/kfb"
	"statebroker/sbis/internal/platform/httpx"
)

const activeVenueOrders = `o.client_order_id LIKE 'LOAD-%' AND o.broker_id=$1 AND o.venue_order_id IS NOT NULL AND o.venue_checked_at IS NOT NULL AND COALESCE(o.venue_check_error,'')='' AND o.venue_status IN ('ACKNOWLEDGED','PARTIALLY_FILLED','CANCEL_PENDING')`

func (a *App) handleActiveLoadOrders(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r.Context())
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if size < 1 {
		size = 50
	}
	if size > 200 {
		size = 200
	}
	var total int
	if err := a.DB.QueryRow(r.Context(), `SELECT count(*) FROM orders o WHERE `+activeVenueOrders, p.BrokerID).Scan(&total); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT o.id::text,o.client_order_id,COALESCE(o.client_code,''),COALESCE(a.trading_account,a.account_number),o.side,o.price::text,o.quantity::text,o.remaining_quantity::text,o.status,o.venue_order_id,COALESCE(o.exchange_order_id,''),o.venue_status,o.venue_checked_at FROM orders o JOIN trading_accounts a ON a.id=o.account_id WHERE `+activeVenueOrders+` ORDER BY o.created_at DESC,o.id LIMIT $2 OFFSET $3`, p.BrokerID, size, (page-1)*size)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	items := make([]loadTestRequestResult, 0, size)
	for rows.Next() {
		var item loadTestRequestResult
		if err := rows.Scan(&item.ServerOrderID, &item.ClientOrderID, &item.ClientCode, &item.AccountID, &item.Side, &item.Price, &item.Quantity, &item.RemainingQuantity, &item.Status, &item.VenueOrderID, &item.ExchangeOrderID, &item.VenueStatus, &item.VenueCheckedAt); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		item.Sequence, _ = strconv.Atoi(item.ClientOrderID[strings.LastIndex(item.ClientOrderID, "-")+1:])
		item.Verified = true
		item.ExchangeReached = true
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"items": items, "total": total, "page": page, "page_size": size})
}

func (a *App) handleLoadOrderTracking(w http.ResponseWriter, r *http.Request) {
	a.KFBSyncMu.Lock()
	defer a.KFBSyncMu.Unlock()
	id, err := chiID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r.Context())
	var venueID string
	var local json.RawMessage
	var cached json.RawMessage
	var detailsLoaded *time.Time
	var checked *time.Time
	var checkError *string
	err = a.DB.QueryRow(r.Context(), `SELECT COALESCE(o.venue_order_id,''),jsonb_build_object('id',o.id,'client_order_id',o.client_order_id,'client_code',o.client_code,'account_id',COALESCE(a.trading_account,a.account_number),'symbol',o.symbol,'board',o.board,'side',o.side,'quantity',o.quantity::text,'price',o.price::text,'remaining_quantity',o.remaining_quantity::text,'status',o.status,'venue_order_id',o.venue_order_id,'exchange_order_id',o.exchange_order_id,'venue_status',o.venue_status,'venue_checked_at',o.venue_checked_at,'venue_check_error',o.venue_check_error,'reject_reason',o.reject_reason),o.venue_details,o.venue_details_loaded_at,o.venue_checked_at,o.venue_check_error FROM orders o JOIN trading_accounts a ON a.id=o.account_id WHERE o.id=$1 AND o.broker_id=$2 AND o.client_order_id LIKE 'LOAD-%'`, id, p.BrokerID).Scan(&venueID, &local, &cached, &detailsLoaded, &checked, &checkError)
	if err != nil {
		httpx.WriteError(w, r, httpx.E(404, "ORDER_NOT_FOUND", "Тестовая заявка не найдена"))
		return
	}
	result := map[string]any{"order": local, "checked_at": checked}
	if len(cached) > 0 {
		result["gateway"] = cached
	}
	if checkError != nil {
		result["gateway_error"] = *checkError
	}
	var updates json.RawMessage
	if err := a.DB.QueryRow(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'received_at',received_at,'payload',payload) ORDER BY id),'[]'::jsonb) FROM venue_order_updates WHERE order_id=$1`, id).Scan(&updates); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	result["updates"] = updates
	if (detailsLoaded == nil || r.URL.Query().Get("refresh") == "true") && venueID != "" && a.KFB != nil && a.KFB.Configured() {
		raw, fetchErr := a.KFB.FetchOrderDetails(r.Context(), venueID)
		result["checked_at"] = time.Now().UTC()
		if fetchErr != nil {
			result["gateway_error"] = fetchErr.Error()
			if _, err := a.DB.Exec(r.Context(), `UPDATE orders SET venue_checked_at=now(),venue_check_error=$2 WHERE id=$1`, id, fetchErr.Error()); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
		} else {
			delete(result, "gateway_error")
			result["gateway"] = raw
			var wrap struct {
				Order kfb.RemoteOrder `json:"order"`
			}
			if json.Unmarshal(raw, &wrap) == nil && wrap.Order.ID == venueID {
				if _, err := a.DB.Exec(r.Context(), `UPDATE orders SET venue_details_loaded_at=now(),venue_details=$4,venue_status=$2,venue_checked_at=now(),venue_check_error=NULL,exchange_order_id=COALESCE(NULLIF($3,''),exchange_order_id) WHERE id=$1`, id, strings.ToUpper(wrap.Order.Status), wrap.Order.QuikOrderID, raw); err != nil {
					httpx.WriteError(w, r, err)
					return
				}
			}
		}
	}
	httpx.WriteJSON(w, 200, result)
}
