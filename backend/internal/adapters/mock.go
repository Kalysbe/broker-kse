package adapters

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"statebroker/sbis/internal/platform/httpx"
)

type MockExchange struct {
	code     string
	weight   int
	health   Health
	mu       sync.Mutex
	fillMode string
	kind     string
}

func NewMockExchange(code string, weight int, fillImmediate bool) *MockExchange {
	mode := "ack_only"
	if fillImmediate {
		mode = "immediate"
	}
	return &MockExchange{code: code, weight: weight, health: Online, fillMode: mode, kind: "MOCK"}
}

func NewMockFIX(code string, weight int) *MockExchange {
	return &MockExchange{code: code, weight: weight, health: Online, fillMode: "ack_only", kind: "FIX"}
}

func (m *MockExchange) Code() string { return m.code }
func (m *MockExchange) Kind() string {
	if m.kind != "" {
		return m.kind
	}
	return "MOCK"
}
func (m *MockExchange) Weight() int { return m.weight }
func (m *MockExchange) SetHealth(h Health) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.health = h
}
func (m *MockExchange) HealthCheck(ctx context.Context) Health {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.health
}

func (m *MockExchange) SendOrder(ctx context.Context, o VenueOrder) (VenueAck, error) {
	if m.HealthCheck(ctx) == Offline {
		return VenueAck{}, httpx.ErrAdapterUnavailable
	}
	return VenueAck{VenueOrderID: "VENUE-" + o.ClientOrderID, Status: "ACKNOWLEDGED"}, nil
}

func (m *MockExchange) CancelOrder(ctx context.Context, venueOrderID, clientOrderID string, orderQty decimal.Decimal) error {
	if m.HealthCheck(ctx) == Offline {
		return httpx.ErrAdapterUnavailable
	}
	_ = orderQty
	return nil
}

func (m *MockExchange) ReplaceOrder(ctx context.Context, venueOrderID string, o VenueOrder) error {
	return m.CancelOrder(ctx, venueOrderID, o.ClientOrderID, o.Quantity)
}

func (m *MockExchange) GetMarketData(ctx context.Context, symbol string) (map[string]decimal.Decimal, error) {
	return map[string]decimal.Decimal{"last": decimal.NewFromInt(100)}, nil
}

func (m *MockExchange) GetTradingStatus(ctx context.Context) (string, error) { return "OPEN", nil }
func (m *MockExchange) GetInstrument(ctx context.Context, symbol string) (string, error) {
	return symbol, nil
}

func ImmediateFill(o VenueOrder) ExecutionReport {
	px := o.Price
	if px.IsZero() {
		px = decimal.NewFromInt(100)
	}
	return ExecutionReport{
		ExecID:        "EXEC-" + uuid.NewString(),
		ClientOrderID: o.ClientOrderID,
		VenueOrderID:  "VENUE-" + o.ClientOrderID,
		Status:        "FILLED",
		LastQty:       o.Quantity,
		LastPx:        px,
		LeavesQty:     decimal.Zero,
	}
}

type OfflineFIX struct {
	code   string
	weight int
}

func NewOfflineFIX(code string, weight int) *OfflineFIX {
	return &OfflineFIX{code: code, weight: weight}
}

func (f *OfflineFIX) Code() string                           { return f.code }
func (f *OfflineFIX) Kind() string                           { return "FIX" }
func (f *OfflineFIX) Weight() int                            { return f.weight }
func (f *OfflineFIX) HealthCheck(ctx context.Context) Health { return Offline }
func (f *OfflineFIX) SendOrder(ctx context.Context, o VenueOrder) (VenueAck, error) {
	return VenueAck{}, httpx.ErrAdapterUnavailable
}
func (f *OfflineFIX) CancelOrder(ctx context.Context, venueOrderID, clientOrderID string, orderQty decimal.Decimal) error {
	_ = orderQty
	return httpx.ErrAdapterUnavailable
}
func (f *OfflineFIX) ReplaceOrder(ctx context.Context, venueOrderID string, o VenueOrder) error {
	return httpx.ErrAdapterUnavailable
}
func (f *OfflineFIX) GetMarketData(ctx context.Context, symbol string) (map[string]decimal.Decimal, error) {
	return nil, httpx.ErrAdapterUnavailable
}
func (f *OfflineFIX) GetTradingStatus(ctx context.Context) (string, error) {
	return "UNKNOWN", httpx.ErrAdapterUnavailable
}
func (f *OfflineFIX) GetInstrument(ctx context.Context, symbol string) (string, error) {
	return "", httpx.ErrAdapterUnavailable
}
