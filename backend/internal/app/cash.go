package app

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/id"
)

type ledgerLine struct {
	Bucket    string
	Debit     decimal.Decimal
	Credit    decimal.Decimal
	EntryType string
}

func (a *App) ensureCashAccounts(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, currency string) error {
	buckets := []string{"AVAILABLE", "BLOCKED", "PENDING", "SETTLED"}
	for _, b := range buckets {
		if _, err := tx.Exec(ctx, `
			INSERT INTO cash_accounts (id, trading_account_id, currency, bucket, owner_type)
			VALUES ($1,$2,$3,$4,'CLIENT')
			ON CONFLICT (trading_account_id, currency, bucket) DO NOTHING`,
			id.New(), accountID, currency, b); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) cashAccountID(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, currency, bucket string) (uuid.UUID, error) {
	var idv uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM cash_accounts
		WHERE trading_account_id=$1 AND currency=$2 AND bucket=$3
		FOR UPDATE`, accountID, currency, bucket).Scan(&idv)
	return idv, err
}

func (a *App) availableBalance(ctx context.Context, tx pgx.Tx, cashAccountID uuid.UUID) (decimal.Decimal, error) {
	var bal decimal.Decimal
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(credit - debit), 0) FROM cash_ledger WHERE cash_account_id=$1`, cashAccountID).Scan(&bal)
	return bal, err
}

func (a *App) postLedger(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, currency, refType string, refID uuid.UUID, desc, idem string, lines []ledgerLine, actor *uuid.UUID) error {
	if err := a.ensureCashAccounts(ctx, tx, accountID, currency); err != nil {
		return err
	}
	var debitSum, creditSum decimal.Decimal
	for _, l := range lines {
		debitSum = debitSum.Add(l.Debit)
		creditSum = creditSum.Add(l.Credit)
		if (l.Debit.IsZero() && l.Credit.IsZero()) || (!l.Debit.IsZero() && !l.Credit.IsZero()) {
			return httpx.E(500, "LEDGER_INVALID", "Each ledger line must be debit XOR credit")
		}
	}
	if !debitSum.Equal(creditSum) {
		return httpx.E(500, "LEDGER_UNBALANCED", "Ledger transaction is not balanced")
	}
	txID := id.New()
	var idemArg any
	if idem != "" {
		idemArg = idem
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ledger_transactions (id, reference_type, reference_id, description, created_by, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6)`, txID, refType, refID, desc, actor, idemArg); err != nil {
		return err
	}
	for _, l := range lines {
		ca, err := a.cashAccountID(ctx, tx, accountID, currency, l.Bucket)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO cash_ledger (id, transaction_id, cash_account_id, currency, debit, credit, entry_type, reference_type, reference_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			id.New(), txID, ca, currency, l.Debit, l.Credit, l.EntryType, refType, refID); err != nil {
			return err
		}
	}
	availID, err := a.cashAccountID(ctx, tx, accountID, currency, "AVAILABLE")
	if err != nil {
		return err
	}
	avail, err := a.availableBalance(ctx, tx, availID)
	if err != nil {
		return err
	}
	var overdraft decimal.Decimal
	if err := tx.QueryRow(ctx, `SELECT overdraft_limit FROM trading_accounts WHERE id=$1`, accountID).Scan(&overdraft); err != nil {
		return err
	}
	if avail.LessThan(overdraft.Neg()) {
		return httpx.ErrInsufficientFunds
	}
	return nil
}

func (a *App) accountBalances(ctx context.Context, accountID uuid.UUID) (map[string]string, error) {
	rows, err := a.DB.Query(ctx, `
		SELECT ca.bucket, COALESCE(SUM(cl.credit - cl.debit),0)
		FROM cash_accounts ca
		LEFT JOIN cash_ledger cl ON cl.cash_account_id = ca.id
		WHERE ca.trading_account_id=$1
		GROUP BY ca.bucket`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var b string
		var v decimal.Decimal
		if err := rows.Scan(&b, &v); err != nil {
			return nil, err
		}
		out[b] = v.StringFixed(2)
	}
	return out, rows.Err()
}

func moneyLine(bucket, entry string, debit, credit decimal.Decimal) ledgerLine {
	return ledgerLine{Bucket: bucket, Debit: debit, Credit: credit, EntryType: entry}
}

func (a *App) handleCash(w http.ResponseWriter, r *http.Request) {
	idv, err := chiID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r.Context())
	if err := a.requireAccountAccess(r.Context(), p, idv); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	b, err := a.accountBalances(r.Context(), idv)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"account_id": idv, "buckets": b})
}

func (a *App) handleLedger(w http.ResponseWriter, r *http.Request) {
	idv, err := chiID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r.Context())
	if err := a.requireAccountAccess(r.Context(), p, idv); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := a.DB.Query(r.Context(), `
		SELECT cl.id, cl.entry_type, cl.debit, cl.credit, cl.reference_type, cl.reference_id, cl.created_at, ca.bucket
		FROM cash_ledger cl
		JOIN cash_accounts ca ON ca.id = cl.cash_account_id
		WHERE ca.trading_account_id=$1
		ORDER BY cl.created_at DESC LIMIT 200`, idv)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type row struct {
		ID, EntryType, Debit, Credit, RefType, Bucket string
		RefID                                         uuid.UUID
		CreatedAt                                     interface{}
	}
	var items []map[string]any
	for rows.Next() {
		var idl uuid.UUID
		var et, rt, bucket string
		var debit, credit decimal.Decimal
		var rid uuid.UUID
		var ts interface{}
		if err := rows.Scan(&idl, &et, &debit, &credit, &rt, &rid, &ts, &bucket); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		items = append(items, map[string]any{
			"id": idl, "entry_type": et, "debit": debit.String(), "credit": credit.String(),
			"reference_type": rt, "reference_id": rid, "created_at": ts, "bucket": bucket,
		})
	}
	if items == nil {
		items = []map[string]any{}
	}
	httpx.WriteJSON(w, 200, map[string]any{"items": items})
}
