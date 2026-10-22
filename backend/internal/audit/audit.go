package audit

import (
	"context"
	"encoding/json"
	"net"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/id"
)

type Logger struct {
	DB *pgxpool.Pool
}

func (l Logger) Write(ctx context.Context, action, entityType string, entityID *uuid.UUID, before, after any) error {
	if l.DB == nil {
		return nil
	}
	return write(ctx, l.DB, action, entityType, entityID, before, after)
}

// WriteTx stores the audit record in the caller's transaction. Financial and
// administrative mutations should use this method so the state change and its
// audit trail either commit together or both roll back.
func (l Logger) WriteTx(ctx context.Context, tx pgx.Tx, action, entityType string, entityID *uuid.UUID, before, after any) error {
	if tx == nil {
		return nil
	}
	return write(ctx, tx, action, entityType, entityID, before, after)
}

type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func write(ctx context.Context, db execer, action, entityType string, entityID *uuid.UUID, before, after any) error {
	var actor *uuid.UUID
	var ip *net.IP
	if p, ok := httpx.PrincipalFrom(ctx); ok {
		if p.UserID != uuid.Nil {
			u := p.UserID
			actor = &u
		}
		if p.IP != "" {
			parsed := net.ParseIP(p.IP)
			if parsed != nil {
				ip = &parsed
			}
		}
	}
	var b, a []byte
	if before != nil {
		b, _ = json.Marshal(before)
	}
	if after != nil {
		a, _ = json.Marshal(after)
	}
	_, err := db.Exec(ctx, `
		INSERT INTO audit_logs (id, action, actor_id, entity_type, entity_id, request_id, ip, before_json, after_json)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id.New(), action, actor, entityType, entityID, httpx.RequestIDFrom(ctx), ip, b, a)
	return err
}
