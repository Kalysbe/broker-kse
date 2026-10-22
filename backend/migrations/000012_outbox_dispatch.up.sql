DROP INDEX IF EXISTS idx_outbox_pending;

CREATE INDEX idx_outbox_dispatch
ON outbox_events(status, available_at, created_at)
WHERE status IN ('PENDING','FAILED','PROCESSING');
