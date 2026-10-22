-- Isolated local fixture: synthetic quote, never sent to an exchange.
INSERT INTO market_snapshots (instrument_id, exchange_id, last_price, bid, ask, volume)
SELECT i.id, e.id, 400, 399, 400, 1000
FROM instruments i CROSS JOIN exchanges e
WHERE i.id='33333333-3333-3333-3333-333333333333' AND e.code='KSE'
  AND NOT EXISTS (SELECT 1 FROM market_snapshots s WHERE s.instrument_id=i.id);

-- An unfunded foreign account proves CLIENT access is actually scoped, rather
-- than testing only a missing UUID. It has no linked mobile login.
INSERT INTO clients (id, client_no, type, status, email, country, kyc_status, risk_level)
VALUES ('88888888-8888-8888-8888-888888888888','CL-MOBILE-SCOPE-OTHER','LEGAL','ACTIVE','scope-other@example.invalid','KZ','APPROVED','LOW')
ON CONFLICT (id) DO NOTHING;
INSERT INTO contracts (id, contract_number, client_id, broker_id, tariff_plan_id, status, start_date)
SELECT '99999999-9999-9999-9999-999999999999','BR-MOBILE-SCOPE-OTHER','88888888-8888-8888-8888-888888888888',broker_id,tariff_plan_id,'ACTIVE',CURRENT_DATE
FROM contracts WHERE id='66666666-6666-6666-6666-666666666666'
ON CONFLICT (id) DO NOTHING;
INSERT INTO trading_accounts (id, account_number, client_id, contract_id, broker_id, currency, status)
SELECT 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa','MOBILE-SCOPE-OTHER','88888888-8888-8888-8888-888888888888',id,broker_id,'KZT','ACTIVE'
FROM contracts WHERE id='99999999-9999-9999-9999-999999999999'
ON CONFLICT (id) DO NOTHING;
