DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE code = 'CLIENT')
  AND permission = 'payments.create';
