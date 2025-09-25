package accounts

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/temmyjay001/ledger-service/internal/auth"
	"github.com/temmyjay001/ledger-service/pkg/api"
)

type Handlers struct {
	accountService *Service
	validator      *validator.Validate
}

func NewHandlers(accountService *Service) *Handlers {
	return &Handlers{
		accountService: accountService,
		validator:      validator.New(),
	}
}

// Post /api/v1/tenants/{tenantSlug}/accounts
func (h *Handlers) CreateAccountHandler(w http.ResponseWriter, r *http.Request) {
	// Get tenant slug from URL
	tenantSlug := chi.URLParam(r, "tenantSlug")

	if tenantSlug == "" {
		api.WriteBadRequestResponse(w, "tenant slug is required")
		return
	}

	// Validate API key claims
	claims, ok := auth.GetAPIKeyClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "API key authentication required")
		return
	}

	// Verify tenant slug matches API key
	if claims.TenantSlug != tenantSlug {
		api.WriteForbiddenResponse(w, "API key not authorized for this tenant")
		return
	}

	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteBadRequestResponse(w, "invalid JSON payload")
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	// Create account
	account, err := h.accountService.CreateAccount(r.Context(), tenantSlug, req)
	if err != nil {
		switch err {
		case ErrAccountCodeExists:
			api.WriteConflictResponse(w, "account code already exists")
		case ErrInvalidAccountCode:
			api.WriteBadRequestResponse(w, "invalid account code format")
		case ErrInvalidAccountType:
			api.WriteBadRequestResponse(w, "invalid account type")
		case ErrInvalidCurrency:
			api.WriteBadRequestResponse(w, "invalid currency code")
		case ErrInvalidParentAccount:
			api.WriteBadRequestResponse(w, "invalid parent account")
		default:
			api.WriteInternalErrorResponse(w, "failed to create account")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusCreated, map[string]interface{}{
		"account": account,
	})
}

// GET /api/v1/tenants/{slug}/accounts
func (h *Handlers) ListAccountsHandler(w http.ResponseWriter, r *http.Request) {
	// Get tenant slug from URL
	tenantSlug := chi.URLParam(r, "tenantSlug")
	if tenantSlug == "" {
		api.WriteBadRequestResponse(w, "tenant slug is required")
		return
	}

	// Validate API key claims
	claims, ok := auth.GetAPIKeyClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "API key authentication required")
		return
	}

	// Verify tenant slug matches API key
	if claims.TenantSlug != tenantSlug {
		api.WriteForbiddenResponse(w, "API key not authorized for this tenant")
		return
	}

	// Parse query parameters
	var req ListAccountsRequest
	req.AccountType = r.URL.Query().Get("account_type")
	req.ParentCode = r.URL.Query().Get("parent_code")
	req.Currency = r.URL.Query().Get("currency")
	req.Search = r.URL.Query().Get("search")

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	// List accounts
	accounts, err := h.accountService.ListAccounts(r.Context(), tenantSlug, req)
	if err != nil {
		switch err {
		case ErrInvalidAccountType:
			api.WriteBadRequestResponse(w, "invalid account type")
		default:
			api.WriteInternalErrorResponse(w, "failed to list accounts")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"accounts": accounts,
		"count":    len(accounts),
	})
}

// GET /api/v1/tenants/{slug}/accounts/{accountId}
func (h *Handlers) GetAccountHandler(w http.ResponseWriter, r *http.Request) {
	// Get tenant slug from URL
	tenantSlug := chi.URLParam(r, "tenantSlug")
	if tenantSlug == "" {
		api.WriteBadRequestResponse(w, "tenant slug is required")
		return
	}

	// Validate API key claims
	claims, ok := auth.GetAPIKeyClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "API key authentication required")
		return
	}

	// Verify tenant slug matches API key
	if claims.TenantSlug != tenantSlug {
		api.WriteForbiddenResponse(w, "API key not authorized for this tenant")
		return
	}

	// Parse account ID
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		api.WriteBadRequestResponse(w, "invalid account ID")
		return
	}

	// Get account
	account, err := h.accountService.GetAccountByID(r.Context(), tenantSlug, accountID)
	if err != nil {
		switch err {
		case ErrAccountNotFound:
			api.WriteNotFoundResponse(w, "account not found")
		default:
			api.WriteInternalErrorResponse(w, "failed to get account")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"account": account,
	})
}

