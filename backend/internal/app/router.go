package app

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"statebroker/sbis/internal/mobileupdates"
	"statebroker/sbis/internal/platform/httpx"
	"statebroker/sbis/internal/platform/telemetry"
)

func (a *App) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(httpx.SecurityHeaders)
	r.Use(a.exchangeReadOnly)
	r.Use(httpx.Recoverer(a.Log))
	r.Use(httpx.AccessLog(a.Log))
	r.Use(telemetry.Middleware)
	r.Use(middleware.RealIP)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/realtime/orders" || r.URL.Path == "/v1/realtime/ws" {
				next.ServeHTTP(w, r)
				return
			}
			middleware.Timeout(60*time.Second)(next).ServeHTTP(w, r)
		})
	})
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   a.Cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-Trace-ID", "Idempotency-Key", "X-Timestamp", "X-Nonce", "X-Signature"},
		ExposedHeaders:   []string{"X-Request-ID", "X-Trace-ID"},
		AllowCredentials: true,
	}))

	r.Get("/health", a.handleHealth)
	r.Get("/health/live", a.handleLive)
	r.Get("/health/ready", a.handleReady)
	r.Get("/ready", a.handleGatewayReady)
	r.Get("/swagger", a.handleSwagger)
	r.Get("/openapi.yaml", a.handleOpenAPI)
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(a.GatewayAuth)
		r.Get("/firm", a.handleGatewayFirm)
		r.Get("/instruments", a.handleGatewayInstruments)
		r.Get("/accounts", a.handleGatewayAccounts)
		r.Post("/orders", a.handleGatewayCreateOrder)
		r.Get("/orders", a.handleGatewayListOrders)
		r.Get("/orders/{id}", a.handleGatewayGetOrder)
		r.Post("/orders/{id}/cancel", a.handleGatewayCancelOrder)
		r.Get("/trades", a.handleGatewayTrades)
		r.Get("/positions", a.handleGatewayPositions)
	})

	r.Route("/v1", func(r chi.Router) {
		updates := mobileupdates.Handler{Dir: a.Cfg.MobileUpdatesDir}
		r.Get("/mobile/updates", updates.Manifest)
		r.Get("/mobile/assets/{key}", updates.Asset)
		r.Get("/realtime/ws", a.handleWebSocket)
		r.Post("/auth/login", a.handleLogin)
		r.Post("/auth/refresh", a.handleRefresh)
		r.Post("/auth/password-reset", a.handlePasswordResetRequest)
		r.Post("/auth/password-reset/confirm", a.handlePasswordResetConfirm)

		r.Group(func(r chi.Router) {
			r.Use(a.Authenticate)
			r.Get("/realtime/rate-limits", a.handleRealtimeLimits)
			r.Put("/realtime/rate-limits", a.handleRealtimeLimits)
			r.With(a.perm("orders.read")).Get("/realtime/orders", a.handleOrderStream)
			r.Post("/auth/logout", a.handleLogout)
			r.Get("/auth/me", a.handleMe)
			r.Post("/auth/change-password", a.handleChangePassword)
			r.Get("/dashboard", a.handleDashboard)
			r.With(a.perm("users.read")).Get("/users", a.handleListUsers)
			r.With(a.perm("users.read")).Get("/users/{id}", a.handleGetUser)
			r.With(a.perm("users.write")).Post("/users", a.handleCreateUser)
			r.With(a.perm("users.write")).Patch("/users/{id}", a.handleUpdateUser)
			r.With(a.perm("users.write")).Delete("/users/{id}", a.handleDeleteUser)
			r.With(a.perm("roles.read")).Get("/roles", a.handleRoles)

			r.With(a.perm("clients.read")).Get("/clients", a.handleListClients)
			r.With(a.perm("clients.read")).Get("/clients/{id}", a.handleGetClient)
			r.With(a.perm("clients.write")).Post("/clients", a.handleCreateClient)
			r.With(a.perm("clients.write")).Patch("/clients/{id}", a.handleUpdateClient)
			r.With(a.perm("clients.write")).Delete("/clients/{id}", a.handleDeleteClient)
			r.With(a.perm("kyc.review")).Post("/clients/{id}/kyc/review", a.handleKYCReview)

			r.With(a.perm("contracts.read")).Get("/contracts", a.handleListContracts)
			r.With(a.perm("contracts.write")).Post("/contracts", a.handleCreateContract)
			r.With(a.perm("contracts.write")).Patch("/contracts/{id}", a.handleUpdateContract)

			r.With(a.perm("accounts.read")).Get("/accounts", a.handleListAccounts)
			r.With(a.perm("accounts.write")).Post("/accounts", a.handleCreateAccount)
			r.With(a.perm("accounts.write")).Patch("/accounts/{id}", a.handleUpdateAccount)
			r.With(a.perm("cash.read")).Get("/accounts/{id}/cash", a.handleCash)
			r.With(a.perm("cash.read")).Get("/accounts/{id}/ledger", a.handleLedger)
			r.With(a.perm("portfolio.read")).Get("/accounts/{id}/portfolio", a.handlePortfolio)
			r.With(a.perm("portfolio.read")).Get("/accounts/{id}/exchange-portfolio", a.handleExchangePortfolio)
			r.With(a.perm("portfolio.read")).Get("/positions", a.handleListPositions)

			r.With(a.perm("instruments.read")).Get("/instruments", a.handleListInstruments)
			r.With(a.perm("instruments.write")).Post("/instruments", a.handleCreateInstrument)
			r.With(a.perm("instruments.read")).Get("/issuers", a.handleIssuers)
			r.With(a.perm("marketdata.read")).Get("/market", a.handleMarket)
			r.With(a.perm("marketdata.read")).Get("/market/{id}/book", a.handleMarketBook)
			r.With(a.perm("marketdata.read")).Get("/market-data/{id}", a.handleMarketSnapshot)

			r.With(a.perm("orders.read")).Get("/orders", a.handleListOrders)
			r.With(a.perm("orders.read")).Get("/orders/{id}", a.handleGetOrder)
			r.With(a.perm("orders.submit")).Post("/orders", a.handleCreateOrder)
			r.With(a.perm("orders.cancel")).Post("/orders/{id}/cancel", a.handleCancelOrder)

			r.With(a.perm("trades.read")).Get("/trades", a.handleListTrades)
			r.With(a.perm("settlement.read")).Get("/settlements", a.handleListSettlements)
			r.With(a.perm("settlement.post")).Post("/settlements/{id}/post", a.handlePostSettlement)

			r.With(a.perm("payments.create")).Post("/deposits", a.handleDeposit)
			r.With(a.perm("payments.create")).Post("/deposits/card/mock", a.handleMockCardDeposit)
			r.With(a.perm("payments.create")).Post("/withdrawals", a.handleWithdrawal)
			r.With(a.perm("cash.read")).Get("/deposits", a.handleListDeposits)
			r.With(a.perm("cash.read")).Get("/withdrawals", a.handleListWithdrawals)
			r.With(a.perm("payments.approve")).Post("/withdrawals/{id}/approve", a.handleApproveWithdrawal)
			r.With(a.perm("payments.approve")).Post("/deposits/{id}/approve", a.handleApproveDeposit)

			r.With(a.perm("tariffs.read")).Get("/tariffs", a.handleTariffs)
			r.With(a.perm("adapters.read")).Get("/adapters", a.handleAdapters)
			r.Get("/integrations/kfb/settings", a.handleKFBSettings)
			r.Put("/integrations/kfb/settings", a.handleKFBSettings)
			r.Get("/integrations/kfb/health", a.handleKFBHealth)
			r.With(a.perm("adapters.read")).Get("/integrations/kfb", a.handleKFBStatus)
			r.With(a.perm("adapters.operate")).Post("/integrations/kfb/connect", a.handleKFBStatus)
			r.With(a.perm("adapters.read")).Get("/integrations/depository", a.handleDepositoryStatus)
			r.With(a.perm("brokers.read")).Get("/brokers", a.handleListBrokers)
			r.With(a.perm("tokens.write")).Post("/tokens", a.handleCreateToken)

			r.With(a.perm("reports.run")).Get("/reports", a.handleListReports)
			r.With(a.perm("reports.export")).Get("/reports/export", a.handleRunReport)
			r.With(a.perm("audit.read")).Get("/audit", a.handleAudit)
			r.With(a.perm("notifications.manage")).Get("/notifications", a.handleNotifications)
			r.Get("/notifications", a.handleNotifications)
			r.With(a.perm("adapters.operate")).Post("/load-tests", a.handleStartLoadTest)
			r.With(a.perm("adapters.operate")).Get("/load-tests/current", a.handleCurrentLoadTest)
			r.With(a.perm("adapters.operate")).Get("/load-tests/current/requests", a.handleLoadTestRequests)
			r.With(a.perm("adapters.operate")).Get("/load-tests/active", a.handleActiveLoadOrders)
			r.With(a.perm("adapters.operate")).Get("/load-tests/orders/{id}", a.handleLoadOrderTracking)
			r.With(a.perm("adapters.operate")).Post("/load-tests/current/cancel", a.handleCancelLoadTest)
			r.With(a.perm("reconciliation.run")).Post("/reconciliation/jobs", a.handleReconciliationRun)
			r.With(a.perm("instruments.read")).Get("/corporate-actions", a.handleCorporateActions)
		})
	})
	return r
}
