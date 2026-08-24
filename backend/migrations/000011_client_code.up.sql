ALTER TABLE clients
  ADD COLUMN IF NOT EXISTS client_code TEXT;

UPDATE clients c
SET client_code = a.client_code
FROM (
  SELECT DISTINCT ON (client_id) client_id, client_code
  FROM trading_accounts
  WHERE client_code IS NOT NULL AND btrim(client_code) <> ''
  ORDER BY client_id, created_at
) a
WHERE c.id = a.client_id
  AND (c.client_code IS NULL OR btrim(c.client_code) = '');

CREATE UNIQUE INDEX IF NOT EXISTS idx_clients_client_code_unique
  ON clients (client_code)
  WHERE client_code IS NOT NULL AND client_code <> '';