// GET /api/v1/tenants/{slug}/accounts/code/{accountCode}
func (h *Handlers) GetAccountByCodeHandler(w http.ResponseWriter, r *http.Request) {
	// Get tenant slug from URL
	tenantSlug := chi.URLParam(r, "tenantSlug")
	if tenantSlug == "" {
		api.WriteBadRequestResponse(w, "tenant slug is required")
		return
	}

	// Validate API key claims
	claims, ok := auth.GetAPIKeyClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "API key authentication required")
		return
	}

	// Verify tenant slug matches API key
	if claims.TenantSlug != tenantSlug {
		api.WriteForbiddenResponse(w, "API key not authorized for this tenant")
		return
	}

	// Get account code from URL
	accountCode := chi.URLParam(r, "accountCode")
	if accountCode == "" {
		api.WriteBadRequestResponse(w, "account code is required")
		return
	}

	// Get account
	account, err := h.accountService.GetAccountByCode(r.Context(), tenantSlug, accountCode)
	if err != nil {
		switch err {
		case ErrAccountNotFound:
			api.WriteNotFoundResponse(w, "account not found")
		default:
			api.WriteInternalErrorResponse(w, "failed to get account")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"account": account,
	})
}

// PUT /api/v1/tenants/{slug}/accounts/{accountId}
func (h *Handlers) UpdateAccountHandler(w http.ResponseWriter, r *http.Request) {
	// Get tenant slug from URL
	tenantSlug := chi.URLParam(r, "tenantSlug")
	if tenantSlug == "" {
		api.WriteBadRequestResponse(w, "tenant slug is required")
		return
	}

	// Validate API key claims
	claims, ok := auth.GetAPIKeyClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "API key authentication required")
		return
	}

	// Verify tenant slug matches API key
	if claims.TenantSlug != tenantSlug {
		api.WriteForbiddenResponse(w, "API key not authorized for this tenant")
		return
	}

	// Parse account ID
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		api.WriteBadRequestResponse(w, "invalid account ID")
		return
	}

	var req UpdateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteBadRequestResponse(w, "invalid JSON payload")
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	// Update account
	account, err := h.accountService.UpdateAccount(r.Context(), tenantSlug, accountID, req)
	if err != nil {
		switch err {
		case ErrAccountNotFound:
			api.WriteNotFoundResponse(w, "account not found")
		default:
			api.WriteInternalErrorResponse(w, "failed to update account")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"account": account,
	})
}

// DELETE /api/v1/tenants/{slug}/accounts/{accountId}
func (h *Handlers) DeleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	// Get tenant slug from URL
	tenantSlug := chi.URLParam(r, "tenantSlug")
	if tenantSlug == "" {
		api.WriteBadRequestResponse(w, "tenant slug is required")
		return
	}

	// Validate API key claims
	claims, ok := auth.GetAPIKeyClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "API key authentication required")
		return
	}

	// Verify tenant slug matches API key
	if claims.TenantSlug != tenantSlug {
		api.WriteForbiddenResponse(w, "API key not authorized for this tenant")
		return
	}

	// Parse account ID
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		api.WriteBadRequestResponse(w, "invalid account ID")
		return
	}

	// Delete account
	err = h.accountService.DeactivateAccount(r.Context(), tenantSlug, accountID)
	if err != nil {
		switch err {
		case ErrAccountNotFound:
			api.WriteNotFoundResponse(w, "account not found")
		case ErrAccountHasChildren:
			api.WriteConflictResponse(w, "cannot delete account with child accounts")
		case ErrAccountHasBalances:
			api.WriteConflictResponse(w, "cannot delete account with non-zero balances")
		default:
			api.WriteInternalErrorResponse(w, "failed to delete account")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"message": "Account deleted successfully",
	})
}

// GET /api/v1/tenants/{slug}/accounts/{accountId}/balance
func (h *Handlers) GetAccountBalanceHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	accountID := chi.URLParam(r, "accountId")

	id, err := uuid.Parse(accountID)
	if err != nil {
		api.WriteBadRequestResponse(w, "Invalid account ID")
		return
	}

	currency := r.URL.Query().Get("currency")
	if currency == "" {
		currency = "NGN" // Default currency
	}

	balance, err := h.accountService.GetAccountBalance(r.Context(), tenantSlug, id, currency)
	if err != nil {
		if err == ErrAccountNotFound {
			api.WriteNotFoundResponse(w, "Account not found")
			return
		}
		api.WriteInternalErrorResponse(w, err.Error())
		return
	}

	// Enhanced response format as specified in document
	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"currency": balance.Currency,
		"balance":  balance.Balance.String(),
		"version":  balance.Version,
	})
}

