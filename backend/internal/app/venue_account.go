package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/id"
)

func venueAccountFromRows(rows []map[string]any, clientCode string) string {
	want := strings.TrimSpace(clientCode)
	if want == "" {
		return ""
	}
	fallback := ""
	for _, row := range rows {
		if !strings.EqualFold(positionClientCode(row), want) {
			continue
		}
		acc := firstNonEmpty(
			mapString(row, "account_id"),
			mapString(row, "trading_account"),
			mapString(row, "account_code"),
		)
		if acc == "" {
			continue
		}
		if strings.EqualFold(mapString(row, "kind"), "CASH") {
			if fallback == "" {
				fallback = acc
			}
			continue
		}
		return acc
	}
	return fallback
}

func (a *App) venueAccountForClient(ctx context.Context, clientCode string) string {
	code := strings.TrimSpace(clientCode)
	if code == "" || a.KFB == nil || !a.KFB.Configured() {
		return ""
	}
	snap := a.KFB.Snapshot()
	if acc := venueAccountFromRows(snap.Positions, code); acc != "" {
		return acc
	}
	if acc := venueAccountFromRows(snap.Accounts, code); acc != "" {
		return acc
	}
	pos, err := a.kfbPositions(ctx)
	if err != nil {
		return ""
	}
	return venueAccountFromRows(pos.Positions, code)
}

func (a *App) resolveOrderAccount(ctx context.Context, p httpx.Principal, raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		idv, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, httpx.E(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Некорректный торговый счёт")
		}
		if err := a.requireAccountAccess(ctx, p, idv); err != nil {
			return uuid.Nil, err
		}
		return idv, nil
	}
	if p.ClientID != nil {
		return a.ensureClientVenueAccount(ctx, p, *p.ClientID)
	}
	rows, err := a.DB.Query(ctx, `
		SELECT id FROM trading_accounts
		WHERE broker_id=$1 AND status='ACTIVE'
		ORDER BY created_at LIMIT 2`, p.BrokerID)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var idv uuid.UUID
		if err := rows.Scan(&idv); err != nil {
			return uuid.Nil, err
		}
		ids = append(ids, idv)
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	return uuid.Nil, httpx.E(http.StatusUnprocessableEntity, "TRADING_ACCOUNT_MISSING",
		"Торговый счёт не найден. Клиентский счёт подставляется автоматически с биржи.")
}

