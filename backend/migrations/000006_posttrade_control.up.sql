CREATE TABLE approval_requests (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  type            TEXT NOT NULL,
  reference_type  TEXT NOT NULL,
  reference_id    UUID NOT NULL,
  status          TEXT NOT NULL CHECK (status IN ('CREATED','SUBMITTED','APPROVED','REJECTED')),
  payload         JSONB NOT NULL DEFAULT '{}',
  created_by      UUID NOT NULL REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE approval_decisions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id      UUID NOT NULL REFERENCES approval_requests(id),
  actor_id        UUID NOT NULL REFERENCES users(id),
  action          TEXT NOT NULL CHECK (action IN ('SUBMIT','APPROVE','REJECT')),
  comment         TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE deposits (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id      UUID NOT NULL REFERENCES trading_accounts(id),
  amount          NUMERIC(28,8) NOT NULL CHECK (amount > 0),
  currency        CHAR(3) NOT NULL REFERENCES currencies(code),
  status          TEXT NOT NULL CHECK (status IN ('REQUESTED','APPROVED','PROCESSING','COMPLETED','REJECTED')),
  external_ref    TEXT,
  idempotency_key TEXT UNIQUE,
  approval_id     UUID REFERENCES approval_requests(id),
  created_by      UUID REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE withdrawals (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id      UUID NOT NULL REFERENCES trading_accounts(id),
  amount          NUMERIC(28,8) NOT NULL CHECK (amount > 0),
  currency        CHAR(3) NOT NULL REFERENCES currencies(code),
  status          TEXT NOT NULL CHECK (status IN ('REQUESTED','APPROVED','PROCESSING','COMPLETED','REJECTED')),
  destination     JSONB NOT NULL DEFAULT '{}',
  idempotency_key TEXT UNIQUE,
  approval_id     UUID REFERENCES approval_requests(id),
  created_by      UUID REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE payments (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id      UUID NOT NULL REFERENCES trading_accounts(id),
  direction       TEXT NOT NULL CHECK (direction IN ('IN','OUT')),
  amount          NUMERIC(28,8) NOT NULL CHECK (amount > 0),
  currency        CHAR(3) NOT NULL,
  status          TEXT NOT NULL,
  bank_ref        TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE settlements (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  trade_id          UUID NOT NULL REFERENCES trades(id),
  account_id        UUID NOT NULL REFERENCES trading_accounts(id),
  instrument_id     UUID NOT NULL REFERENCES instruments(id),
  trade_date        DATE NOT NULL,
  settlement_date   DATE NOT NULL,
  cash_status       TEXT NOT NULL CHECK (cash_status IN ('PENDING','PROCESSING','SETTLED','FAILED')),
  securities_status TEXT NOT NULL CHECK (securities_status IN ('PENDING','PROCESSING','SETTLED','FAILED')),
  cash_amount       NUMERIC(28,8) NOT NULL,
  quantity          NUMERIC(28,8) NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE corporate_actions (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  security_id       UUID NOT NULL REFERENCES securities(id),
  type              TEXT NOT NULL CHECK (type IN ('COUPON','DIVIDEND','SPLIT','REVERSE_SPLIT','REDEMPTION','MATURITY')),
  ex_date           DATE NOT NULL,
  record_date       DATE,
  pay_date          DATE,
  rate              NUMERIC(18,8),
  status            TEXT NOT NULL DEFAULT 'ANNOUNCED' CHECK (status IN ('ANNOUNCED','PROCESSING','APPLIED','CANCELLED')),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE adapters (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code            TEXT NOT NULL UNIQUE,
  kind            TEXT NOT NULL CHECK (kind IN ('FIX','QUIK','EXCHANGE','BANK','DEPOSITORY','MOCK')),
  name            TEXT NOT NULL,
  status          TEXT NOT NULL DEFAULT 'OFFLINE' CHECK (status IN ('ONLINE','DEGRADED','OFFLINE')),
  weight          INT NOT NULL DEFAULT 1 CHECK (weight >= 0),
  max_connections INT NOT NULL DEFAULT 32,
  active_connections INT NOT NULL DEFAULT 0,
  config          JSONB NOT NULL DEFAULT '{}',
  last_health_at  TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE adapter_sessions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  adapter_id      UUID NOT NULL REFERENCES adapters(id),
  protocol        TEXT NOT NULL,
  state           TEXT NOT NULL CHECK (state IN ('DISCONNECTED','LOGON','LOGGED_ON','LOGOUT','ERROR')),
  sender_comp_id  TEXT,
  target_comp_id  TEXT,
  last_heartbeat  TIMESTAMPTZ,
  started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at        TIMESTAMPTZ
);

CREATE TABLE outbox_events (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  aggregate_type  TEXT NOT NULL,
  aggregate_id    UUID NOT NULL,
  event_type      TEXT NOT NULL,
  payload         JSONB NOT NULL,
  status          TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','PROCESSING','SENT','FAILED')),
  attempts        INT NOT NULL DEFAULT 0,
  available_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_at    TIMESTAMPTZ,
  last_error      TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_outbox_pending ON outbox_events(available_at) WHERE status IN ('PENDING','FAILED');

CREATE TABLE idempotency_keys (
  key             TEXT PRIMARY KEY,
  user_id         UUID,
  request_hash    TEXT NOT NULL,
  response_code   INT NOT NULL,
  response_body   JSONB NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE reconciliation_jobs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  job_type        TEXT NOT NULL,
  sources         TEXT[] NOT NULL,
  as_of           TIMESTAMPTZ NOT NULL,
  status          TEXT NOT NULL CHECK (status IN ('PENDING','RUNNING','COMPLETED','FAILED')),
  started_at      TIMESTAMPTZ,
  finished_at     TIMESTAMPTZ,
  created_by      UUID REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE reconciliation_items (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id            UUID NOT NULL REFERENCES reconciliation_jobs(id),
  mismatch_type     TEXT NOT NULL CHECK (mismatch_type IN (
                      'CASH_MISMATCH','POSITION_MISMATCH','TRADE_MISMATCH','SETTLEMENT_MISMATCH')),
  internal_ref      TEXT,
  external_ref      TEXT,
  internal_amount   NUMERIC(28,8),
  external_amount   NUMERIC(28,8),
  status            TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','INVESTIGATING','RESOLVED','WRITTEN_OFF')),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notifications (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID REFERENCES users(id),
  severity      TEXT NOT NULL CHECK (severity IN ('INFO','WARNING','ERROR','CRITICAL')),
  code          TEXT NOT NULL,
  title         TEXT NOT NULL,
  body          TEXT NOT NULL,
  payload       JSONB NOT NULL DEFAULT '{}',
  read_at       TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user ON notifications(user_id, created_at DESC);

CREATE TABLE report_definitions (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code          TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL,
  template      JSONB NOT NULL,
  formats       TEXT[] NOT NULL DEFAULT ARRAY['CSV','XLSX','PDF'],
  status        TEXT NOT NULL DEFAULT 'ACTIVE'
);

CREATE TABLE report_runs (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  definition_id UUID NOT NULL REFERENCES report_definitions(id),
  requested_by  UUID REFERENCES users(id),
  params        JSONB NOT NULL DEFAULT '{}',
  format        TEXT NOT NULL,
  status        TEXT NOT NULL CHECK (status IN ('PENDING','RUNNING','COMPLETED','FAILED')),
  storage_key   TEXT,
  error         TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at   TIMESTAMPTZ
);

CREATE TABLE audit_logs (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  action        TEXT NOT NULL,
  actor_id      UUID,
  actor_type    TEXT NOT NULL DEFAULT 'USER',
  entity_type   TEXT NOT NULL,
  entity_id     UUID,
  request_id    TEXT,
  ip            INET,
  before_json   JSONB,
  after_json    JSONB,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_time ON audit_logs(created_at DESC);

CREATE TRIGGER trg_audit_no_upd BEFORE UPDATE OR DELETE ON audit_logs
FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TABLE api_logs (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id    TEXT NOT NULL,
  user_id       UUID,
  broker_id     UUID,
  token_id      UUID,
  ip            INET,
  method        TEXT NOT NULL,
  endpoint      TEXT NOT NULL,
  status        INT NOT NULL,
  duration_ms   INT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_logs_time ON api_logs(created_at DESC);

CREATE TABLE error_logs (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id    TEXT,
  code          TEXT NOT NULL,
  message       TEXT NOT NULL,
  stack_hash    TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE business_events (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  type          TEXT NOT NULL,
  aggregate_id  UUID,
  payload       JSONB NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE worker_heartbeats (
  worker_name   TEXT PRIMARY KEY,
  status        TEXT NOT NULL CHECK (status IN ('UP','DOWN','DEGRADED')),
  last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  details       JSONB NOT NULL DEFAULT '{}'
);
