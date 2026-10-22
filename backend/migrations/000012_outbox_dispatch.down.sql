DROP INDEX IF EXISTS idx_outbox_dispatch;

CREATE INDEX idx_outbox_pending
ON outbox_events(available_at)
WHERE status IN ('PENDING','FAILED');
