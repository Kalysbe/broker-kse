# RBAC Matrix

Permission codes are stable API contracts. Frontend menus hide items the user cannot call; the backend still enforces them.

## Roles

| Role | Intent |
|---|---|
| SUPER_ADMIN | Break-glass, role assignment, org policy |
| ADMIN | Users, settings, non-financial administration |
| BROKER_MANAGER | Broker tenant operations |
| BROKER_OPERATOR | Day-to-day broker operations |
| TRADER | Order entry and trading terminal |
| RISK_MANAGER | Limits, pre-trade overrides (dual control) |
| COMPLIANCE_OFFICER | KYC/AML review |
| ACCOUNTANT | Cash books, commissions, statements |
| SETTLEMENT_OFFICER | Settlement posting |
| BACK_OFFICE | Contracts, accounts, documents, post-trade ops |
| CLIENT_MANAGER | Client CRM (no KYC decision, no cash approve) |
| AUDITOR | Read audit, reports, logs |
| VIEWER | Read dashboards and reference data |

## Permission catalog (excerpt)

Identity: `auth.session.read`, `users.read`, `users.write`, `roles.read`, `roles.assign`  
Clients: `clients.read`, `clients.write`, `kyc.read`, `kyc.review`, `documents.read`, `documents.write`  
Accounts: `accounts.read`, `accounts.write`, `contracts.read`, `contracts.write`  
Trading: `orders.read`, `orders.submit`, `orders.cancel`, `trades.read`, `marketdata.read`  
Risk: `risk.limits.read`, `risk.limits.write`  
Cash: `cash.read`, `payments.create`, `payments.approve`, `payments.reject`  
Portfolio: `portfolio.read`  
Settlement: `settlement.read`, `settlement.post`  
Commissions: `tariffs.read`, `tariffs.write`  
Integrations: `adapters.read`, `adapters.operate`  
Brokers: `brokers.read`, `brokers.write`, `tokens.write`  
Reports: `reports.run`, `reports.export`  
Admin: `admin.settings`, `audit.read`, `logs.read`, `notifications.manage`

## Matrix

| Permission | SUPER_ADMIN | ADMIN | BROKER_MGR | OPERATOR | TRADER | RISK | COMPLIANCE | ACCOUNTANT | SETTLEMENT | BACK_OFFICE | CLIENT_MGR | AUDITOR | VIEWER |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| users.write | Y | Y | | | | | | | | | | | |
| roles.assign | Y | | | | | | | | | | | | |
| clients.write | Y | | Y | Y | | | | | | Y | Y | | |
| kyc.review | Y | | | | | | Y | | | | | | |
| orders.submit | Y | | | | Y | | | | | | | | |
| orders.cancel | Y | | | Y | Y | Y | | | | Y | | | |
| risk.limits.write | Y | | | | | Y | | | | | | | |
| payments.create | Y | | | | | | | Y | | Y | | | |
| payments.approve | Y | | | | | | | | | | | | * |
| settlement.post | Y | | | | | | | | Y | | | | |
| tariffs.write | Y | | Y | | | | | Y | | | | | |
| tokens.write | Y | | Y | | | | | | | | | | |
| adapters.operate | Y | Y | | | | | | | | | | | |
| reports.export | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | |
| audit.read | Y | Y | | | | | Y | | | | | Y | |
| admin.settings | Y | Y | | | | | | | | | | | |

\* `payments.approve` is granted to a dedicated checker role or dual-control ACCOUNTANT group via role assignment, never to the same principal who created the request when `forbid_self_approval` is true.

Full seed lives in `backend/migrations/000002_seed_rbac.up.sql`.
