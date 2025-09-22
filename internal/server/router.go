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
		r.Post("/auth/register", s.registerHandler)
		r.Post("/auth/login", s.loginHandler)

		// Protected routes
		r.Group(func(r chi.Router) {
			// r.Use(s.authMiddleware)

			// User management
			r.Get("/user", s.getCurrentUserHandler)

			// Tenant management
			r.Post("/tenants", s.createTenantHandler)
			r.Get("/tenants", s.listTenantsHandler)
			r.Get("/tenants/{tenantId}", s.getTenantHandler)

			// API key management
			r.Post("/tenants/{tenantId}/api-keys", s.createAPIKeyHandler)
			r.Get("/tenants/{tenantId}/api-keys", s.listAPIKeysHandler)
			r.Delete("/tenants/{tenantId}/api-keys/{keyId}", s.deleteAPIKeyHandler)
		})

		// Tenant-scoped routes (require API key authentication)
		r.Route("/tenants/{tenantSlug}", func(r chi.Router) {
			// r.Use(s.apiKeyAuthMiddleware) // Will implement this next

			// Account management
			r.Post("/accounts", s.createAccountHandler)
			r.Get("/accounts", s.listAccountsHandler)
			r.Get("/accounts/{accountId}", s.getAccountHandler)
			r.Get("/accounts/{accountId}/balance", s.getAccountBalanceHandler)

			// Transaction management
			r.Post("/transactions", s.createTransactionHandler)
			r.Post("/transactions/double-entry", s.createDoubleEntryTransactionHandler)
			r.Get("/transactions", s.listTransactionsHandler)
			r.Get("/transactions/{transactionId}", s.getTransactionHandler)

			// Reporting
			r.Get("/reports/transactions", s.getTransactionReportHandler)
			r.Get("/reports/balances", s.getBalanceReportHandler)

			// Webhook management
			r.Post("/webhooks", s.createWebhookHandler)
			r.Get("/webhooks", s.listWebhooksHandler)
		})
	})

	return r
}
