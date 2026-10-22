package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultBatchSize = 50
	defaultLease     = 2 * time.Minute
	maxBackoff       = 15 * time.Minute
)

type Event struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	Type          string
	Payload       json.RawMessage
	Attempts      int
	CreatedAt     time.Time
}

type Publisher interface {
	Publish(context.Context, Event) error
}

type Dispatcher struct {
	DB        *pgxpool.Pool
	Publisher Publisher
	BatchSize int
	Lease     time.Duration
}

func (d Dispatcher) Process(ctx context.Context) (int, error) {
	if d.DB == nil || d.Publisher == nil {
		return 0, errors.New("outbox dispatcher is not configured")
	}
	batch := d.BatchSize
	if batch <= 0 {
		batch = defaultBatchSize
	}
	lease := d.Lease
	if lease <= 0 {
		lease = defaultLease
	}
	events, err := d.claim(ctx, batch, lease)
	if err != nil {
		return 0, err
	}

	published := 0
	var batchErr error
	for _, event := range events {
		if err := d.Publisher.Publish(ctx, event); err != nil {
			if markErr := d.markFailed(ctx, event, err); markErr != nil {
				batchErr = errors.Join(batchErr, fmt.Errorf("publish %s: %w; mark failed: %v", event.ID, err, markErr))
			} else {
				batchErr = errors.Join(batchErr, fmt.Errorf("publish %s: %w", event.ID, err))
			}
			continue
		}
		if err := d.markSent(ctx, event.ID); err != nil {
			batchErr = errors.Join(batchErr, fmt.Errorf("mark sent %s: %w", event.ID, err))
			continue
		}
		published++
	}
	return published, batchErr
}

func (d Dispatcher) claim(ctx context.Context, batch int, lease time.Duration) ([]Event, error) {
	rows, err := d.DB.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM outbox_events
			WHERE (status IN ('PENDING','FAILED') AND available_at <= now())
			   OR (status = 'PROCESSING' AND available_at <= now())
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_events o
		SET status='PROCESSING', attempts=o.attempts+1,
		    available_at=now()+$2::interval, last_error=NULL
		FROM candidates c
		WHERE o.id=c.id
		RETURNING o.id, o.aggregate_type, o.aggregate_id, o.event_type,
		          o.payload, o.attempts, o.created_at`, batch, interval(lease))
	if err != nil {
		return nil, fmt.Errorf("claim outbox: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0, batch)
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.AggregateType, &event.AggregateID, &event.Type, &event.Payload, &event.Attempts, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (d Dispatcher) markSent(ctx context.Context, id uuid.UUID) error {
	tag, err := d.DB.Exec(ctx, `
		UPDATE outbox_events
		SET status='SENT', published_at=now(), last_error=NULL
		WHERE id=$1 AND status='PROCESSING'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("outbox lease is no longer owned")
	}
	return nil
}

func (d Dispatcher) markFailed(ctx context.Context, event Event, publishErr error) error {
	tag, err := d.DB.Exec(ctx, `
		UPDATE outbox_events
		SET status='FAILED', available_at=now()+$2::interval, last_error=$3
		WHERE id=$1 AND status='PROCESSING'`, event.ID, interval(backoff(event.Attempts)), truncateError(publishErr))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("outbox lease is no longer owned")
	}
	return nil
}

func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := time.Second << min(attempt-1, 10)
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}

func interval(d time.Duration) string {
	return fmt.Sprintf("%f seconds", d.Seconds())
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 2000 {
		return s[:2000]
	}
	return s
}
