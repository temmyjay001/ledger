package server

import (
	"github.com/temmyjay001/ledger-service/internal/auth"
	"github.com/temmyjay001/ledger-service/internal/config"
	"github.com/temmyjay001/ledger-service/internal/storage"
	"github.com/temmyjay001/ledger-service/internal/tenant"
)

type Server struct {
	config         *config.Config
	db             *storage.DB
	authService    *auth.Service
	authMiddleware *auth.Middleware
	authHandlers   *auth.Handlers
	tenantService  *tenant.Service
	tenantHandlers *tenant.Handlers
}

func New(config *config.Config, db *storage.DB) *Server {
	authService := auth.NewService(db, config)
	authMiddleware := auth.NewMiddleware(authService)
	authHandlers := auth.NewHandlers(authService)
	tenantService := tenant.NewService(db, authService)
	tenantHandlers := tenant.NewHandlers(tenantService)


	return &Server{
		config:         config,
		db:             db,
		authService:    authService,
		authMiddleware: authMiddleware,
		authHandlers:   authHandlers,
		tenantService:  tenantService,
		tenantHandlers: tenantHandlers,
	}
}
