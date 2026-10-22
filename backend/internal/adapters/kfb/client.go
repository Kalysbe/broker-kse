package kfb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"statebroker/sbis/internal/adapters"
	"statebroker/sbis/internal/gateway"
	"statebroker/sbis/internal/platform/httpx"
)

type Client struct {
	connection        atomic.Pointer[ConnectionSettings]
	connectionVersion atomic.Uint64
	gate              *requestGate
	readGate          *requestGate
	healthMu          sync.Mutex
	base              string
	token             string
	http              *http.Client
	mu                sync.Mutex
	health            adapters.Health
	last              Snapshot
}

type Snapshot struct {
	Connected        bool             `json:"connected"`
	BaseURL          string           `json:"base_url"`
	TokenPrefix      string           `json:"token_prefix"`
	Ready            bool             `json:"ready"`
	Health           string           `json:"health"`
	Firm             map[string]any   `json:"firm"`
	Instruments      []map[string]any `json:"instruments"`
	Accounts         []map[string]any `json:"accounts"`
	Positions        []map[string]any `json:"positions"`
	PositionsTakenAt any              `json:"positions_taken_at,omitempty"`
	PositionsNote    string           `json:"positions_note,omitempty"`
	Error            string           `json:"error"`
	LastCheckAt      *time.Time       `json:"last_check_at"`
}

type PositionsResult struct {
	Positions []map[string]any
	TakenAt   any
	Note      string
}

type RemoteOrder struct {
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
	QuikOrderID      string    `json:"quik_order_id"`
	FIXClientOrderID string    `json:"fix_cl_ord_id"`
	ID               string    `json:"id"`
	Status           string    `json:"status"`
	FilledQuantity   string    `json:"filled_quantity"`
	AveragePrice     string    `json:"avg_price"`
	RejectReason     string    `json:"reject_reason"`
}

type OrderBookResult struct {
	Bids      []map[string]any
	Asks      []map[string]any
	Symbol    string
	Board     string
	Currency  string
	Last      string
	UpdatedAt any
	Live      bool
	Source    string
}

func New(baseURL, token string) *Client {
	tok := strings.TrimSpace(token)
	return &Client{
		base:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token: tok,
		http: &http.Client{
			Timeout:   12 * time.Second,
			Transport: http.DefaultTransport,
		},
		health:   adapters.Offline,
		gate:     &requestGate{interval: 51 * time.Millisecond},
		readGate: &requestGate{interval: 11 * time.Millisecond},
	}
}

