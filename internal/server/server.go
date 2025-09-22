package server

import (
	"github.com/temmyjay001/ledger-service/internal/config"
	"github.com/temmyjay001/ledger-service/internal/storage"
)

type Server struct {
	config *config.Config
	db     *storage.DB
}

func New(config *config.Config, db *storage.DB) *Server {
	return &Server{
		config: config,
		db:     db,
	}
}
