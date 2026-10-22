package app

import (
	"context"
	"encoding/json"

	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"statebroker/sbis/internal/adapters/kfb"
)

func (a *App) startKFBRealtime(ctx context.Context) {
	if a.KFB == nil {
		return
	}
	go func() {
		delay := time.Second
		for ctx.Err() == nil {
			attemptCtx, cancel := context.WithCancel(ctx)
			version := a.KFB.ConnectionVersion()
			go func() {
				ticker := time.NewTicker(250 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-attemptCtx.Done():
						return
					case <-ticker.C:
						if a.KFB.ConnectionVersion() != version {
							cancel()
							return
						}
					}
				}
			}()
			err := a.KFB.StreamEvents(attemptCtx, func() error {
				if a.KFB.ConnectionVersion() != version {
					return context.Canceled
				}
				// The upstream does not replay missed events: invalidate active snapshots on reconnect.
				if !a.Cfg.ExchangeReadOnly {
					_, err := a.DB.Exec(ctx, `UPDATE orders SET venue_checked_at=NULL WHERE venue_order_id IS NOT NULL AND (status IN ('SENT','ACKNOWLEDGED','PARTIALLY_FILLED','CANCEL_PENDING') OR venue_status IN ('ACKNOWLEDGED','PARTIALLY_FILLED','CANCEL_PENDING'))`)
					if err != nil {
						return err
					}
				}
				if a.Redis != nil {
					if err := a.Redis.Del(ctx, kfbPositionsKey).Err(); err != nil {
						return err
					}
				}
				a.KFBStreamError.Store(nil)
				a.KFBStreamConnected.Store(true)
				delay = time.Second
				a.Log.Info("kfb_realtime_connected")
				return nil
			}, func(kind string, raw json.RawMessage) error {
				if a.KFB.ConnectionVersion() != version {
					return context.Canceled
				}
				if kind == "positions.updated" {
					return a.receiveKFBPositions(ctx, raw)
				}
				if kind == "marketdata.tick" {
					return a.receiveKFBMarket(ctx, raw)
				}
				if a.Cfg.ExchangeReadOnly {
					return nil
				}
				return a.receiveKFBOrder(ctx, raw)
			})
			cancel()
			a.KFBStreamConnected.Store(false)
			if a.KFB.ConnectionVersion() != version {
				delay = time.Second
				continue
			}
			if err != nil {
				message := err.Error()
				a.KFBStreamError.Store(&message)
			}
			if ctx.Err() != nil {
				return
			}
			a.Log.Warn("kfb_realtime_disconnected", zap.Error(err))
			timer := time.NewTimer(delay)
			changeTicker := time.NewTicker(250 * time.Millisecond)
		waitForRetry:
			for {
				select {
				case <-ctx.Done():
					timer.Stop()
					changeTicker.Stop()
					return
				case <-timer.C:
					break waitForRetry
				case <-changeTicker.C:
					if a.KFB.ConnectionVersion() != version {
						delay = time.Second
						break waitForRetry
					}
				}
			}
			timer.Stop()
			changeTicker.Stop()
			if delay < 30*time.Second {
				delay *= 2
			}
		}
	}()
}

func (a *App) receiveKFBOrder(ctx context.Context, raw json.RawMessage) error {
	var remote kfb.RemoteOrder
	if err := json.Unmarshal(raw, &remote); err != nil {
		return err
	}
	if remote.ID == "" {
		return nil
	}
	var ref kfbOrderRef
	ref.venueID = remote.ID
	err := a.DB.QueryRow(ctx, `SELECT id FROM orders WHERE venue_order_id=$1`, remote.ID).Scan(&ref.id)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	a.KFBSyncMu.Lock()
	defer a.KFBSyncMu.Unlock()
	return a.applyKFBOrder(ctx, ref, remote, raw)
}
