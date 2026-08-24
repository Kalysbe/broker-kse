package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"statebroker/sbis/internal/gateway"
	"statebroker/sbis/internal/platform/config"
	"statebroker/sbis/internal/platform/db"
	"statebroker/sbis/internal/platform/hash"
	"statebroker/sbis/internal/platform/id"
)

func main() {
	cfg := config.Load()
	pass := cfg.SeedAdminPass
	if pass == "" {
		pass = os.Getenv("SEED_ADMIN_PASSWORD")
	}
	if pass == "" {
		pass = "ChangeMe_Admin12"
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	ph, err := hash.Password(pass)
	if err != nil {
		log.Fatal(err)
	}
	uid := id.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, login, email, first_name, last_name, status, password_hash)
		VALUES ($1,$2,$3,'System','Administrator','ACTIVE',$4)
		ON CONFLICT (login) DO UPDATE SET password_hash=EXCLUDED.password_hash, status='ACTIVE'`,
		uid, cfg.SeedAdminLogin, cfg.SeedAdminEmail, ph)
	if err != nil {
		log.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT u.id, r.id FROM users u CROSS JOIN roles r
		WHERE u.login=$1 AND r.code='SUPER_ADMIN'
		ON CONFLICT DO NOTHING`, cfg.SeedAdminLogin)
	log.Printf("seeded admin user %s", cfg.SeedAdminLogin)

	if err := seedGatewayDemo(ctx, pool, cfg.GatewayDevToken); err != nil {
		log.Fatal(err)
	}
	log.Printf("seeded gateway demo token prefix %s", gateway.TokenPrefix(cfg.GatewayDevToken))
}

func seedGatewayDemo(ctx context.Context, pool *pgxpool.Pool, token string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO brokers (code, name, status, license_no, quik_firm, firm_name)
		VALUES ('DEMO', 'Example Broker', 'ACTIVE', 'EX-0001', 'MC0031400000', 'ТОО Example')
		ON CONFLICT (code) DO UPDATE SET quik_firm=EXCLUDED.quik_firm, firm_name=EXCLUDED.firm_name, status='ACTIVE'`)
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO issuers (id, name, short_name, country, sector, status)
		SELECT '11111111-1111-1111-1111-111111111111', 'Example Issuer', 'EXMPL', 'KZ', 'FINANCIAL', 'ACTIVE'
		WHERE NOT EXISTS (SELECT 1 FROM issuers WHERE id='11111111-1111-1111-1111-111111111111')`)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO securities (id, issuer_id, isin, ticker, name, type, currency, status)
		SELECT '22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111',
		       'KG0105166315', 'KG0105166315', 'Example listed stock', 'EQUITY', 'KZT', 'ACTIVE'
		WHERE NOT EXISTS (SELECT 1 FROM securities WHERE isin='KG0105166315')`)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO instruments (id, security_id, ticker, isin, name, type, currency, lot_size, tick_size, status,
		                         internal_symbol, quik_symbol, board, security_type)
		SELECT '33333333-3333-3333-3333-333333333333', s.id, s.ticker, s.isin, s.name, s.type, s.currency, 1, 0.01, 'ACTIVE',
		       s.isin, s.ticker, 'LISTEDSTOCKK', 'CS'
		FROM securities s WHERE s.isin='KG0105166315'
		ON CONFLICT (security_id) DO UPDATE
		  SET internal_symbol=EXCLUDED.internal_symbol, quik_symbol=EXCLUDED.quik_symbol,
		      board=EXCLUDED.board, security_type=EXCLUDED.security_type, status='ACTIVE'`)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO listings (id, instrument_id, exchange_id, venue_symbol, status, supported_types, supported_tif, board)
		SELECT '44444444-4444-4444-4444-444444444444', i.id, e.id, i.ticker, 'ACTIVE',
		       ARRAY['MARKET','LIMIT'], ARRAY['DAY','IOC','FOK','GTC'], 'LISTEDSTOCKK'
		FROM instruments i CROSS JOIN exchanges e
		WHERE i.isin='KG0105166315' AND e.code='KSE'
		ON CONFLICT (instrument_id, exchange_id) DO UPDATE SET board=EXCLUDED.board, venue_symbol=EXCLUDED.venue_symbol`)
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, client_no, client_code, type, status, email, country, kyc_status, risk_level)
		VALUES ('55555555-5555-5555-5555-555555555555', 'CL-DEMO006', '006', 'LEGAL', 'ACTIVE', 'demo@example.kz', 'KZ', 'APPROVED', 'LOW')
		ON CONFLICT (client_no) DO UPDATE SET status='ACTIVE', kyc_status='APPROVED', client_code='006'`)
	if err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, `
		INSERT INTO client_legal_entities (client_id, legal_name, short_name, registration_number, inn, legal_address, director_name, contact_name)
		VALUES ('55555555-5555-5555-5555-555555555555', 'ТОО Example Client', 'Example', 'REG-006', '006', 'Almaty', 'Director', 'Contact')
		ON CONFLICT (client_id) DO NOTHING`)
	_, err = pool.Exec(ctx, `
		INSERT INTO contracts (id, contract_number, client_id, broker_id, tariff_plan_id, status, start_date)
		SELECT '66666666-6666-6666-6666-666666666666', 'BR-DEMO-006', c.id, b.id, tp.id, 'ACTIVE', CURRENT_DATE
		FROM clients c CROSS JOIN brokers b CROSS JOIN tariff_plans tp
		WHERE c.client_no='CL-DEMO006' AND b.code='DEMO' AND tp.code='STANDARD'
		ON CONFLICT (contract_number) DO NOTHING`)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO trading_accounts (id, account_number, client_id, contract_id, broker_id, currency, status, client_code, trading_account)
		SELECT '77777777-7777-7777-7777-777777777777', '1-3101-006', c.id, ct.id, b.id, 'KZT', 'ACTIVE', '006', '1-3101-006'
		FROM clients c
		JOIN contracts ct ON ct.client_id=c.id
		JOIN brokers b ON b.id=ct.broker_id
		WHERE c.client_no='CL-DEMO006' AND b.code='DEMO'
		ON CONFLICT (account_number) DO UPDATE
		  SET client_code='006', trading_account='1-3101-006', status='ACTIVE', currency='KZT'`)
	if err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, `
		INSERT INTO cash_accounts (id, trading_account_id, currency, bucket, owner_type)
		SELECT gen_random_uuid(), a.id, 'KZT', bkt, 'CLIENT'
		FROM trading_accounts a
		CROSS JOIN (VALUES ('AVAILABLE'),('BLOCKED'),('PENDING'),('SETTLED')) AS buckets(bkt)
		WHERE a.account_number='1-3101-006'
		ON CONFLICT (trading_account_id, currency, bucket) DO NOTHING`)

	if token == "" {
		return nil
	}
	prefix := token
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO api_tokens (id, broker_id, name, token_hash, token_prefix, permissions, rate_limit_rps, order_rate_limit_rps, hmac_required, quik_firm)
		SELECT '88888888-8888-8888-8888-888888888888', b.id, 'stage1-dev', $1, $2,
		       ARRAY['orders:create','orders:read','orders:cancel','trades:read','positions:read'],
		       100, 20, false, 'MC0031400000'
		FROM brokers b WHERE b.code='DEMO'
		ON CONFLICT (id) DO UPDATE SET token_hash=EXCLUDED.token_hash, token_prefix=EXCLUDED.token_prefix,
		  permissions=EXCLUDED.permissions, quik_firm=EXCLUDED.quik_firm, revoked_at=NULL`,
		hash.SHA256Hex(token), prefix)
	return err
}
