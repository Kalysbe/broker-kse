package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"statebroker/sbis/internal/loadtest"
	"statebroker/sbis/internal/platform/httpx"
	"strconv"
	"strings"
)

func loadParticipants(rows []map[string]any, account string) ([]loadtest.Participant, error) {
	participants := make([]loadtest.Participant, 0, 4)
	for _, want := range []string{"Kalys", "Imar", "Temirlan", "Emil"} {
		found := false
		for _, row := range rows {
			code := positionClientCode(row)
			venue := firstNonEmpty(mapString(row, "account_id"), mapString(row, "account_code"), mapString(row, "trading_account"))
			if strings.EqualFold(code, want) && venue == account {
				participants = append(participants, loadtest.Participant{AccountID: venue, ClientCode: code})
				found = true
				break
			}
		}
		if !found {
			return nil, httpx.E(422, "LOAD_CLIENT_ACCOUNT_NOT_FOUND", fmt.Sprintf("Шлюз не подтвердил счёт %s для клиента %s", account, want))
		}
	}
	return participants, nil
}

// Only the token of a server-created active test may use its verified client
// pool on a shared venue account. Other gateway requests keep their check.
func (a *App) approvedLoadParticipant(ctx context.Context, tx pgx.Tx, p httpx.Principal, req gwOrderReq) (bool, error) {
	if p.TokenID == nil || !strings.HasPrefix(req.ClientOrderID, "LOAD-") || len(req.ClientOrderID) < 43 {
		return false, nil
	}
	runID, err := uuid.Parse(req.ClientOrderID[5:41])
	if err != nil || req.ClientOrderID[41] != '-' {
		return false, nil
	}
	seq, err := strconv.Atoi(req.ClientOrderID[42:])
	if err != nil || seq < 0 {
		return false, nil
	}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT payload FROM load_test_runs WHERE id=$1`, runID).Scan(&raw)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var run loadTestRun
	if err := json.Unmarshal(raw, &run); err != nil {
		return false, err
	}
	if run.TokenID != *p.TokenID || run.BrokerID != p.BrokerID || run.Status != "RUNNING" || seq >= run.Planned {
		return false, nil
	}
	for _, participant := range run.Participants {
		if participant.AccountID == req.AccountID && participant.ClientCode == req.ClientCode {
			return true, nil
		}
	}
	return false, nil
}
