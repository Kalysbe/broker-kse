package adapters

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Health string

const (
	Online   Health = "ONLINE"
	Degraded Health = "DEGRADED"
	Offline  Health = "OFFLINE"
)

type VenueOrder struct {
	ClientOrderID string
	VenueSymbol   string
	Account       string
	ClientCode    string
	QuikFirm      string
	Board         string
	Side          string
	Type          string
	TIF           string
	Quantity      decimal.Decimal
	Price         decimal.Decimal
	StopPrice     decimal.Decimal
}

type VenueAck struct {
	VenueOrderID string
	Status       string
}

type ExecutionReport struct {
	ExecID        string
	ClientOrderID string
	VenueOrderID  string
	Status        string
	LastQty       decimal.Decimal
	LastPx        decimal.Decimal
	LeavesQty     decimal.Decimal
	Reason        string
}

type ExchangeAdapter interface {
	Code() string
	Kind() string
	Weight() int
	HealthCheck(ctx context.Context) Health
	SendOrder(ctx context.Context, o VenueOrder) (VenueAck, error)
	CancelOrder(ctx context.Context, venueOrderID, clientOrderID string, orderQty decimal.Decimal) error
	ReplaceOrder(ctx context.Context, venueOrderID string, o VenueOrder) error
	GetMarketData(ctx context.Context, symbol string) (map[string]decimal.Decimal, error)
	GetTradingStatus(ctx context.Context) (string, error)
	GetInstrument(ctx context.Context, symbol string) (string, error)
}

type BankAdapter interface {
	Code() string
	HealthCheck(ctx context.Context) Health
	AccountBalance(ctx context.Context, extAccount string) (decimal.Decimal, error)
	Deposit(ctx context.Context, extAccount string, amount decimal.Decimal, idem string) (string, error)
	Withdrawal(ctx context.Context, extAccount string, amount decimal.Decimal, idem string) (string, error)
}

type DepositoryAdapter interface {
	Code() string
	HealthCheck(ctx context.Context) Health
	Positions(ctx context.Context, account string) ([]map[string]any, error)
	Transfer(ctx context.Context, from, to, isin string, qty decimal.Decimal) error
}

type Manager struct {
	mu       sync.RWMutex
	adapters []ExchangeAdapter
	conns    map[string]int
}

func NewManager(list []ExchangeAdapter) *Manager {
	return &Manager{adapters: list, conns: map[string]int{}}
}

func (m *Manager) Healthy() []ExchangeAdapter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ExchangeAdapter, 0, len(m.adapters))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, a := range m.adapters {
		if a.HealthCheck(ctx) != Offline {
			out = append(out, a)
		}
	}
	return out
}

func (m *Manager) Pick(policy string) (ExchangeAdapter, bool) {
	return m.pick(m.Healthy(), policy)
}

func (m *Manager) PickKind(kind, policy string) (ExchangeAdapter, bool) {
	healthy := m.Healthy()
	filtered := make([]ExchangeAdapter, 0, len(healthy))
	for _, a := range healthy {
		if a.Kind() == kind {
			filtered = append(filtered, a)
		}
	}
	return m.pick(filtered, policy)
}

func (m *Manager) HasHealthyKind(kind string) bool {
	for _, a := range m.Healthy() {
		if a.Kind() == kind {
			return true
		}
	}
	return false
}

func (m *Manager) pick(healthy []ExchangeAdapter, policy string) (ExchangeAdapter, bool) {
	if len(healthy) == 0 {
		return nil, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch policy {
	case "LEAST_CONNECTIONS":
		best := healthy[0]
		bestN := m.conns[best.Code()]
		for _, a := range healthy[1:] {
			if m.conns[a.Code()] < bestN {
				best = a
				bestN = m.conns[a.Code()]
			}
		}
		m.conns[best.Code()]++
		return best, true
	default: // WEIGHTED
		total := 0
		for _, a := range healthy {
			total += a.Weight()
		}
		if total <= 0 {
			m.conns[healthy[0].Code()]++
			return healthy[0], true
		}
		pick := int(time.Now().UnixNano() % int64(total))
		acc := 0
		for _, a := range healthy {
			acc += a.Weight()
			if pick < acc {
				m.conns[a.Code()]++
				return a, true
			}
		}
		m.conns[healthy[0].Code()]++
		return healthy[0], true
	}
}

func (m *Manager) Release(code string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conns[code] > 0 {
		m.conns[code]--
	}
}

func AdapterID(code string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(code))
}
