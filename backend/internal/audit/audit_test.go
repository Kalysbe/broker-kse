package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"statebroker/sbis/internal/platform/httpx"
)

type recordingExecer struct {
	err  error
	args []any
}

func (r *recordingExecer) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	r.args = args
	return pgconn.NewCommandTag("INSERT 0 1"), r.err
}

func TestWritePropagatesDatabaseError(t *testing.T) {
	want := errors.New("audit unavailable")
	db := &recordingExecer{err: want}

	err := write(context.Background(), db, "ORDER_CREATED", "order", nil, nil, nil)
	if !errors.Is(err, want) {
		t.Fatalf("write() error = %v, want %v", err, want)
	}
}

func TestWriteUsesPrincipalActor(t *testing.T) {
	userID := uuid.New()
	db := &recordingExecer{}
	ctx := httpx.WithPrincipal(context.Background(), httpx.Principal{UserID: userID, IP: "127.0.0.1"})

	if err := write(ctx, db, "ORDER_CREATED", "order", nil, nil, map[string]string{"status": "NEW"}); err != nil {
		t.Fatalf("write() error = %v", err)
	}
	if len(db.args) != 9 {
		t.Fatalf("argument count = %d, want 9", len(db.args))
	}
	actor, ok := db.args[2].(*uuid.UUID)
	if !ok || actor == nil || *actor != userID {
		t.Fatalf("actor = %#v, want %s", db.args[2], userID)
	}
	if db.args[6] == nil {
		t.Fatal("ip argument is nil")
	}
}
