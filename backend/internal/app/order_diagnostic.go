package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"statebroker/sbis/internal/adapters/kfb"
	"statebroker/sbis/internal/orders"
	"time"
)

func adapterFailureReason(adapter string, err error) string {
	detail := map[string]any{"code": "ADAPTER_ERROR", "source": "UNKNOWN", "message": err.Error(), "at": time.Now().UTC()}
	var structured map[string]any
	if json.Unmarshal([]byte(err.Error()), &structured) == nil && structured != nil {
		detail = structured
	}
	detail["adapter"] = adapter
	b, _ := json.Marshal(detail)
	return string(b)
}

func isGatewayRateLimit(err error) bool {
	var detail *kfb.RequestError
	return errors.As(err, &detail) && (detail.HTTPStatus == 429 || detail.Stage == "rate_limit_wait")
}

func (a *App) queueRateLimitedOrder(ctx context.Context, tx pgx.Tx, id uuid.UUID, adapter string, err error, actor uuid.UUID) error {
	reason := adapterFailureReason(adapter, err)
	if moveErr := a.moveOrder(ctx, tx, id, orders.Sent, orders.Queued, actor); moveErr != nil {
		return moveErr
	}
	if _, updateErr := tx.Exec(ctx, `UPDATE orders SET reject_reason=$2 WHERE id=$1`, id, reason); updateErr != nil {
		return updateErr
	}
	return a.Audit.WriteTx(ctx, tx, "ORDER_RATE_LIMITED", "order", &id, nil, json.RawMessage(reason))
}
