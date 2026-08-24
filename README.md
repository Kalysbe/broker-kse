# State Broker Information System (SBIS)

Production-oriented modular monolith for a state brokerage: onboarding, KYC, trading accounts, OMS, ledger, settlement, adapters (FIX/QUIK/exchange/bank/depository), reporting, and audit.

Architecture is documented in `docs/architecture/`. Visual overview: open the architecture canvas beside chat.

## Stack

- Backend: Go 1.24, Chi, pgx, Zap, OpenTelemetry, Prometheus
- Frontend: React, TypeScript, Vite, TanStack Query, Tailwind
- PostgreSQL 16, Redis 7, Docker Compose, Nginx

## Quick start

```bash
cp .env.example .env
docker compose up --build
```

Then:

```bash
docker compose run --rm seed
```

- API: http://localhost:18080
- UI: http://localhost:15173
- Health: http://localhost:18080/health/ready
- Default staff login: `admin` / `ChangeMe_Admin12` (change immediately)

Local frontend without Docker:

```bash
cd frontend && npm install && npm run dev
cd backend && go run ./cmd/api
```

## Core invariant

CLIENT → ACCOUNT → ORDER → EXECUTION → TRADE → SETTLEMENT → LEDGER → PORTFOLIO → REPORT

Cash balances are computed from immutable `cash_ledger` rows. Order status transitions are enforced by a state machine. Duplicate `client_order_id` per broker is idempotent.

## Tests

```bash
cd backend
go test ./...
```

Load scenarios: `loadtest/k6.js`.

## Docs

- Architecture: `docs/architecture/`
- Security: `docs/security.md`
- Deployment: `docs/deployment.md`
- Backup: `docs/backup.md`
- RBAC: `docs/architecture/03-rbac-matrix.md`
- OpenAPI: `api/openapi.yaml`
