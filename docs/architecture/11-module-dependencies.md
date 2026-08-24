# Module Dependencies

Direction of compile-time imports. Lower layers must not import upper layers.

```
platform (config, db, httpx, money, id, log, telemetry, ws, outbox)
    ↑
auth, roles, users
    ↑
brokers, administration
    ↑
clients, kyc, documents, contracts, accounts
    ↑
instruments, marketdata
    ↑
cash, commissions, risk
    ↑
orders → routing → adapters
    ↑
executions, trades, positions, portfolios
    ↑
settlement, corporateactions, payments
    ↑
reconciliation, reports, notifications, audit
```

Forbidden:

- `orders` importing `fix` codec package
- `cash` importing HTTP handlers
- `marketdata` writing to `trades`
- handlers calling other modules' repositories

Allowed:

- `orders/service` calling `risk.Service`, `cash.Service`, `routing.Service` via interfaces defined in `orders` or a small `internal/ports` package
