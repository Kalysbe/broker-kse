package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"statebroker/sbis/internal/adapters"
	"statebroker/sbis/internal/adapters/kfb"
	"statebroker/sbis/internal/app"
	"statebroker/sbis/internal/audit"
	"statebroker/sbis/internal/outbox"
	"statebroker/sbis/internal/platform/config"
	"statebroker/sbis/internal/platform/db"
)

func main() {
	cfg := config.Load()
	log, _ := zap.NewProduction()
	defer log.Sync()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("db", zap.Error(err))
	}
	defer pool.Close()
	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal("redis_url", zap.Error(err))
	}
	rdb := redis.NewClient(redisOptions)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("redis", zap.Error(err))
	}
	dispatcher := outbox.Dispatcher{
		DB:        pool,
		Publisher: outbox.RedisPublisher{Writer: outbox.RedisStreamWriter{Client: rdb}},
	}

	kfbClient := kfb.New(cfg.KFBGatewayURL, cfg.KFBGatewayToken)
	kfbClient.ConfigureRateLimit(rdb, cfg.KFBTradingRPS, cfg.KFBReadRPS)
	application := &app.App{
		Cfg:      cfg,
		DB:       pool,
		Log:      log,
		Audit:    audit.Logger{DB: pool},
		Adapters: adapters.Default(kfbClient),
		KFB:      kfbClient,
	}

	if err := application.InitKFBSettings(ctx); err != nil {
		log.Fatal("kfb_settings", zap.Error(err))
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	log.Info("worker started")
	for {
		select {
		case <-stop:
			cancel()
			return
		case <-t.C:
			if _, err := pool.Exec(ctx, `
				INSERT INTO worker_heartbeats (worker_name, status, last_seen_at)
				VALUES ('outbox', 'UP', now())
				ON CONFLICT (worker_name) DO UPDATE SET status='UP', last_seen_at=now()`); err != nil {
				log.Error("worker_heartbeat", zap.Error(err))
			}
			if err := application.ProcessGatewayQueue(ctx); err != nil {
				log.Error("gateway_queue", zap.Error(err))
			}
			published, err := dispatcher.Process(ctx)
			if err != nil {
				log.Warn("outbox_dispatch", zap.Int("published", published), zap.Error(err))
			} else if published > 0 {
				log.Info("outbox_published", zap.Int("count", published))
			}
			if _, err := pool.Exec(ctx, `
				UPDATE settlements SET cash_status='SETTLED', securities_status='SETTLED', updated_at=now()
				WHERE cash_status='PENDING' AND settlement_date <= CURRENT_DATE`); err != nil {
				log.Error("settlement_update", zap.Error(err))
			}
			if _, err := pool.Exec(ctx, `DELETE FROM hmac_nonces WHERE seen_at < now() - interval '1 hour'`); err != nil {
				log.Error("nonce_cleanup", zap.Error(err))
			}
		}
	}
}
