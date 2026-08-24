DROP INDEX IF EXISTS idx_clients_client_code_unique;
ALTER TABLE clients DROP COLUMN IF EXISTS client_code;
