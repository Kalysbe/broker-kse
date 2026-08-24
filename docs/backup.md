# Backup and disaster recovery

## PostgreSQL

Enable WAL archiving and take daily base backups.

Example (host):

```bash
pg_basebackup -h postgres -U sbis -D /backup/base-$(date +%F) -Ft -z -P
```

Point-in-time recovery: restore base backup, replay WAL to the target timestamp. Test restores quarterly.

Financial tables (`cash_ledger`, `trades`, `executions`, `audit_logs`, `order_events`) are append-only at the application layer. Do not restore a subset of those tables independently — restore the whole cluster to a consistent LSN.

## Object storage

Replicate the object store (documents, report files) to a second region. Database stores only `storage_key`.

## Adapter / FIX failure

- Adapter Manager excludes OFFLINE sessions.
- Outbox republishes unacked `order.*` events.
- Venue uniqueness is `client_order_id`; recovery must not allocate a new one.

## Verification

After restore:

1. `/health/ready` is 200
2. Ledger trial balance: sum(debit) = sum(credit) per `ledger_transactions.id`
3. Spot-check a client statement vs pre-backup export
4. FIX sessions re-logon; no duplicate live orders
