CREATE TABLE clients (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  client_no         TEXT NOT NULL UNIQUE,
  type              TEXT NOT NULL CHECK (type IN ('INDIVIDUAL','LEGAL','INSTITUTIONAL','GOVERNMENT')),
  status            TEXT NOT NULL CHECK (status IN ('DRAFT','ACTIVE','SUSPENDED','CLOSED')),
  email             CITEXT,
  phone             TEXT,
  country           CHAR(2) NOT NULL DEFAULT 'KG',
  tax_number        TEXT,
  kyc_status        TEXT NOT NULL DEFAULT 'PENDING'
                    CHECK (kyc_status IN ('PENDING','IN_REVIEW','APPROVED','REJECTED','EXPIRED')),
  risk_level        TEXT NOT NULL DEFAULT 'MEDIUM' CHECK (risk_level IN ('LOW','MEDIUM','HIGH')),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by        UUID REFERENCES users(id),
  version           INT NOT NULL DEFAULT 1
);

CREATE TRIGGER trg_clients_updated BEFORE UPDATE ON clients
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE client_individuals (
  client_id         UUID PRIMARY KEY REFERENCES clients(id) ON DELETE CASCADE,
  first_name        TEXT NOT NULL,
  last_name         TEXT NOT NULL,
  middle_name       TEXT,
  birth_date        DATE NOT NULL,
  citizenship       CHAR(2) NOT NULL,
  document_type     TEXT NOT NULL,
  document_number   TEXT NOT NULL,
  document_expires  DATE,
  identifier        TEXT,
  address_line      TEXT NOT NULL,
  city              TEXT,
  postal_code       TEXT
);

CREATE TABLE client_legal_entities (
  client_id             UUID PRIMARY KEY REFERENCES clients(id) ON DELETE CASCADE,
  legal_name            TEXT NOT NULL,
  short_name            TEXT,
  registration_number   TEXT NOT NULL,
  inn                   TEXT,
  legal_address         TEXT NOT NULL,
  director_name         TEXT NOT NULL,
  contact_name          TEXT
);

CREATE TABLE client_beneficiaries (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  client_id     UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
  full_name     TEXT NOT NULL,
  ownership_pct NUMERIC(7,4),
  identifier    TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE client_risk_profiles (
  client_id         UUID PRIMARY KEY REFERENCES clients(id) ON DELETE CASCADE,
  risk_level        TEXT NOT NULL CHECK (risk_level IN ('LOW','MEDIUM','HIGH')),
  rationale         TEXT,
  reviewed_at       TIMESTAMPTZ,
  reviewer_id       UUID REFERENCES users(id)
);

CREATE TABLE client_kyc (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  client_id         UUID NOT NULL REFERENCES clients(id),
  status            TEXT NOT NULL CHECK (status IN ('PENDING','IN_REVIEW','APPROVED','REJECTED','EXPIRED')),
  risk_level        TEXT NOT NULL CHECK (risk_level IN ('LOW','MEDIUM','HIGH')),
  submitted_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  reviewed_at       TIMESTAMPTZ,
  reviewer_id       UUID REFERENCES users(id),
  expires_at        TIMESTAMPTZ,
  notes             TEXT,
  UNIQUE (client_id, submitted_at)
);

CREATE INDEX idx_kyc_client ON client_kyc(client_id, submitted_at DESC);

CREATE TABLE kyc_reviews (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  kyc_id        UUID NOT NULL REFERENCES client_kyc(id),
  reviewer_id   UUID NOT NULL REFERENCES users(id),
  decision      TEXT NOT NULL CHECK (decision IN ('APPROVE','REJECT','REQUEST_INFO')),
  comment       TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (true)
);

CREATE TABLE aml_rule_sets (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code          TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL,
  jurisdiction  TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
  version       INT NOT NULL DEFAULT 1,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE aml_rules (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  rule_set_id   UUID NOT NULL REFERENCES aml_rule_sets(id),
  code          TEXT NOT NULL,
  expression    JSONB NOT NULL,
  severity      TEXT NOT NULL CHECK (severity IN ('INFO','WARNING','BLOCK')),
  status        TEXT NOT NULL DEFAULT 'ACTIVE',
  UNIQUE (rule_set_id, code)
);

CREATE TABLE documents (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  client_id     UUID REFERENCES clients(id),
  type          TEXT NOT NULL CHECK (type IN (
                  'PASSPORT','REGISTRATION','CONTRACT','APPLICATION','QUESTIONNAIRE',
                  'KYC','POWER_OF_ATTORNEY','REPORT','NOTICE','OTHER')),
  status        TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','ARCHIVED','DELETED')),
  storage_key   TEXT NOT NULL,
  content_type  TEXT NOT NULL,
  size_bytes    BIGINT NOT NULL CHECK (size_bytes >= 0),
  version       INT NOT NULL DEFAULT 1,
  uploaded_by   UUID REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_documents_client ON documents(client_id);

CREATE TABLE tariff_plans (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code            TEXT NOT NULL UNIQUE,
  name            TEXT NOT NULL,
  commission_type TEXT NOT NULL CHECK (commission_type IN ('FIXED','PERCENTAGE','TIERED','PER_TRANSACTION','PER_VOLUME')),
  fixed_fee       NUMERIC(28,8) NOT NULL DEFAULT 0,
  percentage      NUMERIC(12,8) NOT NULL DEFAULT 0,
  minimum_fee     NUMERIC(28,8) NOT NULL DEFAULT 0,
  maximum_fee     NUMERIC(28,8),
  market_data_fee NUMERIC(28,8) NOT NULL DEFAULT 0,
  currency        CHAR(3) NOT NULL REFERENCES currencies(code),
  status          TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE commission_tiers (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tariff_id       UUID NOT NULL REFERENCES tariff_plans(id) ON DELETE CASCADE,
  from_amount     NUMERIC(28,8) NOT NULL,
  to_amount       NUMERIC(28,8),
  percentage      NUMERIC(12,8) NOT NULL,
  CHECK (to_amount IS NULL OR to_amount > from_amount)
);

CREATE TABLE contracts (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_number TEXT NOT NULL UNIQUE,
  client_id       UUID NOT NULL REFERENCES clients(id),
  broker_id       UUID NOT NULL REFERENCES brokers(id),
  tariff_plan_id  UUID NOT NULL REFERENCES tariff_plans(id),
  status          TEXT NOT NULL CHECK (status IN ('DRAFT','ACTIVE','SUSPENDED','CLOSED','EXPIRED')),
  signed_on       DATE,
  start_date      DATE NOT NULL,
  end_date        DATE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by      UUID REFERENCES users(id)
);

CREATE TRIGGER trg_contracts_updated BEFORE UPDATE ON contracts
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE trading_accounts (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_number  TEXT NOT NULL UNIQUE,
  client_id       UUID NOT NULL REFERENCES clients(id),
  contract_id     UUID NOT NULL REFERENCES contracts(id),
  broker_id       UUID NOT NULL REFERENCES brokers(id),
  currency        CHAR(3) NOT NULL REFERENCES currencies(code),
  status          TEXT NOT NULL CHECK (status IN ('ACTIVE','BLOCKED','SUSPENDED','CLOSED')),
  overdraft_limit NUMERIC(28,8) NOT NULL DEFAULT 0 CHECK (overdraft_limit >= 0),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  version         INT NOT NULL DEFAULT 1
);

CREATE TRIGGER trg_accounts_updated BEFORE UPDATE ON trading_accounts
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_accounts_client ON trading_accounts(client_id);
