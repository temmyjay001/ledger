package server

import (
	"context"
	"log"

	"github.com/temmyjay001/ledger-service/internal/accounts"
	"github.com/temmyjay001/ledger-service/internal/auth"
	"github.com/temmyjay001/ledger-service/internal/config"
	"github.com/temmyjay001/ledger-service/internal/events"
	"github.com/temmyjay001/ledger-service/internal/storage"
	"github.com/temmyjay001/ledger-service/internal/tenant"
	"github.com/temmyjay001/ledger-service/internal/transactions"
	"github.com/temmyjay001/ledger-service/internal/webhooks"
)

type Server struct {
	config              *config.Config
	db                  *storage.DB
	authService         *auth.Service
	authMiddleware      *auth.Middleware
	authHandlers        *auth.Handlers
	tenantService       *tenant.Service
	tenantHandlers      *tenant.Handlers
	accountHandlers     *accounts.Handlers
	transactionHandlers *transactions.Handlers
	eventService        *events.Service
	webhookService      *webhooks.Service
	webhookHandlers     *webhooks.Handlers
}

func New(config *config.Config, db *storage.DB) *Server {
	// Initialize services in dependency order
	authService := auth.NewService(db, config)
	authMiddleware := auth.NewMiddleware(authService)
	authHandlers := auth.NewHandlers(authService)

	tenantService := tenant.NewService(db, authService)
	tenantHandlers := tenant.NewHandlers(tenantService)

	accountService := accounts.NewService(db)
	accountHandlers := accounts.NewHandlers(accountService)

	eventService := events.NewService(db)
	webhookService := webhooks.NewService(db)
	webhookHandlers := webhooks.NewHandlers(webhookService)

	transactionService := transactions.NewService(db, eventService)
	transactionHandlers := transactions.NewHandlers(transactionService)

	return &Server{
		config:              config,
		db:                  db,
		authMiddleware:      authMiddleware,
		authHandlers:        authHandlers,
		tenantHandlers:      tenantHandlers,
		accountHandlers:     accountHandlers,
		transactionHandlers: transactionHandlers,
		eventService:        eventService,
		webhookService:      webhookService,
		webhookHandlers:     webhookHandlers,
	}
}

// NEW: StartWebhookWorker starts the background webhook delivery worker
func (s *Server) StartWebhookWorker(ctx context.Context) {
	log.Println("Starting webhook delivery worker from server...")
	s.webhookService.StartDeliveryWorker(ctx)
}

// NEW: EventWebhookIntegration handles event-to-webhook flow
func (s *Server) setupEventWebhookIntegration() {
	// This could be expanded to set up event listeners
	// For now, the integration happens in the transaction service
	log.Println("Event-webhook integration configured")
}
