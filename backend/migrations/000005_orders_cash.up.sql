CREATE TABLE cash_accounts (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  trading_account_id    UUID NOT NULL REFERENCES trading_accounts(id),
  currency              CHAR(3) NOT NULL REFERENCES currencies(code),
  bucket                TEXT NOT NULL CHECK (bucket IN ('AVAILABLE','BLOCKED','PENDING','SETTLED','COMMISSION','TAX','CLEARING','NOSTRO')),
  owner_type            TEXT NOT NULL DEFAULT 'CLIENT' CHECK (owner_type IN ('CLIENT','HOUSE')),
  version               INT NOT NULL DEFAULT 1,
  UNIQUE (trading_account_id, currency, bucket)
);

CREATE TABLE ledger_transactions (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  reference_type    TEXT NOT NULL,
  reference_id      UUID NOT NULL,
  description       TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by        UUID REFERENCES users(id),
  idempotency_key   TEXT UNIQUE
);

CREATE TABLE cash_ledger (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  transaction_id        UUID NOT NULL REFERENCES ledger_transactions(id),
  cash_account_id       UUID NOT NULL REFERENCES cash_accounts(id),
  currency              CHAR(3) NOT NULL REFERENCES currencies(code),
  debit                 NUMERIC(28,8) NOT NULL DEFAULT 0 CHECK (debit >= 0),
  credit                NUMERIC(28,8) NOT NULL DEFAULT 0 CHECK (credit >= 0),
  entry_type            TEXT NOT NULL CHECK (entry_type IN (
                          'DEPOSIT','WITHDRAWAL','BUY','SELL','COMMISSION','TAX',
                          'ADJUSTMENT','SETTLEMENT','CORPORATE_ACTION','RESERVE','RELEASE')),
  reference_type        TEXT NOT NULL,
  reference_id          UUID NOT NULL,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((debit > 0 AND credit = 0) OR (credit > 0 AND debit = 0))
);

CREATE INDEX idx_ledger_account_time ON cash_ledger(cash_account_id, created_at);
CREATE INDEX idx_ledger_ref ON cash_ledger(reference_type, reference_id);

CREATE TRIGGER trg_cash_ledger_no_upd BEFORE UPDATE OR DELETE ON cash_ledger
FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TRIGGER trg_ledger_tx_no_upd BEFORE UPDATE OR DELETE ON ledger_transactions
FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TABLE risk_limits (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scope_type      TEXT NOT NULL CHECK (scope_type IN ('CLIENT','ACCOUNT','BROKER','INSTRUMENT','TRADER')),
  scope_id        UUID NOT NULL,
  limit_type      TEXT NOT NULL CHECK (limit_type IN (
                    'MAX_ORDER_QTY','MAX_ORDER_NOTIONAL','MAX_POSITION_QTY',
                    'MAX_GROSS_EXPOSURE','MAX_DAILY_TURNOVER','MAX_OPEN_ORDERS')),
  amount          NUMERIC(28,8) NOT NULL,
  currency        CHAR(3) REFERENCES currencies(code),
  status          TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (scope_type, scope_id, limit_type)
);

CREATE TABLE orders (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  client_order_id     TEXT NOT NULL,
  broker_id           UUID NOT NULL REFERENCES brokers(id),
  client_id           UUID NOT NULL REFERENCES clients(id),
  account_id          UUID NOT NULL REFERENCES trading_accounts(id),
  instrument_id       UUID NOT NULL REFERENCES instruments(id),
  listing_id          UUID REFERENCES listings(id),
  side                TEXT NOT NULL CHECK (side IN ('BUY','SELL')),
  order_type          TEXT NOT NULL CHECK (order_type IN ('MARKET','LIMIT','STOP','STOP_LIMIT')),
  time_in_force       TEXT NOT NULL CHECK (time_in_force IN ('DAY','GTC','IOC','FOK')),
  quantity            NUMERIC(28,8) NOT NULL CHECK (quantity > 0),
  price               NUMERIC(28,8),
  stop_price          NUMERIC(28,8),
  filled_quantity     NUMERIC(28,8) NOT NULL DEFAULT 0 CHECK (filled_quantity >= 0),
  remaining_quantity  NUMERIC(28,8) NOT NULL,
  avg_price           NUMERIC(28,8),
  status              TEXT NOT NULL CHECK (status IN (
                        'NEW','VALIDATING','VALIDATED','PENDING','ROUTING','SENT','ACKNOWLEDGED',
                        'PARTIALLY_FILLED','FILLED','CANCEL_PENDING','CANCELLED','REJECTED','EXPIRED','FAILED')),
  reject_reason       TEXT,
  adapter_id          UUID,
  venue_order_id      TEXT,
  submitted_by        UUID REFERENCES users(id),
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  version             INT NOT NULL DEFAULT 1,
  UNIQUE (broker_id, client_order_id)
);

