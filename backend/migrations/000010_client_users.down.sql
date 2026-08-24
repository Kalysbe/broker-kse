DELETE FROM role_permissions
 WHERE role_id IN (SELECT id FROM roles WHERE code = 'CLIENT');
DELETE FROM user_roles
 WHERE role_id IN (SELECT id FROM roles WHERE code = 'CLIENT');
DELETE FROM roles WHERE code = 'CLIENT';

DROP INDEX IF EXISTS idx_users_client;
ALTER TABLE users DROP COLUMN IF EXISTS client_id;
