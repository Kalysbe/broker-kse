CREATE TABLE currencies (
  code            CHAR(3) PRIMARY KEY,
  name            TEXT NOT NULL,
  minor_units     SMALLINT NOT NULL CHECK (minor_units BETWEEN 0 AND 8),
  status          TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE departments (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code            TEXT NOT NULL UNIQUE,
  name            TEXT NOT NULL,
  status          TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE organization_policies (
  id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  forbid_self_approval        BOOLEAN NOT NULL DEFAULT TRUE,
  withdrawal_dual_control_min NUMERIC(28,8) NOT NULL DEFAULT 0,
  password_min_length         INT NOT NULL DEFAULT 12,
  session_idle_minutes        INT NOT NULL DEFAULT 30,
  access_token_minutes        INT NOT NULL DEFAULT 15,
  refresh_token_days          INT NOT NULL DEFAULT 7,
  retention_audit_days        INT NOT NULL DEFAULT 3650,
  default_timezone            TEXT NOT NULL DEFAULT 'Asia/Bishkek',
  updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE system_settings (
  key           TEXT PRIMARY KEY,
  value_json    JSONB NOT NULL,
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by    UUID
);

CREATE TABLE permissions (
  code          TEXT PRIMARY KEY,
  description   TEXT NOT NULL,
  module        TEXT NOT NULL
);

CREATE TABLE roles (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code          TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL,
  description   TEXT,
  is_system     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
  role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission    TEXT NOT NULL REFERENCES permissions(code) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission)
);

CREATE TABLE users (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  login           CITEXT NOT NULL UNIQUE,
  email           CITEXT NOT NULL UNIQUE,
  phone           TEXT,
  first_name      TEXT NOT NULL,
  last_name       TEXT NOT NULL,
  status          TEXT NOT NULL CHECK (status IN ('ACTIVE','BLOCKED','SUSPENDED','DELETED')),
  department_id   UUID REFERENCES departments(id),
  password_hash   TEXT NOT NULL,
  totp_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
  last_login_at   TIMESTAMPTZ,
  last_login_ip   INET,
  failed_logins   INT NOT NULL DEFAULT 0,
  locked_until    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by      UUID,
  version         INT NOT NULL DEFAULT 1
);

CREATE TRIGGER trg_users_updated BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_roles (
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  assigned_by UUID REFERENCES users(id),
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE totp_secrets (
  user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  secret_enc    BYTEA NOT NULL,
  recovery_hashes TEXT[] NOT NULL DEFAULT '{}',
  confirmed_at  TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES users(id),
  ip            INET,
  user_agent    TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at    TIMESTAMPTZ,
  revoke_reason TEXT
);

CREATE INDEX idx_sessions_user ON sessions(user_id) WHERE revoked_at IS NULL;

CREATE TABLE refresh_tokens (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id    UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  user_id       UUID NOT NULL REFERENCES users(id),
  token_hash    TEXT NOT NULL UNIQUE,
  expires_at    TIMESTAMPTZ NOT NULL,
  revoked_at    TIMESTAMPTZ,
  replaced_by   UUID REFERENCES refresh_tokens(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE password_reset_tokens (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES users(id),
  token_hash    TEXT NOT NULL UNIQUE,
  expires_at    TIMESTAMPTZ NOT NULL,
  used_at       TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE step_up_challenges (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES users(id),
  purpose       TEXT NOT NULL,
  expires_at    TIMESTAMPTZ NOT NULL,
  consumed_at   TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE security_events (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID REFERENCES users(id),
  event_type    TEXT NOT NULL,
  ip            INET,
  user_agent    TEXT,
  payload       JSONB NOT NULL DEFAULT '{}',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_security_events_user_time ON security_events(user_id, created_at DESC);

CREATE TABLE brokers (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code            TEXT NOT NULL UNIQUE,
  name            TEXT NOT NULL,
  status          TEXT NOT NULL CHECK (status IN ('ACTIVE','SUSPENDED','CLOSED')),
  license_no      TEXT,
  country         CHAR(2) NOT NULL DEFAULT 'KG',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE api_tokens (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  broker_id       UUID NOT NULL REFERENCES brokers(id),
  name            TEXT NOT NULL,
  token_hash      TEXT NOT NULL UNIQUE,
  token_prefix    TEXT NOT NULL,
  permissions     TEXT[] NOT NULL DEFAULT '{}',
  ip_whitelist    INET[] NOT NULL DEFAULT '{}',
  rate_limit_rps  INT NOT NULL DEFAULT 20,
  hmac_required   BOOLEAN NOT NULL DEFAULT FALSE,
  hmac_secret_hash TEXT,
  expires_at      TIMESTAMPTZ,
  revoked_at      TIMESTAMPTZ,
  last_used_at    TIMESTAMPTZ,
  created_by      UUID REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_tokens_broker ON api_tokens(broker_id) WHERE revoked_at IS NULL;
