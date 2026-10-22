INSERT INTO role_permissions (role_id, permission)
SELECT r.id, 'payments.create'
FROM roles r
WHERE r.code = 'CLIENT'
ON CONFLICT DO NOTHING;
