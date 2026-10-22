package outbox

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type recordingWriter struct {
	stream string
	values map[string]any
	err    error
}

func (w *recordingWriter) Add(_ context.Context, stream string, values map[string]any) (string, error) {
	w.stream = stream
	w.values = values
	return "1-0", w.err
}

func TestBackoffIsExponentialAndCapped(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, time.Second},
		{1, time.Second},
		{2, 2 * time.Second},
		{5, 16 * time.Second},
		{20, maxBackoff},
	}
	for _, tt := range tests {
		if got := backoff(tt.attempt); got != tt.want {
			t.Fatalf("backoff(%d) = %s, want %s", tt.attempt, got, tt.want)
		}
	}
}

func TestRedisPublisherBuildsDurableEnvelope(t *testing.T) {
	writer := &recordingWriter{}
	event := Event{
		ID:            uuid.New(),
		AggregateType: "order",
		AggregateID:   uuid.New(),
		Type:          "order.updated",
		Payload:       []byte(`{"status":"FILLED"}`),
		Attempts:      2,
		CreatedAt:     time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC),
	}

	if err := (RedisPublisher{Writer: writer}).Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if writer.stream != DefaultStream {
		t.Fatalf("stream = %q, want %q", writer.stream, DefaultStream)
	}
	if writer.values["outbox_id"] != event.ID.String() || writer.values["event_type"] != event.Type {
		t.Fatalf("unexpected envelope: %#v", writer.values)
	}
	if writer.values["payload"] != string(event.Payload) {
		t.Fatalf("payload = %q", writer.values["payload"])
	}
}

func TestRedisPublisherPropagatesTransportError(t *testing.T) {
	want := errors.New("redis unavailable")
	err := (RedisPublisher{Writer: &recordingWriter{err: want}}).Publish(context.Background(), Event{})
	if !errors.Is(err, want) {
		t.Fatalf("Publish() error = %v, want %v", err, want)
	}
}

func TestTruncateErrorBoundsDatabaseValue(t *testing.T) {
	got := truncateError(errors.New(strings.Repeat("x", 2500)))
	if len(got) != 2000 {
		t.Fatalf("len(truncateError) = %d, want 2000", len(got))
	}
}
