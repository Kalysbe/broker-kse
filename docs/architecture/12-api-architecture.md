# API Architecture

Base path: `/v1`  
Auth: `Authorization: Bearer <access_token>`  
Idempotency: `Idempotency-Key` and/or `client_order_id`  
Errors:

```json
{
  "error": {
    "code": "INSUFFICIENT_FUNDS",
    "message": "Insufficient available funds",
    "request_id": "req_01H..."
  }
}
```

No stack traces in responses.

## Surface

| Area | Prefix |
|---|---|
| Auth | `/v1/auth` |
| Users / roles | `/v1/users`, `/v1/roles` |
| Clients / KYC / docs | `/v1/clients`, `/v1/kyc`, `/v1/documents` |
| Contracts / accounts | `/v1/contracts`, `/v1/accounts` |
| Instruments | `/v1/issuers`, `/v1/instruments`, `/v1/securities` |
| Market data | `/v1/market-data` |
| Orders / trades | `/v1/orders`, `/v1/executions`, `/v1/trades` |
| Portfolio / cash | `/v1/portfolio`, `/v1/positions`, `/v1/cash` |
| Payments | `/v1/deposits`, `/v1/withdrawals` |
| Settlement / recon | `/v1/settlements`, `/v1/reconciliation` |
| Commissions | `/v1/tariffs`, `/v1/commissions` |
| Brokers / tokens | `/v1/brokers`, `/v1/tokens` |
| Adapters | `/v1/adapters` |
| Reports | `/v1/reports` |
| Admin | `/v1/admin`, `/v1/audit`, `/v1/logs`, `/v1/notifications` |
| Health | `/health`, `/health/live`, `/health/ready`, `/ready` |
| Broker Gateway (stage 1) | `/api/v1/*` (Bearer `sk_test_` / `sk_live_`, polling, no WebSocket) |
| Swagger | `/swagger`, `/openapi.yaml` |
| WebSocket | `/v1/ws` (staff UI only; brokers poll REST) |

Staff errors use `{error:{code,message,request_id}}`. Broker Gateway errors use `{error:"text",details:{}}`.

## WebSocket topics

`orders`, `executions`, `trades`, `marketdata`, `notifications`, `adapters`, `alerts`

Subscribe after auth frame. Fanout via Redis pub/sub so multiple API replicas work.

## Versioning

URL version `/v1`. Additive fields are compatible. Breaking changes require `/v2`.
