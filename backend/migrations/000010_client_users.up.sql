ALTER TABLE users
  ADD COLUMN IF NOT EXISTS client_id UUID REFERENCES clients(id);

CREATE INDEX IF NOT EXISTS idx_users_client
  ON users(client_id)
  WHERE client_id IS NOT NULL AND status <> 'DELETED';

INSERT INTO roles (code, name, description) VALUES
  ('CLIENT', 'Client', 'Self-service trading login')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'accounts.read',
  'orders.read', 'orders.submit', 'orders.cancel',
  'trades.read',
  'instruments.read', 'marketdata.read',
  'cash.read', 'portfolio.read',
  'documents.read', 'reports.run', 'notifications.manage'
) WHERE r.code = 'CLIENT'
ON CONFLICT DO NOTHING;
