package main

import (
	"context"
	"net/http"
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
	"statebroker/sbis/internal/platform/config"
	"statebroker/sbis/internal/platform/db"
	"statebroker/sbis/internal/platform/storage"
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

	var rdb *redis.Client
	if cfg.RedisURL != "" {
		if opt, err := redis.ParseURL(cfg.RedisURL); err == nil {
			rdb = redis.NewClient(opt)
			if err := rdb.Ping(ctx).Err(); err != nil {
				log.Warn("redis unavailable, continuing without cache", zap.Error(err))
				_ = rdb.Close()
				rdb = nil
			}
		}
	}

	kfbClient := kfb.New(cfg.KFBGatewayURL, cfg.KFBGatewayToken)
	var extra []adapters.ExchangeAdapter
	if kfbClient.Configured() {
		extra = append(extra, kfbClient)
	}
	mgr := adapters.Default(extra...)

	application := &app.App{
		Cfg:      cfg,
		DB:       pool,
		Redis:    rdb,
		Log:      log,
		Audit:    audit.Logger{DB: pool},
		Adapters: mgr,
		KFB:      kfbClient,
		Store:    storage.Filesystem{Root: cfg.ObjectStoreDir},
	}
	if err := application.Bootstrap(ctx); err != nil {
		log.Fatal("bootstrap", zap.Error(err))
	}
	gctx, gcancel := context.WithCancel(ctx)
	defer gcancel()
	application.StartGatewayLoop(gctx)

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: application.Router()}
	go func() {
		log.Info("listening", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("http", zap.Error(err))
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	cctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(cctx)
}
