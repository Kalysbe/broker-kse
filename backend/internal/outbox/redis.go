package outbox

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const DefaultStream = "sbis:events"

type StreamWriter interface {
	Add(context.Context, string, map[string]any) (string, error)
}

type RedisStreamWriter struct {
	Client *redis.Client
}

func (w RedisStreamWriter) Add(ctx context.Context, stream string, values map[string]any) (string, error) {
	if w.Client == nil {
		return "", errors.New("redis client is nil")
	}
	return w.Client.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: values}).Result()
}

type RedisPublisher struct {
	Writer StreamWriter
	Stream string
}

func (p RedisPublisher) Publish(ctx context.Context, event Event) error {
	if p.Writer == nil {
		return errors.New("redis stream writer is nil")
	}
	stream := p.Stream
	if stream == "" {
		stream = DefaultStream
	}
	_, err := p.Writer.Add(ctx, stream, map[string]any{
		"outbox_id":      event.ID.String(),
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID.String(),
		"event_type":     event.Type,
		"payload":        string(event.Payload),
		"attempt":        event.Attempts,
		"created_at":     event.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
	return err
}
