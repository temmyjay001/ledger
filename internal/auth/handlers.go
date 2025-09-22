// internal/auth/handlers.go
package auth

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/temmyjay001/ledger-service/pkg/api"
)

type Handlers struct {
	authService *Service
	validator   *validator.Validate
}

func NewHandlers(authService *Service) *Handlers {
	return &Handlers{
		authService: authService,
		validator:   validator.New(),
	}
}

// POST /api/v1/auth/register
func (h *Handlers) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteBadRequestResponse(w, "invalid JSON payload")
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	// Register user
	user, err := h.authService.RegisterUser(r.Context(), req)
	if err != nil {
		switch err {
		case ErrEmailAlreadyExists:
			api.WriteConflictResponse(w, "email already exists")
		default:
			api.WriteInternalErrorResponse(w, "failed to register user")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusCreated, map[string]interface{}{
		"user":    user,
		"message": "User registered successfully. Please verify your email address.",
	})
}

// POST /api/v1/auth/login
func (h *Handlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteBadRequestResponse(w, "invalid JSON payload")
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		api.WriteValidationErrorResponse(w, err)
		return
	}

	// Login user
	loginResp, err := h.authService.LoginUser(r.Context(), req)
	if err != nil {
		switch err {
		case ErrInvalidCredentials:
			api.WriteUnauthorizedResponse(w, "invalid email or password")
		case ErrUserLocked:
			api.WriteUnauthorizedResponse(w, "account is locked due to too many failed attempts")
		default:
			api.WriteInternalErrorResponse(w, "login failed")
		}
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, loginResp)
}

// GET /api/v1/user
func (h *Handlers) GetCurrentUserHandler(w http.ResponseWriter, r *http.Request) {
	// Get user claims from context (set by middleware)
	claims, ok := GetUserClaims(r.Context())
	if !ok {
		api.WriteUnauthorizedResponse(w, "authentication required")
		return
	}

	// Get user from database (to ensure current data)
	user, err := h.authService.db.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		api.WriteInternalErrorResponse(w, "failed to get user")
		return
	}

	api.WriteSuccessResponse(w, http.StatusOK, map[string]interface{}{
		"user": h.authService.userToResponse(user),
	})
}