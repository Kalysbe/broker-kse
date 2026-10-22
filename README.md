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

Order throughput and loss reconciliation: `loadtest/README.md`.

## Docs

- Architecture: `docs/architecture/`
- Security: `docs/security.md`
- Deployment: `docs/deployment.md`
- Backup: `docs/backup.md`
- RBAC: `docs/architecture/03-rbac-matrix.md`
- OpenAPI: `api/openapi.yaml`

### Лимиты КФБ и комната ожидания

KFB_TRADING_RPS=20 ограничивает торговые запросы (создание и отмену заявок), KFB_READ_RPS=100 — чтение. API и worker используют два общих бюджета Redis для адреса шлюза. HTTP 429 приостанавливает только соответствующий бюджет с учётом Retry-After (по умолчанию 1 секунда); ответ сохраняется в аудите.

Нагрузочные тесты из интерфейса сначала сохраняют заявки в нашей очереди QUEUED в PostgreSQL. Скорость генератора может превышать лимит биржи: превышение накапливается в комнате ожидания. После генерации тест переходит в DRAINING и завершается после отправки всех принятых заявок. Панель показывает принятые, ожидающие, переданные шлюзу, ошибки, возраст самой старой заявки и минимальное время разгрузки. Ожидание не считается потерей или ошибкой. Остановка генератора прекращает создание новых заявок, уже принятые продолжают отправляться.

Параметры теста и журнал запросов сохраняются в load_test_runs и load_test_results. После перезапуска сервис восстанавливает последний тест и продолжает обработку принятых заявок; прерванная генерация отмечается отдельно. Для самостоятельного прямого запуска orderload против биржи используйте LOAD_KFB_LIMIT=true, REDIS_URL, KFB_TRADING_RPS и KFB_READ_RPS. Такой запуск не использует сохраняемую комнату ожидания; для теста очереди запускайте через интерфейс либо направляйте CLI в наш API.

Тесты перемешивают клиентов Kalys, Imar, Temirlan и Emil на общем счёте 1-3101-2040. Каждая полная группа из четырёх заявок включает всех клиентов по одному разу, в случайном порядке. Перед запуском шлюз подтверждает связь клиента со счётом; фактический код клиента (включая imar) сохраняется в каждой записи и JSON/CSV отчётах. Использование кодов общего счёта разрешено только тестовому токену активного сохранённого теста с проверенным пулом клиентов.
