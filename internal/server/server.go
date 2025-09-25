package server

import (
	"github.com/temmyjay001/ledger-service/internal/accounts"
	"github.com/temmyjay001/ledger-service/internal/auth"
	"github.com/temmyjay001/ledger-service/internal/config"
	"github.com/temmyjay001/ledger-service/internal/storage"
	"github.com/temmyjay001/ledger-service/internal/tenant"
	"github.com/temmyjay001/ledger-service/internal/transactions"
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
}

func New(config *config.Config, db *storage.DB) *Server {
	authService := auth.NewService(db, config)
	authMiddleware := auth.NewMiddleware(authService)
	authHandlers := auth.NewHandlers(authService)
	tenantService := tenant.NewService(db, authService)
	tenantHandlers := tenant.NewHandlers(tenantService)
	accountService := accounts.NewService(db)
	accountHandlers := accounts.NewHandlers(accountService)
	transactionService := transactions.NewService(db)
	transactionHandlers := transactions.NewHandlers(transactionService)

	return &Server{
		config:              config,
		db:                  db,
		authMiddleware:      authMiddleware,
		authHandlers:        authHandlers,
		tenantHandlers:      tenantHandlers,
		accountHandlers:     accountHandlers,
		transactionHandlers: transactionHandlers,
	}
}