CREATE INDEX idx_orders_account_status ON orders(account_id, status);
CREATE INDEX idx_orders_created ON orders(created_at DESC);

CREATE TABLE order_events (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id      UUID NOT NULL REFERENCES orders(id),
  from_status   TEXT,
  to_status     TEXT NOT NULL,
  payload       JSONB NOT NULL DEFAULT '{}',
  actor_id      UUID,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_order_events_no_upd BEFORE UPDATE OR DELETE ON order_events
FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TABLE executions (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  exec_id           TEXT NOT NULL,
  order_id          UUID NOT NULL REFERENCES orders(id),
  client_id         UUID NOT NULL REFERENCES clients(id),
  account_id        UUID NOT NULL REFERENCES trading_accounts(id),
  instrument_id     UUID NOT NULL REFERENCES instruments(id),
  side              TEXT NOT NULL CHECK (side IN ('BUY','SELL')),
  quantity          NUMERIC(28,8) NOT NULL CHECK (quantity > 0),
  price             NUMERIC(28,8) NOT NULL CHECK (price > 0),
  gross_amount      NUMERIC(28,8) NOT NULL,
  commission        NUMERIC(28,8) NOT NULL DEFAULT 0,
  net_amount        NUMERIC(28,8) NOT NULL,
  execution_time    TIMESTAMPTZ NOT NULL,
  settlement_date   DATE NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (exec_id)
);

CREATE TRIGGER trg_executions_no_upd BEFORE UPDATE OR DELETE ON executions
FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TABLE trades (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  execution_id      UUID NOT NULL UNIQUE REFERENCES executions(id),
  order_id          UUID NOT NULL REFERENCES orders(id),
  client_id         UUID NOT NULL REFERENCES clients(id),
  account_id        UUID NOT NULL REFERENCES trading_accounts(id),
  instrument_id     UUID NOT NULL REFERENCES instruments(id),
  side              TEXT NOT NULL,
  quantity          NUMERIC(28,8) NOT NULL,
  price             NUMERIC(28,8) NOT NULL,
  gross_amount      NUMERIC(28,8) NOT NULL,
  commission        NUMERIC(28,8) NOT NULL,
  net_amount        NUMERIC(28,8) NOT NULL,
  trade_date        DATE NOT NULL,
  settlement_date   DATE NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_trades_no_upd BEFORE UPDATE OR DELETE ON trades
FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TABLE positions (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id        UUID NOT NULL REFERENCES trading_accounts(id),
  instrument_id     UUID NOT NULL REFERENCES instruments(id),
  quantity          NUMERIC(28,8) NOT NULL DEFAULT 0,
  available_qty     NUMERIC(28,8) NOT NULL DEFAULT 0,
  blocked_qty       NUMERIC(28,8) NOT NULL DEFAULT 0,
  avg_price         NUMERIC(28,8) NOT NULL DEFAULT 0,
  version           INT NOT NULL DEFAULT 1,
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (account_id, instrument_id)
);

CREATE TABLE position_events (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  position_id       UUID NOT NULL REFERENCES positions(id),
  account_id        UUID NOT NULL,
  instrument_id     UUID NOT NULL,
  delta_qty         NUMERIC(28,8) NOT NULL,
  price             NUMERIC(28,8),
  reference_type    TEXT NOT NULL,
  reference_id      UUID NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_position_events_no_upd BEFORE UPDATE OR DELETE ON position_events
FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TABLE commissions (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  trade_id          UUID NOT NULL REFERENCES trades(id),
  tariff_id         UUID NOT NULL REFERENCES tariff_plans(id),
  amount            NUMERIC(28,8) NOT NULL,
  currency          CHAR(3) NOT NULL REFERENCES currencies(code),
  calculation       JSONB NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
