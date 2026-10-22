package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/id"
)

type mockCardDepositReq struct {
	AccountID     string `json:"account_id"`
	Amount        string `json:"amount"`
	CardNumber    string `json:"card_number"`
	Expiry        string `json:"expiry"`
	CVC           string `json:"cvc"`
	Cardholder    string `json:"cardholder"`
	IdempotencyKey string `json:"idempotency_key"`
}

// handleMockCardDeposit emulates acquiring in development. It never persists
// the PAN, expiry or CVC; only the card brand and last four digits are retained.
func (a *App) handleMockCardDeposit(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(a.Cfg.Env, "production") {
		httpx.WriteError(w, r, httpx.E(http.StatusNotFound, "NOT_FOUND", "Mock card payments are disabled"))
		return
	}
	var req mockCardDepositReq
	if err := decode(r, &req); err != nil { httpx.WriteError(w, r, err); return }
	p, _ := httpx.PrincipalFrom(r.Context())
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil { httpx.WriteError(w, r, httpx.ErrValidation); return }
	if err := a.requireAccountAccess(r.Context(), p, accountID); err != nil { httpx.WriteError(w, r, err); return }
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || !amount.IsPositive() { httpx.WriteError(w, r, httpx.E(422, "VALIDATION_ERROR", "Укажите корректную сумму")); return }
	pan := strings.NewReplacer(" ", "", "-", "").Replace(req.CardNumber)
	if len(pan) < 4 || len(req.CVC) != 3 || len(req.Expiry) != 5 || strings.TrimSpace(req.Cardholder) == "" {
		httpx.WriteError(w, r, httpx.E(422, "VALIDATION_ERROR", "Проверьте данные тестовой карты")); return
	}
	if pan == "4000000000000002" {
		httpx.WriteError(w, r, httpx.E(402, "CARD_DECLINED", "Тестовый банк отклонил операцию")); return
	}
	brand := "VISA"
	if strings.HasPrefix(pan, "5") { brand = "MASTERCARD" }
	last4 := pan[len(pan)-4:]
	depositID := id.New()
	operationID := "mock_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	idem := req.IdempotencyKey
	if idem == "" { idem = uuid.NewString() }
	tx, err := a.DB.Begin(r.Context())
	if err != nil { httpx.WriteError(w, r, err); return }
	defer tx.Rollback(r.Context())
	var currency string
	if err := tx.QueryRow(r.Context(), `SELECT currency FROM trading_accounts WHERE id=$1 AND status='ACTIVE' FOR UPDATE`, accountID).Scan(&currency); err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound); return
	}
	externalRef := fmt.Sprintf("%s:%s:%s", operationID, brand, last4)
	if _, err := tx.Exec(r.Context(), `INSERT INTO deposits (id, account_id, amount, currency, status, external_ref, idempotency_key, created_by) VALUES ($1,$2,$3,$4,'COMPLETED',$5,$6,$7)`, depositID, accountID, amount, currency, externalRef, idem, p.UserID); err != nil {
		httpx.WriteError(w, r, httpx.ErrConflict); return
	}
	if err := a.postLedger(r.Context(), tx, accountID, currency, "DEPOSIT", depositID, "Mock card deposit", "mock-card-"+idem, []ledgerLine{
		moneyLine("AVAILABLE", "DEPOSIT", decimal.Zero, amount),
		moneyLine("SETTLED", "DEPOSIT", amount, decimal.Zero),
	}, &p.UserID); err != nil { httpx.WriteError(w, r, err); return }
	if err := tx.Commit(r.Context()); err != nil { httpx.WriteError(w, r, err); return }
	a.Audit.Write(r.Context(), "MOCK_CARD_DEPOSIT_COMPLETED", "deposit", &depositID, nil, map[string]any{"amount": amount.String(), "currency": currency, "brand": brand, "last4": last4, "operation_id": operationID})
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"id": depositID, "status": "COMPLETED", "amount": amount.String(), "currency": currency,
		"card_brand": brand, "card_last4": last4, "operation_id": operationID, "processed_at": time.Now().UTC(), "mock": true,
	})
}
