# Adapter Architecture

Business modules depend on interfaces, never on a vendor SDK.

## ExchangeAdapter

```
SendOrder(ctx, Order) (VenueAck, error)
CancelOrder(ctx, CancelRequest) error
ReplaceOrder(ctx, ReplaceRequest) error
GetMarketData(ctx, InstrumentID) (Snapshot, error)
GetTradingStatus(ctx, ExchangeID) (Status, error)
GetInstrument(ctx, VenueSymbol) (InstrumentRef, error)
HealthCheck(ctx) Health
```

Implementations: `MockExchangeAdapter`, `KSEAdapter` (skeleton ready for venue API), future venues.

## FIXAdapter

Messages: Logon, Logout, Heartbeat, NewOrderSingle, OrderCancelRequest, OrderCancelReplaceRequest, ExecutionReport, OrderCancelReject, Reject.

The FIX session layer is isolated in `internal/adapters/fix`. OMS maps domain orders to a canonical `VenueOrder` DTO, then the FIX codec translates tags. ARQA/QUIK specifics stay behind the codec.

## QUIK

QUIK is reached through FIX (or a QUIK LUA bridge later). Multiple FIX sessions can be registered as separate adapter rows and load-balanced.

## Adapter Manager

- Registry of adapters with weight, max connections, status
- Health probe loop updates `ONLINE` / `DEGRADED` / `OFFLINE`
- Routing policies: `WEIGHTED`, `LEAST_CONNECTIONS`, `FAILOVER`
- Retry with exponential backoff, never to OFFLINE
- Session table `adapter_sessions` for FIX logon state

## BankAdapter / DepositoryAdapter

Same pattern: interface + mock + production stub. Bank: balance, deposit, withdrawal, payment, statement. Depository: positions, transfers, settlements, corporate actions.

## Failure behaviour

If all adapters for a route are OFFLINE, the order moves to `FAILED`, reserved funds are released, notification `ADAPTER_OFFLINE` is emitted, audit is written. No silent drop.
