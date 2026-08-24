# Logging Architecture

## Stack

Zap JSON logs → stdout (container) → optional collector. OpenTelemetry traces and metrics on the same `trace_id`.

## Request fields

`request_id`, `trace_id`, `broker_id`, `client_id`, `user_id`, `token_id`, `ip`, `endpoint`, `method`, `status`, `duration_ms`, `timestamp`

Redacted: password, token, Authorization, cookie, private key, secret, TOTP, recovery codes.

## Streams

| Stream | Destination |
|---|---|
| API logs | Zap + `api_logs` (sampled or all, retention policy) |
| Order logs | Zap + `order_events` |
| FIX logs | Zap (session id, msg type, not raw secrets) + adapter_sessions |
| Auth logs | `security_events` |
| Error logs | Zap error + `error_logs` |
| Audit | `audit_logs` only (immutable) |
| System | Zap + worker heartbeats |

## Metrics (Prometheus)

HTTP count/latency, orders, trades, errors, FIX messages, adapter status, queue depth, DB pool, WS connections.

## Health

- `GET /health` — liveness alias  
- `GET /health/live` — process up  
- `GET /health/ready` — DB, Redis, outbox worker heartbeat, adapter probes
