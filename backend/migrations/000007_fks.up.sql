CREATE INDEX IF NOT EXISTS idx_orders_broker_status ON orders(broker_id, status);
ALTER TABLE orders
  ADD CONSTRAINT fk_orders_adapter FOREIGN KEY (adapter_id) REFERENCES adapters(id);

CREATE TABLE IF NOT EXISTS broker_accounts (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  broker_id   UUID NOT NULL REFERENCES brokers(id),
  label       TEXT NOT NULL,
  currency    CHAR(3) NOT NULL REFERENCES currencies(code),
  status      TEXT NOT NULL DEFAULT 'ACTIVE'
);
