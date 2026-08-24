# Deployment

## Environments

| Variable | Purpose |
|---|---|
| `APP_ENV` | `development` / `production` |
| `DATABASE_URL` | PostgreSQL DSN |
| `REDIS_URL` | Redis DSN |
| `JWT_SECRET` | HS256 secret, ≥32 bytes |
| `HMAC_PEPPER` | Broker API HMAC |
| `OBJECT_STORE_DIR` | Document blob root (replace with S3-compatible in production) |
| `CORS_ORIGINS` | Comma-separated allowlist |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Optional traces |

Secrets must come from the environment or a secret manager. Do not commit `.env`.

## Docker Compose (dev / single host)

```bash
cp .env.example .env
# edit JWT_SECRET and SEED_ADMIN_PASSWORD
docker compose up --build
```

Host ports (defaults 80/8080/5173/5432/6379 are avoided):

| Service | URL |
|---|---|
| UI | http://localhost:15173 |
| API | http://localhost:18080 |
| Health | http://localhost:18080/health/ready |
| Postgres | localhost:15432 |
| Redis | localhost:16379 |
| Nginx (prod profile) | http://localhost:18000 |

Production profile with Nginx:

```bash
docker compose --profile prod up --build -d
```

Put TLS in front of Nginx (load balancer or certbot). The app expects HTTPS at the edge.

## Linux systemd (sketch)

Run `api` and `worker` as separate units, both with the same `DATABASE_URL`. Migrations run before start:

```bash
migrate -path backend/migrations -database "$DATABASE_URL" up
/usr/local/bin/sbis-api
/usr/local/bin/sbis-worker
```

## Kubernetes (future)

Extract later: Order, Execution, Notification, Reporting, Market Data. Until then deploy:

- Deployment `api` (N replicas) + Service
- Deployment `worker` (1–N; outbox uses `SKIP LOCKED`)
- StatefulSet or managed PostgreSQL with PITR
- Redis for rate limit and WS fanout
- Ingress TLS
- PVC or S3 for object storage

Health probes: `/health/live` (liveness), `/health/ready` (readiness).

## Metrics

Scrape `/metrics` with Prometheus. Import `deploy/grafana/sbis-dashboard.json`.
