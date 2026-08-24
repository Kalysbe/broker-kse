INSERT INTO currencies (code, name, minor_units) VALUES
  ('KGS', 'Kyrgyz som', 2),
  ('USD', 'US dollar', 2),
  ('RUB', 'Russian ruble', 2),
  ('EUR', 'Euro', 2)
ON CONFLICT DO NOTHING;

INSERT INTO departments (code, name) VALUES
  ('EXEC', 'Executive'),
  ('FRONT', 'Front Office'),
  ('BACK', 'Back Office'),
  ('RISK', 'Risk'),
  ('COMP', 'Compliance'),
  ('SETT', 'Settlement'),
  ('IT', 'Information Technology')
ON CONFLICT (code) DO NOTHING;

INSERT INTO organization_policies DEFAULT VALUES;

INSERT INTO system_settings (key, value_json) VALUES
  ('trading.default_currency', '"KGS"'),
  ('trading.session_timezone', '"Asia/Bishkek"'),
  ('orders.allowed_types', '["MARKET","LIMIT","STOP","STOP_LIMIT"]'),
  ('orders.allowed_tif', '["DAY","GTC","IOC","FOK"]'),
  ('risk.default_max_order_notional', '{"KGS": 10000000}'),
  ('retention.api_logs_days', '90'),
  ('rate_limit.default_rps', '50')
ON CONFLICT DO NOTHING;

INSERT INTO permissions (code, description, module) VALUES
  ('users.read','Read users','users'),
  ('users.write','Create and update users','users'),
  ('roles.read','Read roles','roles'),
  ('roles.assign','Assign roles','roles'),
  ('clients.read','Read clients','clients'),
  ('clients.write','Create and update clients','clients'),
  ('kyc.read','Read KYC','kyc'),
  ('kyc.review','Review KYC','kyc'),
  ('documents.read','Read documents','documents'),
  ('documents.write','Upload documents','documents'),
  ('contracts.read','Read contracts','contracts'),
  ('contracts.write','Manage contracts','contracts'),
  ('accounts.read','Read accounts','accounts'),
  ('accounts.write','Manage accounts','accounts'),
  ('orders.read','Read orders','orders'),
  ('orders.submit','Submit orders','orders'),
  ('orders.cancel','Cancel orders','orders'),
  ('trades.read','Read trades','trades'),
  ('marketdata.read','Read market data','marketdata'),
  ('instruments.read','Read instruments','instruments'),
  ('instruments.write','Manage instruments','instruments'),
  ('risk.limits.read','Read limits','risk'),
  ('risk.limits.write','Change limits','risk'),
  ('cash.read','Read cash','cash'),
  ('payments.create','Create payments','payments'),
  ('payments.approve','Approve payments','payments'),
  ('payments.reject','Reject payments','payments'),
  ('portfolio.read','Read portfolio','portfolio'),
  ('settlement.read','Read settlement','settlement'),
  ('settlement.post','Post settlement','settlement'),
  ('tariffs.read','Read tariffs','commissions'),
  ('tariffs.write','Change tariffs','commissions'),
  ('adapters.read','Read adapters','adapters'),
  ('adapters.operate','Operate adapters','adapters'),
  ('brokers.read','Read brokers','brokers'),
  ('brokers.write','Manage brokers','brokers'),
  ('tokens.write','Manage API tokens','brokers'),
  ('reports.run','Run reports','reports'),
  ('reports.export','Export reports','reports'),
  ('admin.settings','Change settings','admin'),
  ('audit.read','Read audit log','audit'),
  ('logs.read','Read technical logs','logging'),
  ('notifications.manage','Manage notifications','notifications'),
  ('reconciliation.run','Run reconciliation','reconciliation')
ON CONFLICT DO NOTHING;

