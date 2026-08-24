# Milestones

Criteria are binary. A milestone is done only when APIs, tables, UI (if listed), and tests exist and the ready-when condition holds.

## Milestone 1 — Foundation

**Implements:** architecture wiring, auth, RBAC, users, clients, KYC shell, contracts, trading accounts  
**Tables:** users, roles, permissions, user_roles, role_permissions, sessions, refresh_tokens, brokers, clients, client_*, contracts, trading_accounts, audit_logs, currencies, organization_policies  
**API:** `/v1/auth/*`, `/v1/users`, `/v1/roles`, `/v1/clients`, `/v1/kyc`, `/v1/contracts`, `/v1/accounts`  
**UI:** Login, Users, Clients, Accounts  
**Tests:** auth, RBAC deny, password hashing, session revoke  
**Ready when:** staff can log in, create a client, attach a contract and an account; every write is audited

## Milestone 2 — Instruments & market data

**Implements:** issuers, securities, instruments, listings, snapshots, order book, history  
**Tables:** issuers, securities, instruments, listings, exchanges, market_snapshots, order_book_levels, market_bars  
**API:** `/v1/issuers`, `/v1/instruments`, `/v1/market-data`  
**UI:** Securities, Market  
**Tests:** listing uniqueness, market data isolated from trades  
**Ready when:** catalog is configurable (no hardcoded instruments in services)

## Milestone 3 — OMS

**Implements:** orders, state machine, risk, idempotency, router stub  
**Tables:** orders, order_events, risk_limits, idempotency_keys, outbox_events  
**API:** `POST /v1/orders`, cancel/replace, get by client_order_id  
**UI:** Order entry  
**Tests:** invalid transition, duplicate client_order_id, insufficient funds  
**Ready when:** duplicate ORDER-123 returns the original order

## Milestone 4 — Trades & portfolio

**Implements:** executions, trades, positions, portfolio aggregation  
**Tables:** executions, trades, positions, position_events  
**API:** `/v1/executions`, `/v1/trades`, `/v1/portfolio`, `/v1/positions`  
**UI:** Trades, Positions, Portfolio  
**Tests:** partial fill, no duplicate exec_id  
**Ready when:** a fill updates position average price and portfolio value

## Milestone 5 — Cash & commissions

**Implements:** ledger, deposits, withdrawals, approval workflow, tariff engine  
**Tables:** cash_accounts, cash_ledger, ledger_transactions, deposits, withdrawals, approval_*, tariff_*, commissions  
**API:** `/v1/cash`, `/v1/payments/*`, `/v1/tariffs`  
**UI:** Cash, Deposits, Withdrawals, Tariffs  
**Tests:** double withdrawal, concurrent debit, no float money  
**Ready when:** balances match ledger sums; self-approval is rejected

## Milestone 6 — Venue adapters

**Implements:** FIX/QUIK/Exchange interfaces, manager, LB, failover, mock adapters  
**Tables:** adapters, adapter_sessions  
**API:** `/v1/adapters`, health  
**UI:** Integrations  
**Tests:** OFFLINE adapter skipped, failover, retry backoff  
**Ready when:** order routes only to ONLINE adapters

## Milestone 7 — Post-trade integrations

**Implements:** settlement, recon, depository and bank adapters  
**Tables:** settlements, reconciliation_*, plus adapter usage  
**API:** `/v1/settlements`, `/v1/reconciliation`  
**UI:** Settlement, Monitoring mismatches  
**Tests:** settlement status machine, mismatch enqueue  
**Ready when:** a trade produces settlement rows; recon job opens mismatches

## Milestone 8 — Control plane

**Implements:** reports, documents/object storage, notifications, structured logging  
**Tables:** report_*, documents, notifications, api_logs, error_logs, business_events  
**API:** `/v1/reports`, `/v1/documents`, `/v1/notifications`  
**UI:** Reports, Documents, Notifications, Audit, Logs  
**Tests:** statement numbers, redacted logs  
**Ready when:** client statement has opening/closing cash and positions

## Milestone 9 — Frontends

**Implements:** trading terminal, back office, admin, client portal routes, i18n ky/ru/en  
**UI:** all sidebar sections  
**Tests:** route guards by permission  
**Ready when:** locale files contain all UI strings; no hardcoded copy in components

## Milestone 10 — Operate

**Implements:** OTel, Prometheus metrics, Grafana dashboard JSON, k6 load tests, security headers  
**Ready when:** `/health/ready` fails if DB is down; load scenarios cover adapter OFFLINE and FIX disconnect

## Implementation note

Code lands in the same repository as these documents. Mock adapters are production-shaped (interface + registry + health), not inline fake balances or fake commissions.
