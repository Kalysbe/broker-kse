# System Architecture

**System:** State Broker Information System (SBIS)  
**Style:** Modular monolith, extractable bounded contexts  
**Runtime:** Go API + workers, React SPA, PostgreSQL, Redis, object storage  
**Date:** 2026-08-17

## Purpose

Unify the brokerage lifecycle for a state-owned broker:

Client → Account opening → KYC → Funding → Order → OMS → Exchange / FIX / QUIK → Execution → Trade → Settlement → Cash & position change → Portfolio → Reporting

Every operation is traceable end-to-end and has an audit trail.

## Architectural style

A **modular monolith** is the first production shape.

- One deployable Go binary for the HTTP/WebSocket API
- One deployable Go binary for background workers (same modules, different `main`)
- Module boundaries are package-level: `domain`, `service`, `repository`, `handler`, `dto`, `validation`
- No cross-module table writes except through published services
- Order, Execution, Notification, Reporting, and Market Data can later become separate services without rewriting domain logic

Microservices are not created for their own sake.

## Runtime topology

```
 React Frontend (Back Office / Trading / Admin / Client Portal)
                 HTTPS + WebSocket
                         |
                   Nginx (TLS)
                         |
                    Go API (Chi)
                         |
          +--------------+---------------+
          |              |               |
     PostgreSQL        Redis         Object storage
          |
     Outbox table
          |
     Worker processes
          |
     Order Router → Adapter Manager
          |                |
     FIX / QUIK      Exchange / Bank / Depository
```

## Module catalog

| Bounded context | Package | Responsibility |
|---|---|---|
| Platform | `internal/platform` | Config, DB, HTTP, errors, money, IDs, telemetry, WS |
| Auth | `internal/auth` | Login, refresh, 2FA, sessions, step-up, password reset |
| Users / Roles | `internal/users`, `internal/roles` | Staff directory, RBAC |
| Brokers | `internal/brokers` | Broker tenants, API tokens, IP allowlists |
| Clients | `internal/clients` | Natural, legal, institutional, government |
| KYC / AML | `internal/kyc` | Reviews, risk, configurable AML rule sets |
| Documents | `internal/documents` | Metadata + object-storage blobs |
| Contracts | `internal/contracts` | Brokerage agreements, tariffs |
| Accounts | `internal/accounts` | Trading accounts |
| Instruments | `internal/instruments` | Issuers, securities, listings |
| Market data | `internal/marketdata` | Snapshots, books, history (non-transactional) |
| Orders | `internal/orders` | OMS, lifecycle, idempotency |
| Risk | `internal/risk` | Pre-trade checks, configurable limits |
| Routing | `internal/routing` | OrderRouter, AdapterManager, LB, failover |
| Adapters | `internal/adapters` | FIX, QUIK, Exchange, Bank, Depository |
| Executions / Trades | `internal/executions`, `internal/trades` | Fills, trade capture |
| Portfolios | `internal/portfolios`, `internal/positions` | Holdings, P&L |
| Cash | `internal/cash` | Double-entry ledger |
| Payments | `internal/payments` | Deposits, withdrawals, maker-checker |
| Commissions | `internal/commissions` | Tariff engine |
| Settlement | `internal/settlement` | Cash and securities settlement |
| Corporate actions | `internal/corporateactions` | Coupon, dividend, split, redemption |
| Reconciliation | `internal/reconciliation` | Multi-source mismatch queue |
| Notifications | `internal/notifications` | In-app + worker fanout |
| Reports | `internal/reports` | Configurable statements and files |
| Audit / Logging | `internal/audit`, `internal/logging` | Immutable audit, structured logs |
| Administration | `internal/administration` | Settings, policies, health |

## Cross-cutting rules

1. **Identity of the actor** comes from the authenticated principal. `broker_id` in a request body is ignored.
2. **Permissions** are checked in handlers via RBAC middleware, never only in the UI.
3. **Money** is `NUMERIC` with currency-specific scale. Floats are forbidden.
4. **Time** is stored in UTC (`TIMESTAMPTZ`). UI converts to org/user timezone.
5. **Financial rows** are not physically deleted. Status + compensating events only.
6. **Idempotency** is mandatory for order, payment, and adapter publish paths.
7. **Secrets** live in environment / secret manager. Never in Git.

## Scaling path

Keep the monolith until a module has independent scale, failure, or release needs. Extraction order:

1. Market Data Service  
2. Notification Service  
3. Order Service + Execution Service  
4. Reporting Service  

Shared contracts remain: UUID identifiers, outbox events, RBAC permission names, money/currency types.
