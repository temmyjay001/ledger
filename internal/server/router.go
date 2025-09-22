package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	// Basic Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	r.Use(s.contentTypeMiddleware)

	r.Get("/health", s.healthHandler)
	r.Get("/health/db", s.healthDBHandler)

	r.Route("/api/v1", func(r chi.Router) {
		// Public auth routes
		r.Post("/auth/register", s.authHandlers.RegisterHandler)
		r.Post("/auth/login", s.authHandlers.LoginHandler)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware.UserAuthMiddleware)

			// User management
			r.Get("/user", s.authHandlers.GetCurrentUserHandler)

			// Tenant management
			r.Post("/tenants", s.tenantHandlers.CreateTenantHandler)
			r.Get("/tenants", s.tenantHandlers.ListTenantsHandler)
			r.Get("/tenants/{tenantId}", s.tenantHandlers.GetTenantHandler)

			// API key management
			r.Post("/tenants/{tenantId}/api-keys", s.tenantHandlers.CreateAPIKeyHandler)
			r.Get("/tenants/{tenantId}/api-keys", s.tenantHandlers.ListAPIKeysHandler)
			r.Delete("/tenants/{tenantId}/api-keys/{keyId}", s.tenantHandlers.DeleteAPIKeyHandler)
		})

		// Tenant-scoped routes (require API key authentication)
		r.Route("/tenants/{tenantSlug}", func(r chi.Router) {
			r.Use(s.authMiddleware.APIKeyAuthMiddleware)

			// Account management
			r.With(s.authMiddleware.RequireScopes("accounts:write")).Post("/accounts", s.createAccountHandler)
			r.With(s.authMiddleware.RequireScopes("accounts:read")).Get("/accounts", s.listAccountsHandler)
			r.With(s.authMiddleware.RequireScopes("accounts:read")).Get("/accounts/{accountId}", s.getAccountHandler)
			r.With(s.authMiddleware.RequireScopes("balances:read")).Get("/accounts/{accountId}/balance", s.getAccountBalanceHandler)

			// Transaction management
			r.With(s.authMiddleware.RequireScopes("transactions:write")).Post("/transactions", s.createTransactionHandler)
			r.With(s.authMiddleware.RequireScopes("transactions:write")).Post("/transactions/double-entry", s.createDoubleEntryTransactionHandler)
			r.With(s.authMiddleware.RequireScopes("transactions:read")).Get("/transactions", s.listTransactionsHandler)
			r.With(s.authMiddleware.RequireScopes("transactions:read")).Get("/transactions/{transactionId}", s.getTransactionHandler)

			// Reporting
			r.With(s.authMiddleware.RequireScopes("reports:read")).Get("/reports/transactions", s.getTransactionReportHandler)
			r.With(s.authMiddleware.RequireScopes("reports:read")).Get("/reports/balances", s.getBalanceReportHandler)

			// Webhook management
			r.With(s.authMiddleware.RequireScopes("webhooks:manage")).Post("/webhooks", s.createWebhookHandler)
			r.With(s.authMiddleware.RequireScopes("webhooks:manage")).Get("/webhooks", s.listWebhooksHandler)
		})
	})

	return r
}
