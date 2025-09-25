// internal/transactions/handlers.go
package transactions

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/temmyjay001/ledger-service/pkg/api"
	cV "github.com/temmyjay001/ledger-service/pkg/validator"
)

type Handlers struct {
	service   *Service
	validator *validator.Validate
}

func NewHandlers(service *Service) *Handlers {
	return &Handlers{
		service:   service,
		validator: cV.GetValidator(),
	}
}

// CreateTransactionHandler handles simple transaction creation
func (h *Handlers) CreateTransactionHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")

	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteBadRequestResponse(w, "invalid JSON payload")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	response, err := h.service.CreateSimpleTransaction(r.Context(), tenantSlug, req)
	if err != nil {
		// Handle specific error types
		if err == ErrDuplicateIdempotencyKey {
			api.WriteConflictResponse(w, "Transaction with this idempotency key already exists")
			return
		}
		if err == ErrInvalidAccountCode {
			api.WriteBadRequestResponse(w, "Invalid account code")
			return
		}
		api.WriteInternalErrorResponse(w, err.Error())
		return
	}

	api.WriteSuccessResponse(w, http.StatusCreated, response)
}

// CreateDoubleEntryTransactionHandler handles double-entry transaction creation
func (h *Handlers) CreateDoubleEntryTransactionHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")

	var req CreateDoubleEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteBadRequestResponse(w, "invalid JSON payload")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	response, err := h.service.CreateDoubleEntryTransaction(r.Context(), tenantSlug, req)
	if err != nil {
		// Handle specific error types
		if err == ErrDuplicateIdempotencyKey {
			api.WriteConflictResponse(w, "Transaction with this idempotency key already exists")
			return
		}
		if err == ErrUnbalancedTransaction {
			api.WriteBadRequestResponse(w, "Debits must equal credits for double-entry transactions")
			return
		}
		if err == ErrInvalidCurrency {
			api.WriteBadRequestResponse(w, "All transaction entries must use the same currency")
			return
		}
		if err == ErrInvalidAccountCode {
			api.WriteBadRequestResponse(w, "One or more account codes are invalid")
			return
		}
		api.WriteInternalErrorResponse(w, err.Error())
		return
	}

	api.WriteSuccessResponse(w, http.StatusCreated, response)
}

// GetTransactionHandler retrieves a single transaction
func (h *Handlers) GetTransactionHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	transactionID := chi.URLParam(r, "transactionId")

	id, err := uuid.Parse(transactionID)
	if err != nil {
		api.WriteBadRequestResponse(w, "Invalid transaction ID")
		return
	}

	response, err := h.service.GetTransaction(r.Context(), tenantSlug, id)
	if err != nil {
		if err == ErrTransactionNotFound {
			api.WriteNotFoundResponse(w, "Transaction not found")
			return
		}
		api.WriteInternalErrorResponse(w, err.Error())
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, response)
}

// GetTransactionLinesHandler retrieves transaction lines
func (h *Handlers) GetTransactionLinesHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	transactionID := chi.URLParam(r, "transactionId")

	id, err := uuid.Parse(transactionID)
	if err != nil {
		api.WriteBadRequestResponse(w, "Invalid transaction ID")
		return
	}

	lines, err := h.service.GetTransactionLines(r.Context(), tenantSlug, id)
	if err != nil {
		api.WriteInternalErrorResponse(w, err.Error())
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"transaction_lines": lines,
	})
}

// ListTransactionsHandler retrieves transactions with filtering
func (h *Handlers) ListTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")

	// Parse query parameters
	filters := ListTransactionsRequest{
		Limit:       getIntParam(r, "limit", 50),
		Offset:      getIntParam(r, "offset", 0),
		AccountCode: r.URL.Query().Get("account_code"),
		StartDate:   r.URL.Query().Get("start_date"),
		EndDate:     r.URL.Query().Get("end_date"),
	}

	// Validate limits
	if filters.Limit > 100 {
		filters.Limit = 100
	}
	if filters.Limit <= 0 {
		filters.Limit = 50
	}

	if err := h.validator.Struct(filters); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	response, err := h.service.ListTransactions(r.Context(), tenantSlug, filters)
	if err != nil {
		api.WriteInternalErrorResponse(w, err.Error())
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, response)
}

// Helper function to parse integer parameters
func getIntParam(r *http.Request, key string, defaultValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}
