ALTER TABLE orders ADD COLUMN exchange_order_id text,
 ADD COLUMN venue_status text,
 ADD COLUMN venue_checked_at timestamptz,
 ADD COLUMN venue_check_error text;
CREATE INDEX orders_venue_check ON orders(venue_checked_at NULLS FIRST)
 WHERE venue_order_id IS NOT NULL AND status IN ('SENT','ACKNOWLEDGED','PARTIALLY_FILLED','CANCEL_PENDING');