INSERT INTO roles (code, name, description) VALUES
  ('SUPER_ADMIN','Super administrator','Break-glass'),
  ('ADMIN','Administrator','Non-financial administration'),
  ('BROKER_MANAGER','Broker manager','Broker tenant operations'),
  ('BROKER_OPERATOR','Broker operator','Daily operations'),
  ('TRADER','Trader','Order entry'),
  ('RISK_MANAGER','Risk manager','Limits'),
  ('COMPLIANCE_OFFICER','Compliance officer','KYC/AML'),
  ('ACCOUNTANT','Accountant','Cash and commissions'),
  ('SETTLEMENT_OFFICER','Settlement officer','Settlement posting'),
  ('BACK_OFFICE','Back office','Post-trade operations'),
  ('CLIENT_MANAGER','Client manager','CRM'),
  ('AUDITOR','Auditor','Read-only audit'),
  ('VIEWER','Viewer','Read dashboards')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r CROSS JOIN permissions p WHERE r.code = 'SUPER_ADMIN'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'users.read','users.write','roles.read','clients.read','contracts.read','accounts.read',
  'orders.read','trades.read','marketdata.read','instruments.read','instruments.write',
  'adapters.read','adapters.operate','brokers.read','reports.run','reports.export',
  'admin.settings','audit.read','logs.read','notifications.manage'
) WHERE r.code = 'ADMIN'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'clients.read','clients.write','contracts.read','contracts.write','accounts.read','accounts.write',
  'orders.read','trades.read','brokers.read','brokers.write','tokens.write','tariffs.read','tariffs.write',
  'reports.run','reports.export'
) WHERE r.code = 'BROKER_MANAGER'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'clients.read','accounts.read','orders.read','orders.cancel','trades.read','documents.read','documents.write',
  'reports.run','reports.export'
) WHERE r.code = 'BROKER_OPERATOR'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'orders.read','orders.submit','orders.cancel','trades.read','marketdata.read','portfolio.read','cash.read','instruments.read'
) WHERE r.code = 'TRADER'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'orders.read','orders.cancel','risk.limits.read','risk.limits.write','trades.read','portfolio.read','cash.read'
) WHERE r.code = 'RISK_MANAGER'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'clients.read','kyc.read','kyc.review','documents.read','documents.write','audit.read','reports.run'
) WHERE r.code = 'COMPLIANCE_OFFICER'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'cash.read','payments.create','tariffs.read','tariffs.write','reports.run','reports.export','portfolio.read'
) WHERE r.code = 'ACCOUNTANT'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'settlement.read','settlement.post','trades.read','cash.read','reports.run'
) WHERE r.code = 'SETTLEMENT_OFFICER'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'clients.read','contracts.read','contracts.write','accounts.read','accounts.write','documents.read','documents.write',
  'orders.read','orders.cancel','trades.read','cash.read','settlement.read','reports.run','reports.export'
) WHERE r.code = 'BACK_OFFICE'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'clients.read','clients.write','documents.read','documents.write','contracts.read','accounts.read','kyc.read'
) WHERE r.code = 'CLIENT_MANAGER'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'clients.read','accounts.read','orders.read','trades.read','cash.read','portfolio.read',
  'audit.read','logs.read','reports.run','reports.export','settlement.read'
) WHERE r.code = 'AUDITOR'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.code FROM roles r JOIN permissions p ON p.code IN (
  'clients.read','accounts.read','orders.read','trades.read','marketdata.read','portfolio.read','instruments.read','reports.run'
) WHERE r.code = 'VIEWER'
ON CONFLICT DO NOTHING;

INSERT INTO brokers (code, name, status, license_no) VALUES
  ('HOUSE', 'State Broker', 'ACTIVE', 'SB-0001')
ON CONFLICT (code) DO NOTHING;

INSERT INTO tariff_plans (code, name, commission_type, percentage, minimum_fee, maximum_fee, currency, status) VALUES
  ('STANDARD','Standard','TIERED', 0, 50, NULL, 'KGS', 'ACTIVE'),
  ('PREMIUM','Premium','PERCENTAGE', 0.0008, 20, 50000, 'KGS', 'ACTIVE'),
  ('INSTITUTIONAL','Institutional','PERCENTAGE', 0.0005, 0, NULL, 'KGS', 'ACTIVE'),
  ('GOVERNMENT','Government','PERCENTAGE', 0.0003, 0, NULL, 'KGS', 'ACTIVE')
ON CONFLICT (code) DO NOTHING;

