// internal/tenant/handlers.go
package tenant

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
	tenantService *Service
	validator     *validator.Validate
}

func NewHandlers(tenantService *Service) *Handlers {
	return &Handlers{
		tenantService: tenantService,
		validator:     validator.New(),
	}
}

// POST /api/v1/tenants
func (h *Handlers) CreateTenantHandler(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "authentication required")
		return
	}

	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteBadRequestResponse(w, "invalid JSON payload")
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	// Create tenant
	tenant, err := h.tenantService.CreateTenant(r.Context(), claims.UserID, req)
	if err != nil {
		switch err {
		case ErrTenantSlugExists:
			api.WriteConflictResponse(w, "tenant slug already exists")
		case ErrInvalidSlug:
			api.WriteBadRequestResponse(w, "invalid slug format")
		default:
			api.WriteInternalErrorResponse(w, "failed to create tenant")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusCreated, map[string]interface{}{
		"tenant": tenant,
	})
}

// GET /api/v1/tenants
func (h *Handlers) ListTenantsHandler(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "authentication required")
		return
	}

	// List tenants
	tenants, err := h.tenantService.ListUserTenants(r.Context(), claims.UserID)
	if err != nil {
		api.WriteInternalErrorResponse(w, "failed to list tenants")
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"tenants": tenants,
	})
}

// GET /api/v1/tenants/{tenantId}
func (h *Handlers) GetTenantHandler(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "authentication required")
		return
	}

	// Parse tenant ID
	tenantIDStr := chi.URLParam(r, "tenantId")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		api.WriteBadRequestResponse(w, "invalid tenant ID")
		return
	}

	// Get tenant
	tenant, err := h.tenantService.GetTenant(r.Context(), claims.UserID, tenantID)
	if err != nil {
		switch err {
		case ErrTenantNotFound:
			api.WriteNotFoundResponse(w, "tenant not found")
		default:
			api.WriteInternalErrorResponse(w, "failed to get tenant")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"tenant": tenant,
	})
}

// POST /api/v1/tenants/{tenantId}/api-keys
func (h *Handlers) CreateAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "authentication required")
		return
	}

	// Parse tenant ID
	tenantIDStr := chi.URLParam(r, "tenantId")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		api.WriteBadRequestResponse(w, "invalid tenant ID")
		return
	}

	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteBadRequestResponse(w, "invalid JSON payload")
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	// Create API key
	apiKey, err := h.tenantService.CreateAPIKey(r.Context(), claims.UserID, tenantID, req)
	if err != nil {
		switch err {
		case ErrTenantNotFound:
			api.WriteNotFoundResponse(w, "tenant not found")
		case ErrInsufficientPermissions:
			api.WriteForbiddenResponse(w, "insufficient permissions")
		case ErrInvalidScopes:
			api.WriteBadRequestResponse(w, "invalid scopes provided")
		case ErrAPIKeyNameExists:
			api.WriteConflictResponse(w, "API key name already exists for this tenant")
		default:
			api.WriteInternalErrorResponse(w, "failed to create API key")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusCreated, map[string]interface{}{
		"api_key": apiKey,
		"warning": "This is the only time the API key will be shown. Please save it securely.",
	})
}

// GET /api/v1/tenants/{tenantId}/api-keys
func (h *Handlers) ListAPIKeysHandler(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "authentication required")
		return
	}

	// Parse tenant ID
	tenantIDStr := chi.URLParam(r, "tenantId")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		api.WriteBadRequestResponse(w, "invalid tenant ID")
		return
	}

	// Parse query parameters
	limit := 50 // default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// List API keys
	apiKeys, err := h.tenantService.ListAPIKeys(r.Context(), claims.UserID, tenantID)
	if err != nil {
		switch err {
		case ErrTenantNotFound:
			api.WriteNotFoundResponse(w, "tenant not found")
		default:
			api.WriteInternalErrorResponse(w, "failed to list API keys")
		}
		return
	}

	// Apply limit
	if len(apiKeys) > limit {
		apiKeys = apiKeys[:limit]
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"api_keys": apiKeys,
		"count":    len(apiKeys),
	})
}

// DELETE /api/v1/tenants/{tenantId}/api-keys/{keyId}
func (h *Handlers) DeleteAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "authentication required")
		return
	}

	// Parse tenant ID
	tenantIDStr := chi.URLParam(r, "tenantId")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		api.WriteBadRequestResponse(w, "invalid tenant ID")
		return
	}

	// Parse key ID
	keyIDStr := chi.URLParam(r, "keyId")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		api.WriteBadRequestResponse(w, "invalid key ID")
		return
	}

	// Delete API key
	err = h.tenantService.DeleteAPIKey(r.Context(), claims.UserID, tenantID, keyID)
	if err != nil {
		switch err {
		case ErrTenantNotFound:
			api.WriteNotFoundResponse(w, "tenant not found")
		case ErrInsufficientPermissions:
			api.WriteForbiddenResponse(w, "insufficient permissions")
		default:
			api.WriteInternalErrorResponse(w, "failed to delete API key")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"message": "API key deleted successfully",
	})
}
