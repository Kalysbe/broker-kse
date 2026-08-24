DELETE FROM permissions WHERE code IN (
  'orders:create','orders:read','orders:cancel','trades:read','positions:read'
);
DELETE FROM system_settings WHERE key IN (
  'gateway.hmac_required','gateway.rate_limit_rps','gateway.order_rate_limit_rps'
);

DROP TABLE IF EXISTS position_snapshots;
DROP TABLE IF EXISTS hmac_nonces;

ALTER TABLE orders
  DROP COLUMN IF EXISTS client_code,
  DROP COLUMN IF EXISTS symbol,
  DROP COLUMN IF EXISTS board,
  DROP COLUMN IF EXISTS quik_firm,
  DROP COLUMN IF EXISTS request_id;

ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (status IN (
  'NEW','VALIDATING','VALIDATED','PENDING','ROUTING','SENT','ACKNOWLEDGED',
  'PARTIALLY_FILLED','FILLED','CANCEL_PENDING','CANCELLED','REJECTED','EXPIRED','FAILED'
));

ALTER TABLE listings DROP COLUMN IF EXISTS board;
DROP INDEX IF EXISTS idx_instruments_symbol_board;
ALTER TABLE instruments
  DROP COLUMN IF EXISTS internal_symbol,
  DROP COLUMN IF EXISTS quik_symbol,
  DROP COLUMN IF EXISTS board,
  DROP COLUMN IF EXISTS security_type,
  DROP COLUMN IF EXISTS public_id;

DROP INDEX IF EXISTS idx_trading_accounts_broker_code;
ALTER TABLE trading_accounts
  DROP COLUMN IF EXISTS client_code,
  DROP COLUMN IF EXISTS trading_account;

ALTER TABLE api_tokens
  DROP COLUMN IF EXISTS order_rate_limit_rps,
  DROP COLUMN IF EXISTS quik_firm;

ALTER TABLE brokers
  DROP COLUMN IF EXISTS quik_firm,
  DROP COLUMN IF EXISTS firm_name;