INSERT INTO commission_tiers (tariff_id, from_amount, to_amount, percentage)
SELECT id, 0, 1000000, 0.0015 FROM tariff_plans WHERE code = 'STANDARD';
INSERT INTO commission_tiers (tariff_id, from_amount, to_amount, percentage)
SELECT id, 1000000, 10000000, 0.0010 FROM tariff_plans WHERE code = 'STANDARD';
INSERT INTO commission_tiers (tariff_id, from_amount, to_amount, percentage)
SELECT id, 10000000, NULL, 0.0005 FROM tariff_plans WHERE code = 'STANDARD';

INSERT INTO exchanges (code, name, timezone, status) VALUES
  ('KSE', 'Kyrgyz Stock Exchange', 'Asia/Bishkek', 'ACTIVE')
ON CONFLICT (code) DO NOTHING;

INSERT INTO trading_calendars (exchange_id, weekday, open_time, close_time)
SELECT e.id, d, TIME '10:00', TIME '16:00'
FROM exchanges e CROSS JOIN generate_series(1,5) AS d
WHERE e.code = 'KSE'
ON CONFLICT DO NOTHING;

INSERT INTO adapters (code, kind, name, status, weight, config) VALUES
  ('MOCK-EXCHANGE','MOCK','Mock Exchange','ONLINE', 10, '{"fill_mode":"immediate"}'),
  ('FIX-QUIK-1','FIX','QUIK FIX Session 1','OFFLINE', 5, '{"sender":"BROKER","target":"QUIK"}'),
  ('FIX-QUIK-2','FIX','QUIK FIX Session 2','OFFLINE', 5, '{"sender":"BROKER","target":"QUIK"}'),
  ('KSE-GATEWAY','EXCHANGE','KSE Gateway','OFFLINE', 8, '{}'),
  ('BANK-MOCK','BANK','Mock Bank','ONLINE', 1, '{}'),
  ('DEPO-MOCK','DEPOSITORY','Mock Depository','ONLINE', 1, '{}')
ON CONFLICT (code) DO NOTHING;

INSERT INTO aml_rule_sets (code, name, jurisdiction, status) VALUES
  ('KG-BASE','Kyrgyz baseline AML','KG','ACTIVE')
ON CONFLICT (code) DO NOTHING;

INSERT INTO aml_rules (rule_set_id, code, expression, severity, status)
SELECT id, 'HIGH_CASH_DEPOSIT', '{"field":"deposit.amount","op":">=","param":"cash_threshold"}', 'WARNING', 'ACTIVE'
FROM aml_rule_sets WHERE code = 'KG-BASE'
ON CONFLICT DO NOTHING;

INSERT INTO report_definitions (code, name, template, formats) VALUES
  ('DAILY_BROKERAGE','Daily brokerage report','{"source":"trades"}','{CSV,XLSX,PDF}'),
  ('CLIENT_STATEMENT','Client statement','{"source":"ledger_positions"}','{CSV,XLSX,PDF}'),
  ('TRANSACTION_STATEMENT','Transaction statement','{"source":"cash_ledger"}','{CSV,XLSX,PDF}'),
  ('PORTFOLIO_STATEMENT','Portfolio statement','{"source":"positions"}','{CSV,XLSX,PDF}'),
  ('TRADE_REPORT','Trade report','{"source":"trades"}','{CSV,XLSX,PDF}'),
  ('ORDER_REPORT','Order report','{"source":"orders"}','{CSV,XLSX,PDF}'),
  ('COMMISSION_REPORT','Commission report','{"source":"commissions"}','{CSV,XLSX,PDF}'),
  ('CASH_MOVEMENT','Cash movement report','{"source":"cash_ledger"}','{CSV,XLSX,PDF}'),
  ('SECURITIES_MOVEMENT','Securities movement report','{"source":"position_events"}','{CSV,XLSX,PDF}'),
  ('SETTLEMENT_REPORT','Settlement report','{"source":"settlements"}','{CSV,XLSX,PDF}'),
  ('TURNOVER_REPORT','Turnover report','{"source":"trades"}','{CSV,XLSX,PDF}'),
  ('REVENUE_REPORT','Revenue report','{"source":"commissions"}','{CSV,XLSX,PDF}')
ON CONFLICT (code) DO NOTHING;
