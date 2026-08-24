CREATE TABLE exchanges (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code          TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL,
  timezone      TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','HALTED','CLOSED')),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE trading_calendars (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  exchange_id   UUID NOT NULL REFERENCES exchanges(id),
  weekday       SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),
  open_time     TIME NOT NULL,
  close_time    TIME NOT NULL,
  UNIQUE (exchange_id, weekday)
);

CREATE TABLE issuers (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name                  TEXT NOT NULL,
  short_name            TEXT NOT NULL,
  registration_number   TEXT,
  country               CHAR(2) NOT NULL,
  sector                TEXT,
  status                TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','SUSPENDED','DELISTED')),
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE securities (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  issuer_id         UUID NOT NULL REFERENCES issuers(id),
  isin              TEXT NOT NULL UNIQUE,
  ticker            TEXT NOT NULL,
  issue_number      TEXT,
  name              TEXT NOT NULL,
  type              TEXT NOT NULL CHECK (type IN ('EQUITY','BOND','GOVERNMENT_BOND','ETF','UNIT','OTHER')),
  currency          CHAR(3) NOT NULL REFERENCES currencies(code),
  nominal           NUMERIC(28,8),
  coupon            NUMERIC(12,8),
  coupon_frequency  TEXT,
  issue_date        DATE,
  maturity_date     DATE,
  quantity          NUMERIC(28,8),
  status            TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','SUSPENDED','MATURED','REDEEMED')),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE instruments (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  security_id   UUID NOT NULL REFERENCES securities(id),
  ticker        TEXT NOT NULL,
  isin          TEXT NOT NULL,
  name          TEXT NOT NULL,
  type          TEXT NOT NULL,
  currency      CHAR(3) NOT NULL REFERENCES currencies(code),
  lot_size      NUMERIC(28,8) NOT NULL DEFAULT 1 CHECK (lot_size > 0),
  tick_size     NUMERIC(28,8) NOT NULL DEFAULT 0.01 CHECK (tick_size > 0),
  status        TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','SUSPENDED','HALTED','DELISTED')),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (security_id)
);

CREATE TABLE listings (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  instrument_id   UUID NOT NULL REFERENCES instruments(id),
  exchange_id     UUID NOT NULL REFERENCES exchanges(id),
  venue_symbol    TEXT NOT NULL,
  status          TEXT NOT NULL DEFAULT 'ACTIVE',
  supported_types TEXT[] NOT NULL DEFAULT ARRAY['MARKET','LIMIT'],
  supported_tif   TEXT[] NOT NULL DEFAULT ARRAY['DAY','GTC'],
  UNIQUE (exchange_id, venue_symbol),
  UNIQUE (instrument_id, exchange_id)
);

CREATE TABLE market_snapshots (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  instrument_id   UUID NOT NULL REFERENCES instruments(id),
  exchange_id     UUID NOT NULL REFERENCES exchanges(id),
  last_price      NUMERIC(28,8),
  bid             NUMERIC(28,8),
  ask             NUMERIC(28,8),
  open            NUMERIC(28,8),
  high            NUMERIC(28,8),
  low             NUMERIC(28,8),
  volume          NUMERIC(28,8),
  turnover        NUMERIC(28,8),
  previous_close  NUMERIC(28,8),
  ts              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_snapshots_instr_ts ON market_snapshots(instrument_id, ts DESC);

CREATE TABLE order_book_levels (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  instrument_id   UUID NOT NULL REFERENCES instruments(id),
  exchange_id     UUID NOT NULL REFERENCES exchanges(id),
  side            TEXT NOT NULL CHECK (side IN ('BID','ASK')),
  price           NUMERIC(28,8) NOT NULL,
  quantity        NUMERIC(28,8) NOT NULL,
  ts              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE market_bars (
  instrument_id   UUID NOT NULL REFERENCES instruments(id),
  interval        TEXT NOT NULL CHECK (interval IN ('1m','5m','1h','1d')),
  ts              TIMESTAMPTZ NOT NULL,
  open            NUMERIC(28,8) NOT NULL,
  high            NUMERIC(28,8) NOT NULL,
  low             NUMERIC(28,8) NOT NULL,
  close           NUMERIC(28,8) NOT NULL,
  volume          NUMERIC(28,8) NOT NULL,
  PRIMARY KEY (instrument_id, interval, ts)
);
