# Reconciliation Architecture

## Goal

Detect and queue mismatches between:

Internal books ↔ QUIK ↔ Exchange ↔ Central depository ↔ Bank

## Job model

`reconciliation_jobs`: type, sources, as-of timestamp, status, started/finished, actor (system or user).

`reconciliation_items`: job_id, mismatch_type, internal_ref, external_ref, internal_amount, external_amount, status (`OPEN`, `INVESTIGATING`, `RESOLVED`, `WRITTEN_OFF`).

Mismatch types: `CASH_MISMATCH`, `POSITION_MISMATCH`, `TRADE_MISMATCH`, `SETTLEMENT_MISMATCH`.

## Process

1. Worker snapshots internal aggregates as-of T  
2. Pulls adapter extracts (or files) for the same T  
3. Matches on account/instrument/trade id / amount / qty  
4. Unmatched rows become queue items  
5. Resolution is a compensating ledger/position event with reason — never a silent UPDATE of a trade price

## Scheduling

Corporate action worker, settlement worker, and recon worker are independent. Recon typically runs after settlement window.
