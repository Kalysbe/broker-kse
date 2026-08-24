package adapters

func Default(extra ...ExchangeAdapter) *Manager {
	list := append([]ExchangeAdapter{}, extra...)
	list = append(list,
		NewMockExchange("MOCK-EXCHANGE", 10, true),
		NewMockFIX("FIX-QUIK-LOOP", 8),
		NewOfflineFIX("FIX-QUIK-1", 5),
		NewOfflineFIX("FIX-QUIK-2", 5),
	)
	return NewManager(list)
}