type bearerTransport struct {
	base  http.RoundTripper
	token string
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.TrimSpace(t.token) == "" {
		return nil, fmt.Errorf("kfb token is not configured")
	}
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	if clone.Header.Get("Accept") == "" {
		clone.Header.Set("Accept", "application/json")
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL() != "" && c.gatewayToken() != ""
}

func (c *Client) Code() string { return "KFB" }
func (c *Client) Kind() string { return "EXCHANGE" }
func (c *Client) Weight() int  { return 20 }

func (c *Client) HealthCheck(ctx context.Context) adapters.Health {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	snapshot := c.Snapshot()
	if snapshot.LastCheckAt != nil && time.Since(*snapshot.LastCheckAt) < 15*time.Second {
		var detail RequestError
		if json.Unmarshal([]byte(snapshot.Error), &detail) == nil && (detail.HTTPStatus == 429 || detail.Stage == "rate_limit_wait") {
			return adapters.Degraded
		}
		return adapters.Health(snapshot.Health)
	}
	if !c.Configured() {
		c.setHealth(adapters.Offline, "KFB gateway URL or token is not configured")
		return adapters.Offline
	}
	_, err := c.get(ctx, "/health")
	if err != nil {
		if isThrottled(err) {
			c.setHealth(adapters.Degraded, err.Error())
			return adapters.Degraded
		}
		c.setHealth(adapters.Offline, err.Error())
		return adapters.Offline
	}
	_, err = c.get(ctx, "/api/v1/firm")
	if err != nil {
		if isThrottled(err) {
			c.setHealth(adapters.Degraded, err.Error())
			return adapters.Degraded
		}
		c.setHealth(adapters.Offline, err.Error())
		return adapters.Offline
	}
	c.setHealth(adapters.Online, "")
	return adapters.Online
}

func (c *Client) Refresh(ctx context.Context) Snapshot {
	if !c.Configured() {
		snap := Snapshot{
			Connected:   false,
			BaseURL:     c.baseURL(),
			TokenPrefix: "",
			Health:      string(adapters.Offline),
			Error:       "KFB gateway URL or token is not configured",
			Instruments: []map[string]any{},
			Accounts:    []map[string]any{},
			Positions:   []map[string]any{},
			Firm:        map[string]any{},
		}
		c.mu.Lock()
		c.last = snap
		c.health = adapters.Offline
		c.mu.Unlock()
		return snap
	}
	now := time.Now().UTC()
	snap := Snapshot{
		BaseURL:     c.baseURL(),
		TokenPrefix: gateway.TokenPrefix(c.gatewayToken()),
		Instruments: []map[string]any{},
		Accounts:    []map[string]any{},
		Positions:   []map[string]any{},
		Firm:        map[string]any{},
		LastCheckAt: &now,
	}
	if _, err := c.get(ctx, "/health"); err != nil {
		snap.Error = err.Error()
		snap.Health = string(adapters.Offline)
		c.store(snap, adapters.Offline)
		return snap
	}
	readyBody, err := c.get(ctx, "/ready")
	if err == nil {
		snap.Ready = true
		var ready map[string]any
		_ = json.Unmarshal(readyBody, &ready)
		if v, ok := ready["ready"].(bool); ok {
			snap.Ready = v
		} else if s, ok := ready["status"].(string); ok && s != "" && s != "ready" {
			snap.Ready = false
		}
	}
	firmBody, err := c.get(ctx, "/api/v1/firm")
	if err != nil {
		snap.Error = err.Error()
		snap.Health = string(adapters.Offline)
		c.store(snap, adapters.Offline)
		return snap
	}
	_ = json.Unmarshal(firmBody, &snap.Firm)
	snap.Connected = true
	snap.Health = string(adapters.Online)
	if instBody, err := c.get(ctx, "/api/v1/instruments"); err == nil {
		var wrap struct {
			Instruments []map[string]any `json:"instruments"`
		}
		if json.Unmarshal(instBody, &wrap) == nil && wrap.Instruments != nil {
			snap.Instruments = wrap.Instruments
		}
	}
	if accBody, err := c.get(ctx, "/api/v1/accounts"); err == nil {
		var wrap struct {
			Accounts []map[string]any `json:"accounts"`
		}
		if json.Unmarshal(accBody, &wrap) == nil && wrap.Accounts != nil {
			snap.Accounts = wrap.Accounts
		}
	}
	if posBody, err := c.get(ctx, "/api/v1/positions"); err == nil {
		pos := parsePositions(posBody)
		snap.Positions = pos.Positions
		snap.PositionsTakenAt = pos.TakenAt
		snap.PositionsNote = pos.Note
	}
	c.store(snap, adapters.Online)
	return snap
}

func (c *Client) FetchPositions(ctx context.Context) (PositionsResult, error) {
	if !c.Configured() {
		return PositionsResult{Positions: []map[string]any{}}, fmt.Errorf("kfb token is not configured")
	}
	raw, err := c.get(ctx, "/api/v1/positions")
	if err != nil {
		return PositionsResult{Positions: []map[string]any{}}, err
	}
	pos := parsePositions(raw)
	c.mu.Lock()
	c.last.Positions = pos.Positions
	c.last.PositionsTakenAt = pos.TakenAt
	c.last.PositionsNote = pos.Note
	c.mu.Unlock()
	return pos, nil
}

func (c *Client) FetchOrder(ctx context.Context, venueOrderID string) (RemoteOrder, error) {
	if !c.Configured() || strings.TrimSpace(venueOrderID) == "" {
		return RemoteOrder{}, fmt.Errorf("kfb order lookup is not configured")
	}
	if err := validateGatewayOrderID(venueOrderID); err != nil {
		return RemoteOrder{}, err
	}
	raw, err := c.get(ctx, "/api/v1/orders/"+url.PathEscape(venueOrderID))
	if err != nil {
		return RemoteOrder{}, err
	}
	var wrap struct {
		Order RemoteOrder `json:"order"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return RemoteOrder{}, err
	}
	if wrap.Order.ID == "" {
		return RemoteOrder{}, fmt.Errorf("kfb order response is empty")
	}
	return wrap.Order, nil
}

func (c *Client) FetchOrderDetails(ctx context.Context, venueOrderID string) (json.RawMessage, error) {
	if !c.Configured() || strings.TrimSpace(venueOrderID) == "" {
		return nil, fmt.Errorf("kfb order lookup is not configured")
	}
	if err := validateGatewayOrderID(venueOrderID); err != nil {
		return nil, err
	}
	raw, err := c.get(ctx, "/api/v1/orders/"+url.PathEscape(venueOrderID))
	if err != nil {
		return nil, err
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("invalid gateway order JSON")
	}
	return json.RawMessage(raw), nil
}

func (c *Client) FetchOrderBook(ctx context.Context, symbol, board string) (OrderBookResult, error) {
	if !c.Configured() {
		return OrderBookResult{}, fmt.Errorf("kfb order book is not configured")
	}
	path := "/api/v1/orderbook?symbol=" + url.QueryEscape(strings.TrimSpace(symbol)) + "&board=" + url.QueryEscape(strings.TrimSpace(board))
	raw, err := c.get(ctx, path)
	if err != nil {
		return OrderBookResult{}, err
	}
	var response struct {
		Books []struct {
			Symbol    string `json:"symbol"`
			Board     string `json:"board"`
			Currency  string `json:"currency"`
			Last      string `json:"last"`
			UpdatedAt any    `json:"updated_at"`
			Bids      []struct {
				Price string `json:"price"`
				Size  string `json:"size"`
			} `json:"bids"`
			Offers []struct {
				Price string `json:"price"`
				Size  string `json:"size"`
			} `json:"offers"`
		} `json:"books"`
		Live   bool   `json:"live"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return OrderBookResult{}, err
	}
	if len(response.Books) == 0 {
		return OrderBookResult{Bids: []map[string]any{}, Asks: []map[string]any{}, Live: response.Live, Source: response.Source}, nil
	}
	book := response.Books[0]
	result := OrderBookResult{Bids: []map[string]any{}, Asks: []map[string]any{}, Symbol: book.Symbol, Board: book.Board, Currency: book.Currency, Last: book.Last, UpdatedAt: book.UpdatedAt, Live: response.Live, Source: response.Source}
	for _, level := range book.Bids {
		result.Bids = append(result.Bids, map[string]any{"price": level.Price, "quantity": level.Size})
	}
	for _, level := range book.Offers {
		result.Asks = append(result.Asks, map[string]any{"price": level.Price, "quantity": level.Size})
	}
	return result, nil
}

func (c *Client) FetchTrades(ctx context.Context) ([]map[string]any, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("kfb trades are not configured")
	}
	raw, err := c.get(ctx, "/api/v1/trades")
	if err != nil {
		return nil, err
	}
	var response struct {
		Trades []map[string]any `json:"trades"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	if response.Trades == nil {
		response.Trades = []map[string]any{}
	}
	return response.Trades, nil
}

func (c *Client) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.last
	if s.Instruments == nil {
		s.Instruments = []map[string]any{}
	}
	if s.Accounts == nil {
		s.Accounts = []map[string]any{}
	}
	if s.Positions == nil {
		s.Positions = []map[string]any{}
	}
	if s.Firm == nil {
		s.Firm = map[string]any{}
	}
	if s.TokenPrefix == "" && c.gatewayToken() != "" {
		s.TokenPrefix = gateway.TokenPrefix(c.gatewayToken())
	}
	if s.BaseURL == "" {
		s.BaseURL = c.baseURL()
	}
	return s
}

func (c *Client) SendOrder(ctx context.Context, o adapters.VenueOrder) (adapters.VenueAck, error) {
	tif := o.TIF
	if tif == "" {
		tif = "DAY"
	}
	body := map[string]any{
		"account_id":      o.Account,
		"client_code":     o.ClientCode,
		"client_order_id": o.ClientOrderID,
		"symbol":          o.VenueSymbol,
		"board":           o.Board,
		"side":            o.Side,
		"order_type":      o.Type,
		"time_in_force":   tif,
		"quantity":        o.Quantity.String(),
	}
	if o.Type == "LIMIT" && !o.Price.IsZero() {
		body["price"] = o.Price.String()
	}
	raw, status, err := c.do(ctx, http.MethodPost, "/api/v1/orders", body)
	if err != nil {
		return adapters.VenueAck{}, err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return adapters.VenueAck{}, fmt.Errorf("kfb create order: HTTP %d: %s", status, string(raw))
	}
	var resp struct {
		Order struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"order"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return adapters.VenueAck{}, &RequestError{Code: "ADAPTER_ERROR", Source: "GATEWAY", Stage: "decode_response", Message: err.Error(), Method: http.MethodPost, Path: "/api/v1/orders", Request: body, HTTPStatus: status, Response: string(raw), At: time.Now().UTC()}
	}
	if resp.Order.ID == "" {
		return adapters.VenueAck{}, &RequestError{Code: "ADAPTER_ERROR", Source: "GATEWAY", Stage: "decode_response", Message: "gateway returned an empty order id", Method: http.MethodPost, Path: "/api/v1/orders", Request: body, HTTPStatus: status, Response: string(raw), At: time.Now().UTC()}
	}
	return adapters.VenueAck{VenueOrderID: resp.Order.ID, Status: resp.Order.Status}, nil
}

func validateGatewayOrderID(orderID string) error {
	if strings.HasPrefix(orderID, "VENUE-") {
		return fmt.Errorf("mock order id cannot be sent to KFB gateway")
	}
	return nil
}

func (c *Client) CancelOrder(ctx context.Context, venueOrderID, clientOrderID string, orderQty decimal.Decimal) error {
	if venueOrderID == "" {
		return fmt.Errorf("missing KFB order id")
	}
	if err := validateGatewayOrderID(venueOrderID); err != nil {
		return err
	}
	qty := orderQty.String()
	body := map[string]any{
		"client_order_id": clientOrderID,
		"quantity":        qty,
		"OrderQty":        qty,
	}
	raw, status, err := c.do(ctx, http.MethodPost, "/api/v1/orders/"+venueOrderID+"/cancel", body)
	if err != nil {
		return err
	}
	if status >= 300 {
		return fmt.Errorf("kfb cancel: %s", gatewayError(raw, status))
	}
	return nil
}

func (c *Client) ReplaceOrder(ctx context.Context, venueOrderID string, o adapters.VenueOrder) error {
	return fmt.Errorf("replace is not available in KFB stage 1")
}

func (c *Client) GetMarketData(ctx context.Context, symbol string) (map[string]decimal.Decimal, error) {
	return map[string]decimal.Decimal{}, nil
}

func (c *Client) GetTradingStatus(ctx context.Context) (string, error) {
	if c.HealthCheck(ctx) == adapters.Online {
		return "OPEN", nil
	}
	return "UNKNOWN", httpx.ErrAdapterUnavailable
}

func (c *Client) GetInstrument(ctx context.Context, symbol string) (string, error) {
	return symbol, nil
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	raw, status, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("%s", gatewayError(raw, status))
	}
	return raw, nil
}

func (c *Client) do(ctx context.Context, method, path string, payload any) ([]byte, int, error) {
	started := time.Now().UTC()
	diagnostic := &RequestError{Code: "ADAPTER_ERROR", Source: "OUR_SYSTEM", Stage: "prepare", Method: method, Path: path, Request: payload, At: started, RequestID: uuid.NewString()}
	failure := func(stage, source string, status int, raw []byte, err error) ([]byte, int, error) {
		diagnostic.Stage, diagnostic.Source, diagnostic.HTTPStatus = stage, source, status
		diagnostic.Response, diagnostic.Message = string(raw), err.Error()
		diagnostic.ElapsedMS = time.Since(started).Milliseconds()
		return raw, status, diagnostic
	}
	if c == nil || strings.TrimSpace(c.gatewayToken()) == "" {
		return failure("configuration", "OUR_SYSTEM", 0, nil, fmt.Errorf("kfb token is not configured"))
	}
	settings := c.ConnectionSettings()
	var rdr io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return failure("serialize", "OUR_SYSTEM", 0, nil, err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, settings.RESTURL+path, rdr)
	if err != nil {
		return failure("prepare", "OUR_SYSTEM", 0, nil, err)
	}
	req.Header.Set("Authorization", "Bearer "+settings.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-ID", diagnostic.RequestID)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	gate := c.gateFor(method)
	if err := gate.wait(ctx); err != nil {
		return failure("rate_limit_wait", "OUR_SYSTEM", 0, nil, err)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return failure("transport", "UNKNOWN", 0, nil, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return failure("read_response", "UNKNOWN", res.StatusCode, raw, err)
	}
	if res.StatusCode >= 300 {
		if res.StatusCode == http.StatusTooManyRequests {
			diagnostic.Code = "RATE_LIMITED"
			if pauseErr := gate.pause(ctx, retryDelay(res.Header.Get("Retry-After"))); pauseErr != nil {
				return failure("rate_limit_pause", "OUR_SYSTEM", res.StatusCode, raw, pauseErr)
			}
		}
		return failure("gateway_response", "GATEWAY", res.StatusCode, raw, fmt.Errorf("HTTP %d: %s", res.StatusCode, string(raw)))
	}
	return raw, res.StatusCode, nil
}

func (c *Client) setHealth(h adapters.Health, errText string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.health = h
	c.last.Health = string(h)
	c.last.Connected = h == adapters.Online
	c.last.Error = errText
	c.last.BaseURL = c.baseURL()
	if c.gatewayToken() != "" {
		c.last.TokenPrefix = gateway.TokenPrefix(c.gatewayToken())
	}
	now := time.Now().UTC()
	c.last.LastCheckAt = &now
}

func (c *Client) store(snap Snapshot, h adapters.Health) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.health = h
	c.last = snap
}

// ParsePositions normalizes a full REST or gRPC positions snapshot.
func ParsePositions(raw []byte) PositionsResult { return parsePositions(raw) }

func parsePositions(raw []byte) PositionsResult {
	out := PositionsResult{Positions: []map[string]any{}}
	var wrap struct {
		Positions   json.RawMessage  `json:"positions"`
		Cash        []map[string]any `json:"cash"`
		Instruments []map[string]any `json:"instruments"`
		Snapshot    map[string]any   `json:"snapshot"`
		Source      string           `json:"source"`
		TakenAt     any              `json:"taken_at"`
		Note        any              `json:"note"`
	}
	if json.Unmarshal(raw, &wrap) == nil {
		if wrap.TakenAt != nil {
			out.TakenAt = wrap.TakenAt
		}
		if wrap.Snapshot != nil {
			if v := wrap.Snapshot["completed_at"]; v != nil {
				out.TakenAt = v
			} else if v := wrap.Snapshot["requested_at"]; v != nil && out.TakenAt == nil {
				out.TakenAt = v
			}
			if st, ok := wrap.Snapshot["status"].(string); ok && st != "" {
				out.Note = st
			}
		}
		if s, ok := wrap.Note.(string); ok && s != "" {
			out.Note = s
		} else if out.Note == "" && wrap.Source != "" {
			out.Note = wrap.Source
		}
		if wrap.Cash != nil || wrap.Instruments != nil {
			out.Positions = flattenPositionRows(append(append([]map[string]any{}, wrap.Cash...), wrap.Instruments...))
			return out
		}
		if len(wrap.Positions) > 0 {
			var items []map[string]any
			if json.Unmarshal(wrap.Positions, &items) == nil && items != nil {
				out.Positions = flattenPositionRows(items)
				return out
			}
			var nested struct {
				Cash        []map[string]any `json:"cash"`
				Instruments []map[string]any `json:"instruments"`
			}
			if json.Unmarshal(wrap.Positions, &nested) == nil && (nested.Cash != nil || nested.Instruments != nil) {
				out.Positions = flattenPositionRows(append(append([]map[string]any{}, nested.Cash...), nested.Instruments...))
				return out
			}
		}
	}
	var items []map[string]any
	if json.Unmarshal(raw, &items) == nil && items != nil {
		out.Positions = flattenPositionRows(items)
	}
	return out
}

func flattenPositionRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		out = append(out, flattenPositionRow(row))
	}
	return out
}

func flattenPositionRow(m map[string]any) map[string]any {
	items := map[string]any{}
	if nested, ok := m["items"].(map[string]any); ok {
		items = nested
	}
	return map[string]any{
		"kind":          firstString(m["kind"]),
		"account_id":    firstString(m["account_id"], m["account_code"]),
		"client_code":   firstString(m["client_code"]),
		"symbol":        firstString(m["symbol"], m["security_id"]),
		"board":         firstString(m["board"]),
		"currency":      firstString(m["currency"], items["CUR"]),
		"quantity":      firstString(m["quantity"], m["net_qty"], items["CUR_BAL"]),
		"avg_price":     firstString(m["avg_price"], items["AVG_POS_PRICE"]),
		"available_qty": firstString(m["available_qty"], items["CUR_BAL"]),
		"blocked_qty":   firstString(m["blocked_qty"], items["LOCKED"]),
	}
}

func firstString(vals ...any) string {
	for _, v := range vals {
		if v == nil {
			continue
		}
		var s string
		switch t := v.(type) {
		case string:
			s = t
		case json.Number:
			s = t.String()
		default:
			s = strings.TrimSpace(fmt.Sprint(t))
		}
		s = strings.TrimSpace(s)
		if s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

func gatewayError(raw []byte, status int) string {
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &body) == nil && body.Error != "" {
		return fmt.Sprintf("HTTP %d: %s", status, body.Error)
	}
	msg := strings.TrimSpace(string(raw))
	if msg == "" {
		return fmt.Sprintf("HTTP %d", status)
	}
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return fmt.Sprintf("HTTP %d: %s", status, msg)
}
