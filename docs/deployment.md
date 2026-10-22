# Deployment

## Server deployment

Repository: https://github.com/Kalysbe/broker-kse

The standalone `docker-compose.production.yml` publishes the application on port
80. The browser uses `/api` on the same origin, including WebSocket traffic.
PostgreSQL and Redis have no published ports; diagnostic HTTP and gRPC ports bind
only to server loopback. Long-running services restart after a host reboot.

```bash
git clone https://github.com/Kalysbe/broker-kse.git ~/broker-kse
cd ~/broker-kse
cp .env.example .env
chmod 600 .env
# Set APP_ENV=production, CORS_ORIGINS to the server URL, unique JWT_SECRET,
# HMAC_PEPPER, GATEWAY_DEV_TOKEN, SEED_ADMIN_PASSWORD, and POSTGRES_PASSWORD.
# POSTGRES_PASSWORD must be URL-safe (for example, a generated hex value).
# Configure the exchange token through Integrations -> KFB after signing in.
mkdir -p loadtest/reports
sudo chown 65534:65534 loadtest/reports
docker compose -f docker-compose.production.yml config --quiet
docker compose -f docker-compose.production.yml build
docker compose -f docker-compose.production.yml up -d
curl --fail http://127.0.0.1/health/ready
```

Migrations and administrator seeding complete before the API starts. Seeding
updates the administrator password from `.env` on subsequent deployments. Keep
the JWT secret stable because saved exchange settings are encrypted with it.
The exchange token is entered and saved through the administrator interface;
client certificates and private keys are not required for deployment.

For an update, back up the database, fetch the intended commit, rebuild, then run
`up -d`. To roll back application code, check out the previous commit and repeat
the build and start commands; do not remove volumes or automatically reverse
database migrations. Configure TLS at the edge when a hostname is assigned.

```bash
mkdir -p backups
chmod 700 backups
docker compose -f docker-compose.production.yml exec -T postgres \
  pg_dump -U sbis sbis | gzip > "backups/sbis-$(date +%Y%m%d-%H%M%S).sql.gz"
docker compose -f docker-compose.production.yml ps
docker compose -f docker-compose.production.yml logs --tail=100 api worker nginx
```

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
