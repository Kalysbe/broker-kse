package app

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"statebroker/sbis/internal/outbox"
	"time"
)

const kfbQuotesKey = "sbis:kfb:live-quotes"

func (a *App) receiveKFBMarket(ctx context.Context, raw json.RawMessage) error {
	if a.Redis == nil {
		return nil
	}
	var data struct {
		Quotes []map[string]any `json:"quotes"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	pipe := a.Redis.TxPipeline()
	for _, quote := range data.Quotes {
		symbol, _ := quote["symbol"].(string)
		board, _ := quote["board"].(string)
		if symbol == "" {
			continue
		}
		payload, err := json.Marshal(quote)
		if err != nil {
			return err
		}
		pipe.HSet(ctx, kfbQuotesKey, symbol+":"+board, payload)
	}
	pipe.XAdd(ctx, &redis.XAddArgs{Stream: outbox.DefaultStream, Values: map[string]any{"outbox_id": uuid.NewString(), "aggregate_type": "market", "aggregate_id": uuid.NewString(), "event_type": "marketdata.updated", "created_at": time.Now().UTC().Format(time.RFC3339Nano), "payload": "{}"}})
	_, err := pipe.Exec(ctx)
	return err
}

func (a *App) liveKFBQuote(ctx context.Context, symbol, board string) map[string]any {
	if a.Redis == nil || !a.KFBStreamConnected.Load() {
		return nil
	}
	raw, err := a.Redis.HGet(ctx, kfbQuotesKey, symbol+":"+board).Bytes()
	if err != nil {
		return nil
	}
	var quote map[string]any
	if json.Unmarshal(raw, &quote) != nil {
		return nil
	}
	return quote
}

func quoteLevels(value any) []map[string]any {
	result := []map[string]any{}
	rows, _ := value.([]any)
	for _, row := range rows {
		if level, ok := row.(map[string]any); ok {
			result = append(result, map[string]any{"price": level["price"], "quantity": level["size"]})
		}
	}
	return result
}
