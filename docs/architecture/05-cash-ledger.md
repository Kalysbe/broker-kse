# Cash / Ledger Model

## Principle

There is no writable `balance` column that is the source of truth.

Balances are **computed** as the sum of immutable ledger entries per cash account bucket. Optional `balance_cache` exists only as a consistency-checked projection, updated in the same transaction as the posting.

## Chart of buckets

For each `(trading_account_id, currency)`:

| Bucket | Meaning |
|---|---|
| AVAILABLE | Free cash |
| BLOCKED | Reserved for live orders |
| PENDING | Inbound, not yet available |
| SETTLED | Post-settlement control bucket |

House accounts (same ledger, different owner type): COMMISSION, TAX, CLEARING, NOSTRO.

## Double entry

Each `ledger_transactions` row groups 2+ `cash_ledger` lines. Constraint: sum(debit) = sum(credit) inside the transaction.

Lines store `debit` and `credit` as `NUMERIC(28,8)`. Exactly one of them is > 0.

## Entry types

`DEPOSIT`, `WITHDRAWAL`, `BUY`, `SELL`, `COMMISSION`, `TAX`, `ADJUSTMENT`, `SETTLEMENT`, `CORPORATE_ACTION`, `RESERVE`, `RELEASE`

## Example: BUY limit accepted then filled

1. Reserve: AVAILABLE credit? No — AVAILABLE debit, BLOCKED credit (client asset decrease of free cash).  
2. Fill (trade date): BLOCKED debit, CLEARING credit; COMMISSION lines to house.  
3. Settlement: CLEARING vs SETTLED/AVAILABLE according to cash vs security legs.

## Integrity rules

- Application DB user cannot `UPDATE` amount columns on `cash_ledger`
- Negative AVAILABLE is rejected unless the account has an approved overdraft policy
- Concurrent orders use `SELECT … FOR UPDATE` on the cash account row (or advisory lock on account UUID)
- Duplicate deposits/withdrawals keyed by idempotency key
- Adjustments require reason, actor, reference, and audit event `CASH_ADJUSTED`

## Money representation

PostgreSQL `NUMERIC(28,8)`. Go `shopspring/decimal`. Currency table defines `minor_units` (KGS=2, etc.). API serializes as string decimal, never float.
