# Broker gRPC realtime admission

`sbis.realtime.v1.RealtimeService/Connect` now uses the shared token-bucket/GCRA
admission engine from FIX_GateWay. Broker identity comes from the verified JWT
`broker_id` claim and the existing session authentication. Buckets use the broker
UUID, not user ID, session ID, IP, API token or gRPC connection. Different users
and streams of the same broker share capacity.

The current protobuf supports Subscribe, Unsubscribe and Ping. It does not
support SubmitOrder or CancelOrder, so this integration protects realtime
commands and stream setup. REST trading and the existing outgoing Redis-backed
KFB budget are unchanged. No protobuf contract was changed.

## Admission and defaults

Global aggregate (optional), global System, broker aggregate (optional), broker
System, and the specific operation policies apply together. Rate, burst and
concurrency are independent. No message waits for limiter capacity.

Migration 000017 seeds configurable policies in `realtime_rate_limits`:

- Global System: 1000/s, burst 200, concurrency 256.
- Each broker's System: 100/s, burst 50, concurrency 64.
- Each broker's Connect: 5/s, burst 10, at most 8 open streams.
- Each broker's Subscribe and Unsubscribe: 20/s, burst 10, concurrency 32 each.
- Each broker's Ping: 100/s, burst 50, concurrency 32.

Connect holds its concurrency admission until the stream closes, including its
parent System gates. Each incoming message also consumes category/operation
admission. Command concurrency is held through command processing and response
send; outbound exchange events do not acquire inbound command tokens. The
existing command/event channels remain bounded. Server receive messages are
limited to 64 KiB and HTTP/2 connection streams to 128.

An exhausted budget terminates the gRPC stream with `ResourceExhausted` plus
`google.rpc.RetryInfo`. The SSE bridge maps an initial admission rejection to
HTTP 429 with Retry-After. Once SSE headers have been sent, a gRPC rejection ends
the stream and the existing browser reconnect mechanism handles recovery.

## Administration

GET and PUT `/v1/realtime/rate-limits` require the existing SUPER_ADMIN role.
Ordinary brokers, clients and ADMIN users cannot change broker/global budgets.
GET returns persisted policies and per-process usage. PUT accepts:

```json
{
  "scope": "broker",
  "subject": "f8dfd1a6-690f-48b6-909c-cd8c7519d76d",
  "operation": "Ping",
  "rate": 100,
  "burst": 50,
  "concurrent": 32,
  "enabled": true,
  "reason": "Approved broker traffic budget"
}
```

Subject `*` in broker scope is a default template applied independently to each
authenticated broker. A broker UUID overrides that template for one operation.
Global subject must be `*`; global operations are `*` or System. Broker operation
names are `*`, System, Connect, Subscribe, Unsubscribe, Ping and UnknownCommand.
Disabling a policy removes enforcement at that layer but retains its telemetry;
parent policies still apply. Raising Ping alone does not raise System capacity.

Policy and audit commit atomically using the existing audit logger: actor,
broker, before/after, reason, request ID and timestamp. Refresh runs each second;
administrative writes refresh immediately. Changes do not restart active streams
or reset exhausted buckets. Lower concurrency affects new operations/connections;
existing streams stay open. Initial configuration failure prevents API startup;
later database failures retain the last valid policy snapshot.

## Deployment and monitoring

Apply migration 000017 via the existing migrate service before starting the
rebuilt API. The worker does not host this gRPC service. REALTIME_LIMIT_MAX_STATES
controls the bounded in-memory bucket count, default 100000. Exhaustion rejects
new runtime buckets, and process restart restores settings with fresh runtime
tokens/counters. No Redis round trip occurs for individual admission decisions.
Redis remains the existing event delivery backend.

The current Compose deployment has one API process. **Admission is local to that
process.** For multiple API replicas, pin all connections of a broker to one
replica and partition global capacity, or add distributed admission. Existing
Redis event delivery does not make these counters or concurrency limits distributed.

The `grpc_rate_limit_*` metrics are exposed by the existing `/metrics` endpoint.
Labels scope/broker_id/method represent individual policy layers; do not sum
category and operation counters as accepted order counts. Sampled structured
warnings include broker_id, method, configured rate/burst, current rate, reason
and request_id. Outgoing order/trade/position events use their existing delivery
and authorization checks independently of inbound rates.

## Verification

Run `go test -race ./...` and `go vet ./...`. The opt-in native gRPC integration
test requires an empty disposable database named realtime_limit_test and a
dedicated Redis host named broker-app-realtime-test-redis via
REALTIME_LIMIT_TEST_DATABASE_URL / REALTIME_LIMIT_TEST_REDIS_URL. It tests two
users and streams of one broker, a second independent broker, stream concurrency,
live policy refresh, outgoing events and policy recovery after restart. The
standalone admission unit/load tests cover rate, burst, refill, concurrency,
global protection and bounded memory without submitting real trading orders.