// GET /api/v1/tenants/{slug}/accounts/{accountId}/balance/history
func (h *Handlers) GetAccountBalanceHistoryHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	accountID := chi.URLParam(r, "accountId")

	id, err := uuid.Parse(accountID)
	if err != nil {
		api.WriteBadRequestResponse(w, "Invalid account ID")
		return
	}

	days := getIntParam(r, "days", 30)
	currency := r.URL.Query().Get("currency")
	if currency == "" {
		currency = "NGN" // Default currency
	}

	// Validate days parameter
	if days <= 0 || days > 365 {
		days = 30
	}

	history, err := h.accountService.GetAccountBalanceHistory(r.Context(), tenantSlug, id, currency, days)
	if err != nil {
		api.WriteInternalErrorResponse(w, err.Error())
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, history)
}

// GET /api/v1/tenants/{slug}/accounts/{accountId}/balance/summary 
func (h *Handlers) GetBalanceSummaryHandler(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	currency := r.URL.Query().Get("currency")

	summary, err := h.accountService.GetBalanceSummary(r.Context(), tenantSlug, currency)
	if err != nil {
		api.WriteInternalErrorResponse(w, err.Error())
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, summary)
}

// GET /api/v1/tenants/{slug}/accounts/hierarchy
func (h *Handlers) GetAccountHierarchyHandler(w http.ResponseWriter, r *http.Request) {
	// Get tenant slug from URL
	tenantSlug := chi.URLParam(r, "tenantSlug")
	if tenantSlug == "" {
		api.WriteBadRequestResponse(w, "tenant slug is required")
		return
	}

	// Validate API key claims
	claims, ok := auth.GetAPIKeyClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "API key authentication required")
		return
	}

	// Verify tenant slug matches API key
	if claims.TenantSlug != tenantSlug {
		api.WriteForbiddenResponse(w, "API key not authorized for this tenant")
		return
	}

	// Get account hierarchy
	accounts, err := h.accountService.GetAccountHierarchy(r.Context(), tenantSlug)
	if err != nil {
		api.WriteInternalErrorResponse(w, "failed to get account hierarchy")
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"accounts": accounts,
		"count":    len(accounts),
	})
}

// GET /api/v1/tenants/{slug}/accounts/stats
func (h *Handlers) GetAccountStatsHandler(w http.ResponseWriter, r *http.Request) {
	// Get tenant slug from URL
	tenantSlug := chi.URLParam(r, "tenantSlug")
	if tenantSlug == "" {
		api.WriteBadRequestResponse(w, "tenant slug is required")
		return
	}

	// Validate API key claims
	claims, ok := auth.GetAPIKeyClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "API key authentication required")
		return
	}

	// Verify tenant slug matches API key
	if claims.TenantSlug != tenantSlug {
		api.WriteForbiddenResponse(w, "API key not authorized for this tenant")
		return
	}

	// Get account stats
	stats, err := h.accountService.GetAccountStats(r.Context(), tenantSlug)
	if err != nil {
		api.WriteInternalErrorResponse(w, "failed to get account stats")
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"stats": stats,
	})
}

// POST /api/v1/tenants/{slug}/accounts/setup
func (h *Handlers) SetupChartOfAccountsHandler(w http.ResponseWriter, r *http.Request) {
	// Get tenant slug from URL
	tenantSlug := chi.URLParam(r, "tenantSlug")
	if tenantSlug == "" {
		api.WriteBadRequestResponse(w, "tenant slug is required")
		return
	}

	// Validate API key claims
	claims, ok := auth.GetAPIKeyClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "API key authentication required")
		return
	}

	// Verify tenant slug matches API key
	if claims.TenantSlug != tenantSlug {
		api.WriteForbiddenResponse(w, "API key not authorized for this tenant")
		return
	}

	// Parse request
	var req struct {
		BusinessType string `json:"business_type" validate:"required,oneof=wallet payments lending trading basic"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteBadRequestResponse(w, "invalid JSON payload")
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	// Setup chart of accounts
	err := h.accountService.SetupChartOfAccounts(r.Context(), tenantSlug, req.BusinessType)
	if err != nil {
		api.WriteInternalErrorResponse(w, "failed to setup chart of accounts")
		return
	}

	// Get the created accounts to return
	accounts, err := h.accountService.GetAccountHierarchy(r.Context(), tenantSlug)
	if err != nil {
		// Don't fail if we can't get the accounts, setup was successful
		api.WriteSuccessResponse(w, http.StatusCreated, map[string]interface{}{
			"message": "Chart of accounts setup successfully",
		})
		return
	}

	api.WriteSuccessResponse(w, http.StatusCreated, map[string]interface{}{
		"message":  "Chart of accounts setup successfully",
		"accounts": accounts,
		"count":    len(accounts),
	})
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
