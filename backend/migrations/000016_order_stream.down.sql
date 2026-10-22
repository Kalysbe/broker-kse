DROP TABLE venue_order_updates;
ALTER TABLE orders DROP COLUMN venue_details;
ALTER TABLE orders DROP COLUMN venue_details_loaded_at;
