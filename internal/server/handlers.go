// internal/server/handlers.go
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/temmyjay001/ledger-service/pkg/api"
)

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	})
}

func (s *Server) healthDBHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := s.db.Health(ctx)
	if err != nil {
		api.WriteErrorResponse(w, http.StatusServiceUnavailable, "Database connection failed")
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"database":  "connected",
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) getTransactionReportHandler(w http.ResponseWriter, r *http.Request) {
	api.WriteErrorResponse(w, http.StatusNotImplemented, "Get transaction report endpoint not implemented yet")
}

func (s *Server) getBalanceReportHandler(w http.ResponseWriter, r *http.Request) {
	api.WriteErrorResponse(w, http.StatusNotImplemented, "Get balance report endpoint not implemented yet")
}

func (s *Server) createWebhookHandler(w http.ResponseWriter, r *http.Request) {
	api.WriteErrorResponse(w, http.StatusNotImplemented, "Create webhook endpoint not implemented yet")
}

func (s *Server) listWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	api.WriteErrorResponse(w, http.StatusNotImplemented, "List webhooks endpoint not implemented yet")
}
