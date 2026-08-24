package adapters

import "testing"

func TestNeverPicksOffline(t *testing.T) {
	online := NewMockExchange("A", 10, true)
	offline := NewOfflineFIX("B", 100)
	m := NewManager([]ExchangeAdapter{offline, online})
	for i := 0; i < 20; i++ {
		a, ok := m.Pick("WEIGHTED")
		if !ok {
			t.Fatal("expected healthy adapter")
		}
		if a.Code() != "A" {
			t.Fatalf("picked %s", a.Code())
		}
		m.Release(a.Code())
	}
}

func TestPickKindPrefersFIX(t *testing.T) {
	m := NewManager([]ExchangeAdapter{
		NewMockExchange("MOCK", 100, true),
		NewMockFIX("FIX-LOOP", 1),
	})
	a, ok := m.PickKind("FIX", "WEIGHTED")
	if !ok || a.Kind() != "FIX" {
		t.Fatalf("got %+v %v", a, ok)
	}
	m.Release(a.Code())
}

func TestAllOffline(t *testing.T) {
	a := NewMockExchange("A", 1, true)
	a.SetHealth(Offline)
	m := NewManager([]ExchangeAdapter{a, NewOfflineFIX("B", 1)})
	if _, ok := m.Pick("WEIGHTED"); ok {
		t.Fatal("should not pick offline adapters")
	}
}
