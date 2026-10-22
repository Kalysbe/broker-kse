DROP INDEX IF EXISTS orders_venue_check;
ALTER TABLE orders DROP COLUMN venue_check_error,DROP COLUMN venue_checked_at,DROP COLUMN venue_status,DROP COLUMN exchange_order_id;