func (a *App) ensureClientVenueAccount(ctx context.Context, p httpx.Principal, clientID uuid.UUID) (uuid.UUID, error) {
	code, err := a.clientCodeForUser(ctx, clientID)
	if err != nil {
		return uuid.Nil, err
	}
	if strings.TrimSpace(code) == "" {
		return uuid.Nil, httpx.E(http.StatusUnprocessableEntity, "TRADING_ACCOUNT_MISSING",
			"У клиента не задан код клиента. Без него нельзя определить торговый счёт на бирже.")
	}
	venue := a.venueAccountForClient(ctx, code)
	var existing uuid.UUID
	var existingNumber, existingVenue string
	err = a.DB.QueryRow(ctx, `
		SELECT id, account_number, COALESCE(trading_account,'')
		FROM trading_accounts
		WHERE client_id=$1 AND status='ACTIVE'
		ORDER BY CASE
		  WHEN $2 <> '' AND (trading_account=$2 OR account_number=$2) THEN 0
		  ELSE 1
		END, created_at
		LIMIT 1`, clientID, venue).Scan(&existing, &existingNumber, &existingVenue)
	if err == nil {
		if venue != "" && existingVenue != venue {
			_, _ = a.DB.Exec(ctx, `
				UPDATE trading_accounts
				SET trading_account=$2, client_code=COALESCE(NULLIF(client_code,''), $3)
				WHERE id=$1`, existing, venue, code)
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	if venue == "" {
		return uuid.Nil, httpx.E(http.StatusUnprocessableEntity, "TRADING_ACCOUNT_MISSING",
			"Торговый счёт не найден на бирже для этого клиента")
	}
	return a.createClientVenueAccount(ctx, p, clientID, code, venue)
}

func (a *App) createClientVenueAccount(ctx context.Context, p httpx.Principal, clientID uuid.UUID, clientCode, venue string) (uuid.UUID, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	contractID, err := a.ensureClientContract(ctx, tx, p, clientID)
	if err != nil {
		return uuid.Nil, err
	}
	currency := a.venueCurrencyForClient(ctx, clientCode)
	var curOK bool
	_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM currencies WHERE code=$1)`, currency).Scan(&curOK)
	if !curOK {
		currency = "KGS"
	}
	acctID := id.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO trading_accounts (
			id, account_number, client_id, contract_id, broker_id, currency, status, client_code, trading_account)
		VALUES ($1,$2,$3,$4,$5,$6,'ACTIVE',$7,$8)`,
		acctID, venue, clientID, contractID, p.BrokerID, currency, clientCode, venue)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			var existing uuid.UUID
			qerr := tx.QueryRow(ctx, `
				SELECT id FROM trading_accounts
				WHERE client_id=$1 AND (account_number=$2 OR trading_account=$2)
				LIMIT 1`, clientID, venue).Scan(&existing)
			if qerr == nil {
				if err := tx.Commit(ctx); err != nil {
					return uuid.Nil, err
				}
				return existing, nil
			}
			acctID = id.New()
			number := venue + ":" + clientCode
			if _, err2 := tx.Exec(ctx, `
				INSERT INTO trading_accounts (
					id, account_number, client_id, contract_id, broker_id, currency, status, client_code, trading_account)
				VALUES ($1,$2,$3,$4,$5,$6,'ACTIVE',$7,$8)`,
				acctID, number, clientID, contractID, p.BrokerID, currency, clientCode, venue); err2 != nil {
				return uuid.Nil, dbWriteError(err2)
			}
		} else {
			return uuid.Nil, dbWriteError(err)
		}
	}
	if err := a.ensureCashAccounts(ctx, tx, acctID, currency); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return acctID, nil
}

func (a *App) ensureClientContract(ctx context.Context, tx pgx.Tx, p httpx.Principal, clientID uuid.UUID) (uuid.UUID, error) {
	var contractID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM contracts
		WHERE client_id=$1 AND status='ACTIVE'
		ORDER BY created_at LIMIT 1`, clientID).Scan(&contractID)
	if err == nil {
		return contractID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	contractID = id.New()
	num := "BR-KFB-" + strings.ToUpper(contractID.String()[:8])
	tag, err := tx.Exec(ctx, `
		INSERT INTO contracts (id, contract_number, client_id, broker_id, tariff_plan_id, status, start_date, created_by)
		SELECT $1,$2,$3,$4, tp.id, 'ACTIVE', $5, $6 FROM tariff_plans tp WHERE tp.code='STANDARD'`,
		contractID, num, clientID, p.BrokerID, time.Now().UTC().Format("2006-01-02"), nilIfUUID(p.UserID))
	if err != nil {
		return uuid.Nil, dbWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return uuid.Nil, httpx.E(http.StatusUnprocessableEntity, "TRADING_ACCOUNT_MISSING",
			"Не удалось создать договор: тариф STANDARD не найден")
	}
	return contractID, nil
}

func (a *App) venueCurrencyForClient(ctx context.Context, clientCode string) string {
	cur := "KGS"
	if a.KFB == nil || !a.KFB.Configured() {
		return cur
	}
	snap := a.KFB.Snapshot()
	rows := snap.Positions
	if len(rows) == 0 {
		if pos, err := a.kfbPositions(ctx); err == nil {
			rows = pos.Positions
		}
	}
	for _, row := range rows {
		if !strings.EqualFold(positionClientCode(row), clientCode) {
			continue
		}
		c := strings.ToUpper(mapString(row, "currency"))
		if len(c) == 3 {
			return c
		}
	}
	return cur
}
