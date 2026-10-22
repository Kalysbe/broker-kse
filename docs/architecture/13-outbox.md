# Transactional outbox

Domain mutations insert rows into `outbox_events` in the same PostgreSQL
transaction as the aggregate change. The worker publishes those rows to the
Redis Stream `sbis:events`.

## State machine

`PENDING | FAILED -> PROCESSING -> SENT`

- A single atomic `UPDATE ... FOR UPDATE SKIP LOCKED` claims up to 50 rows.
- Claiming increments `attempts` and places a two-minute lease in `available_at`.
- A worker crash leaves the row in `PROCESSING`; another worker reclaims it
  after the lease expires.
- Publication failures move the row to `FAILED`, store `last_error`, and set an
  exponential retry delay capped at 15 minutes.
- Successful publication records `published_at` and clears `last_error`.

## Delivery guarantee

Delivery is **at least once**. A worker can publish to Redis and crash before
marking the PostgreSQL row as `SENT`. Consumers must therefore deduplicate by
the `outbox_id` field in the stream envelope.

The stream envelope contains:

- `outbox_id`
- `aggregate_type`
- `aggregate_id`
- `event_type`
- `payload`
- `attempt`
- `created_at`

Use Redis consumer groups for downstream services. A consumer should store the
processed `outbox_id` in the same transaction as its local side effect whenever
possible.
