ALTER TABLE brokers
  ADD COLUMN IF NOT EXISTS quik_firm TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS firm_name TEXT;

ALTER TABLE api_tokens
  ALTER COLUMN rate_limit_rps SET DEFAULT 100,
  ADD COLUMN IF NOT EXISTS order_rate_limit_rps INT NOT NULL DEFAULT 20,
  ADD COLUMN IF NOT EXISTS quik_firm TEXT NOT NULL DEFAULT '';

UPDATE api_tokens SET rate_limit_rps = 100 WHERE rate_limit_rps = 20;
UPDATE api_tokens
SET permissions = ARRAY['orders:create','orders:read','orders:cancel','trades:read','positions:read']
WHERE permissions = '{}' OR permissions IS NULL;

ALTER TABLE trading_accounts
  ADD COLUMN IF NOT EXISTS client_code TEXT,
  ADD COLUMN IF NOT EXISTS trading_account TEXT;

UPDATE trading_accounts
SET trading_account = account_number
WHERE trading_account IS NULL OR trading_account = '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_trading_accounts_broker_code
  ON trading_accounts (broker_id, trading_account);

ALTER TABLE instruments
  ADD COLUMN IF NOT EXISTS internal_symbol TEXT,
  ADD COLUMN IF NOT EXISTS quik_symbol TEXT,
  ADD COLUMN IF NOT EXISTS board TEXT,
  ADD COLUMN IF NOT EXISTS security_type TEXT NOT NULL DEFAULT 'CS',
  ADD COLUMN IF NOT EXISTS public_id BIGSERIAL;

UPDATE instruments
SET internal_symbol = COALESCE(NULLIF(isin, ''), ticker),
    quik_symbol = COALESCE(NULLIF(ticker, ''), isin)
WHERE internal_symbol IS NULL OR quik_symbol IS NULL;

CREATE INDEX IF NOT EXISTS idx_instruments_symbol_board
  ON instruments (quik_symbol, board);

ALTER TABLE listings
  ADD COLUMN IF NOT EXISTS board TEXT;

ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (status IN (
  'QUEUED','NEW','VALIDATING','VALIDATED','PENDING','ROUTING','SENT','ACKNOWLEDGED',
  'PARTIALLY_FILLED','FILLED','CANCEL_PENDING','CANCELLED','REJECTED','EXPIRED','FAILED'
));

ALTER TABLE orders
  ADD COLUMN IF NOT EXISTS client_code TEXT,
  ADD COLUMN IF NOT EXISTS symbol TEXT,
  ADD COLUMN IF NOT EXISTS board TEXT,
  ADD COLUMN IF NOT EXISTS quik_firm TEXT,
  ADD COLUMN IF NOT EXISTS request_id TEXT;

CREATE TABLE IF NOT EXISTS hmac_nonces (
  token_id    UUID NOT NULL REFERENCES api_tokens(id) ON DELETE CASCADE,
  nonce       TEXT NOT NULL,
  seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (token_id, nonce)
);
CREATE INDEX IF NOT EXISTS idx_hmac_nonces_seen ON hmac_nonces (seen_at);

CREATE TABLE IF NOT EXISTS position_snapshots (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  broker_id     UUID NOT NULL REFERENCES brokers(id),
  taken_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  note          TEXT,
  payload       JSONB NOT NULL DEFAULT '[]'
);
CREATE INDEX IF NOT EXISTS idx_position_snapshots_broker ON position_snapshots (broker_id, taken_at DESC);

INSERT INTO currencies (code, name, minor_units) VALUES
  ('KZT', 'Kazakhstani tenge', 2)
ON CONFLICT DO NOTHING;

INSERT INTO permissions (code, description, module) VALUES
  ('orders:create', 'Broker API: create orders', 'broker_api'),
  ('orders:read', 'Broker API: read orders', 'broker_api'),
  ('orders:cancel', 'Broker API: cancel orders', 'broker_api'),
  ('trades:read', 'Broker API: read trades', 'broker_api'),
  ('positions:read', 'Broker API: read positions', 'broker_api')
ON CONFLICT DO NOTHING;

INSERT INTO system_settings (key, value_json) VALUES
  ('gateway.hmac_required', 'false'),
  ('gateway.rate_limit_rps', '100'),
  ('gateway.order_rate_limit_rps', '20')
ON CONFLICT DO NOTHING;
