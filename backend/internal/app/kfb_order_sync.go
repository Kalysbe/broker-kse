package app

import (
	"context"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"time"
)

type kfbOrderRef struct {
	id      uuid.UUID
	venueID string
}

// syncKFBOrderStatuses reconciles non-terminal local orders with the actual
// state returned by the KFB gateway. HTTP acceptance is not exchange acceptance.
func (a *App) syncKFBOrderStatuses(ctx context.Context) error {
	a.KFBSyncMu.Lock()
	defer a.KFBSyncMu.Unlock()
	full := !a.KFBStreamConnected.Load()
	if full && time.Now().Unix()-a.KFBLastReconcile.Load() < 30 {
		return nil
	}
	if full {
		a.KFBLastReconcile.Store(time.Now().Unix())
	}
	if a.KFB == nil || !a.KFB.Configured() {
		return nil
	}
	rows, err := a.DB.Query(ctx, `SELECT id, venue_order_id FROM orders WHERE venue_order_id IS NOT NULL AND venue_order_id NOT LIKE 'VENUE-%' AND ($1 OR venue_checked_at IS NULL) AND (status IN ('SENT','ACKNOWLEDGED','PARTIALLY_FILLED','CANCEL_PENDING') OR venue_status IN ('ACKNOWLEDGED','PARTIALLY_FILLED','CANCEL_PENDING')) ORDER BY venue_checked_at NULLS FIRST,created_at DESC LIMIT 100`, full)
	if err != nil {
		return err
	}
	var refs []kfbOrderRef
	for rows.Next() {
		var ref kfbOrderRef
		if rows.Scan(&ref.id, &ref.venueID) == nil {
			refs = append(refs, ref)
		}
	}
	rows.Close()
	for _, ref := range refs {
		remote, err := a.KFB.FetchOrder(ctx, ref.venueID)
		if err != nil {
			if _, saveErr := a.DB.Exec(ctx, `UPDATE orders SET venue_checked_at=now(),venue_check_error=$2 WHERE id=$1`, ref.id, err.Error()); saveErr != nil {
				return saveErr
			}
			if a.Log != nil {
				a.Log.Warn("kfb_order_status", zap.String("order_id", ref.id.String()), zap.Error(err))
			}
			continue
		}
		if err := a.applyKFBOrder(ctx, ref, remote); err != nil {
			return err
		}
	}
	return nil
}
