package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"statebroker/sbis/internal/outbox"
	"time"

	"statebroker/sbis/internal/adapters/kfb"
)

const kfbPositionsKey = "sbis:kfb:live-positions"

func (a *App) receiveKFBPositions(ctx context.Context, raw json.RawMessage) error {
	var snapshot struct {
		Cash        json.RawMessage `json:"cash"`
		Instruments json.RawMessage `json:"instruments"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return err
	}
	// A missing or malformed array must not erase the last valid snapshot.
	for _, data := range []json.RawMessage{snapshot.Cash, snapshot.Instruments} {
		var rows []map[string]any
		if len(data) == 0 || string(data) == "null" {
			return fmt.Errorf("KFB positions snapshot requires cash and instruments arrays")
		}
		if err := json.Unmarshal(data, &rows); err != nil {
			return err
		}
	}
	if a.Redis == nil {
		return fmt.Errorf("KFB positions cache is unavailable")
	}
	// Every event replaces the entire snapshot, including empty arrays.
	pipe := a.Redis.TxPipeline()
	pipe.Set(ctx, kfbPositionsKey, []byte(raw), 0)
	pipe.XAdd(ctx, &redis.XAddArgs{Stream: outbox.DefaultStream, Values: map[string]any{"outbox_id": uuid.NewString(), "aggregate_type": "positions", "aggregate_id": uuid.NewString(), "event_type": "positions.updated", "created_at": time.Now().UTC().Format(time.RFC3339Nano), "payload": "{}"}})
	_, err := pipe.Exec(ctx)
	return err
}

func (a *App) kfbPositions(ctx context.Context) (kfb.PositionsResult, error) {
	if a.Redis != nil && a.KFBStreamConnected.Load() {
		if raw, err := a.Redis.Get(ctx, kfbPositionsKey).Bytes(); err == nil {
			return kfb.ParsePositions(raw), nil
		}
	}
	return a.KFB.FetchPositions(ctx)
}
