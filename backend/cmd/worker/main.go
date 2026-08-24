package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"statebroker/sbis/internal/adapters"
	"statebroker/sbis/internal/adapters/kfb"
	"statebroker/sbis/internal/app"
	"statebroker/sbis/internal/audit"
	"statebroker/sbis/internal/platform/config"
	"statebroker/sbis/internal/platform/db"
	"statebroker/sbis/internal/platform/id"
)

func main() {
	cfg := config.Load()
	log, _ := zap.NewProduction()
	defer log.Sync()
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("db", zap.Error(err))
	}
	defer pool.Close()

	kfbClient := kfb.New(cfg.KFBGatewayURL, cfg.KFBGatewayToken)
	var extra []adapters.ExchangeAdapter
	if kfbClient.Configured() {
		extra = append(extra, kfbClient)
	}
	application := &app.App{
		Cfg:      cfg,
		DB:       pool,
		Log:      log,
		Audit:    audit.Logger{DB: pool},
		Adapters: adapters.Default(extra...),
		KFB:      kfbClient,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	log.Info("worker started")
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			_, _ = pool.Exec(ctx, `
				INSERT INTO worker_heartbeats (worker_name, status, last_seen_at)
				VALUES ('outbox', 'UP', now())
				ON CONFLICT (worker_name) DO UPDATE SET status='UP', last_seen_at=now()`)
			_ = application.ProcessGatewayQueue(ctx)
			rows, err := pool.Query(ctx, `
				SELECT id FROM outbox_events
				WHERE status IN ('PENDING','FAILED') AND available_at <= now()
				ORDER BY created_at LIMIT 50
				FOR UPDATE SKIP LOCKED`)
			if err != nil {
				log.Error("outbox_select", zap.Error(err))
				continue
			}
			var ids []string
			for rows.Next() {
				var idv string
				if rows.Scan(&idv) == nil {
					ids = append(ids, idv)
				}
			}
			rows.Close()
			for _, oid := range ids {
				_, _ = pool.Exec(ctx, `UPDATE outbox_events SET status='SENT', published_at=now() WHERE id=$1`, oid)
			}
			_ = id.New()
			_, _ = pool.Exec(ctx, `
				UPDATE settlements SET cash_status='SETTLED', securities_status='SETTLED', updated_at=now()
				WHERE cash_status='PENDING' AND settlement_date <= CURRENT_DATE`)
			_, _ = pool.Exec(ctx, `DELETE FROM hmac_nonces WHERE seen_at < now() - interval '1 hour'`)
		}
	}
}
