package audit

import (
	"context"
	"encoding/json"
	"net"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/id"
)

type Logger struct {
	DB *pgxpool.Pool
}

func (l Logger) Write(ctx context.Context, action, entityType string, entityID *uuid.UUID, before, after any) {
	if l.DB == nil {
		return
	}
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
	_, _ = l.DB.Exec(ctx, `
		INSERT INTO audit_logs (id, action, actor_id, entity_type, entity_id, request_id, ip, before_json, after_json)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id.New(), action, actor, entityType, entityID, httpx.RequestIDFrom(ctx), ip, b, a)
}
