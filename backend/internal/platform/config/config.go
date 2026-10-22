package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                    string
	ExchangeReadOnly       bool
	HTTPAddr               string
	GRPCAddr               string
	DatabaseURL            string
	RedisURL               string
	JWTSecret              string
	HMACPepper             string
	ObjectStoreDir         string
	MobileUpdatesDir       string
	AccessTTL              time.Duration
	RefreshTTL             time.Duration
	CORSOrigins            []string
	OTLPEndpoint           string
	SeedAdminLogin         string
	SeedAdminEmail         string
	SeedAdminPass          string
	HMACRequired           bool
	GatewayDevToken        string
	KFBGatewayURL          string
	KFBGatewayToken        string
	KFBTradingRPS          int
	KFBReadRPS             int
	RealtimeLimitMaxStates int
}

func Load() Config {
	accessMin := getenvInt("ACCESS_TOKEN_MINUTES", 15)
	refreshDays := getenvInt("REFRESH_TOKEN_DAYS", 7)
	return Config{
		Env:                    getenv("APP_ENV", "development"),
		ExchangeReadOnly:       getenv("EXCHANGE_READ_ONLY", "false") == "true",
		HTTPAddr:               getenv("HTTP_ADDR", ":18080"),
		GRPCAddr:               getenv("GRPC_ADDR", ":19090"),
		DatabaseURL:            getenv("DATABASE_URL", "postgres://postgres:123@localhost:5432/sbis?sslmode=disable"),
		RedisURL:               getenv("REDIS_URL", ""),
		JWTSecret:              mustEnv("JWT_SECRET"),
		HMACPepper:             getenv("HMAC_PEPPER", "change-me-pepper"),
		ObjectStoreDir:         getenv("OBJECT_STORE_DIR", "./data/objects"),
		MobileUpdatesDir:       getenv("MOBILE_UPDATES_DIR", "./data/mobile-updates"),
		AccessTTL:              time.Duration(accessMin) * time.Minute,
		RefreshTTL:             time.Duration(refreshDays) * 24 * time.Hour,
		CORSOrigins:            strings.Split(getenv("CORS_ORIGINS", "http://localhost:15173,http://localhost:18000"), ","),
		OTLPEndpoint:           getenv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		SeedAdminLogin:         getenv("SEED_ADMIN_LOGIN", "admin"),
		SeedAdminEmail:         getenv("SEED_ADMIN_EMAIL", "admin@broker.local"),
		SeedAdminPass:          getenv("SEED_ADMIN_PASSWORD", ""),
		HMACRequired:           getenv("HMAC_REQUIRED", "false") == "true",
		GatewayDevToken:        getenv("GATEWAY_DEV_TOKEN", "sk_test_devStage1TokenDoNotUseInProd"),
		KFBGatewayURL:          getenv("KFB_GATEWAY_URL", "http://localhost:8080"),
		KFBGatewayToken:        getenv("KFB_GATEWAY_TOKEN", ""),
		KFBTradingRPS:          getenvInt("KFB_TRADING_RPS", 20),
		KFBReadRPS:             getenvInt("KFB_READ_RPS", 100),
		RealtimeLimitMaxStates: getenvInt("REALTIME_LIMIT_MAX_STATES", 100000),
	}
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func getenvInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return d
}

func mustEnv(k string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	if os.Getenv("APP_ENV") == "test" {
		return "test-secret-test-secret-test-secret-32"
	}
	return getenv(k, "dev-only-change-me-in-production-32b")
}
