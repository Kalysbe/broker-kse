# Order Lifecycle

## States

Staff OMS: `NEW` → `VALIDATING` → `VALIDATED` → `PENDING` → `ROUTING` → `SENT` → `ACKNOWLEDGED` → `PARTIALLY_FILLED` → `FILLED`

Broker Gateway (stage 1): `QUEUED` → `ROUTING` → `SENT` → `ACKNOWLEDGED` → …

Terminal / exception: `CANCEL_PENDING`, `CANCELLED`, `REJECTED`, `EXPIRED`, `FAILED`

## Allowed transitions

| From | To |
|---|---|
| QUEUED | ROUTING, REJECTED, FAILED |
| NEW | VALIDATING, REJECTED, FAILED |
| VALIDATING | VALIDATED, REJECTED, FAILED |
| VALIDATED | PENDING, REJECTED |
| PENDING | ROUTING, CANCELLED, REJECTED, FAILED |
| ROUTING | SENT, FAILED, REJECTED |
| SENT | ACKNOWLEDGED, REJECTED, FAILED, CANCEL_PENDING |
| ACKNOWLEDGED | PARTIALLY_FILLED, FILLED, CANCEL_PENDING, EXPIRED, REJECTED |
| PARTIALLY_FILLED | PARTIALLY_FILLED, FILLED, CANCEL_PENDING, EXPIRED, CANCELLED |
| CANCEL_PENDING | CANCELLED, PARTIALLY_FILLED, FILLED, REJECTED |
| FILLED, CANCELLED, REJECTED, EXPIRED, FAILED | (terminal) |

Any other transition returns `ORDER_INVALID_TRANSITION`.

## Idempotency

Unique `(broker_id, client_order_id)` (and for staff UI, `(user_id, client_order_id)` when broker_id is the house broker).

Retry of `ORDER-123` after timeout returns the existing order, HTTP 200, same body. It does not create a second reservation or a second venue message if the outbox already has `order.sent` for that id.

## Pre-trade risk (before PENDING)

Configurable checks, not hardcoded business constants:

- Account and instrument status
- Trading session / calendar
- Available cash (BUY) / available quantity (SELL)
- Client, account, broker, instrument, trader limits
- Max order qty / max notional
- Venue-supported order type and TIF

Failure example: `INSUFFICIENT_FUNDS`

## Routing

```
OMS → Risk → reserve ledger → OrderRouter → AdapterManager
  → healthy adapters (weighted / least-conn)
  → FIX | QUIK | Exchange | Mock
```

Offline adapters are never selected. Failover retries remaining healthy adapters. Exhaustion → `FAILED` with `ADAPTER_UNAVAILABLE`, cash reservation released.

## Recovery

On process crash: outbox worker republishes unacked events. Venue-side uniqueness is `client_order_id`. Duplicate ExecutionReports are matched by `exec_id` unique constraint.
