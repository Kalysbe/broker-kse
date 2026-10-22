package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
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
	kfbClient.ConfigureRateLimit(rdb, cfg.KFBTradingRPS, cfg.KFBReadRPS)
	// The saved profile (including its token) is loaded after construction.
	// Keep the client registered so a profile loaded later can route orders.
	mgr := adapters.Default(kfbClient)

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
	if err := application.InitRealtimeAdmission(gctx); err != nil {
		log.Fatal("realtime_rate_limits", zap.Error(err))
	}
	if err := application.InitKFBSettings(gctx); err != nil {
		log.Fatal("kfb_settings", zap.Error(err))
	}
	if kfbClient.Configured() && rdb == nil {
		log.Fatal("KFB shared rate limiter requires Redis")
	}
	application.StartGatewayLoop(gctx)

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: application.Router()}
	grpcServer := grpc.NewServer(grpc.MaxRecvMsgSize(64<<10), grpc.MaxConcurrentStreams(128))
	application.RegisterRealtime(grpcServer)
	healthServer := health.NewServer()
	healthServer.SetServingStatus("sbis.realtime.v1.RealtimeService", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	reflection.Register(grpcServer)
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatal("grpc_listen", zap.Error(err))
	}
	go func() {
		log.Info("grpc listening", zap.String("addr", cfg.GRPCAddr))
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatal("grpc", zap.Error(err))
		}
	}()
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
	grpcServer.GracefulStop()
	_ = srv.Shutdown(cctx)
}
