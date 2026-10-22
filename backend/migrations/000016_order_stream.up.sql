ALTER TABLE orders ADD COLUMN venue_details jsonb;
ALTER TABLE orders ADD COLUMN venue_details_loaded_at timestamptz;
CREATE TABLE venue_order_updates (
 id bigserial PRIMARY KEY,
 order_id uuid NOT NULL REFERENCES orders(id),
 received_at timestamptz NOT NULL DEFAULT now(),
 payload jsonb NOT NULL
);
CREATE INDEX venue_order_updates_order ON venue_order_updates(order_id,id);
