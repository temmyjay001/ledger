// internal/auth/types.go
package auth

import (
	"time"

	"github.com/google/uuid"
)

// Registration request
type RegisterRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required,min=1"`
	LastName  string `json:"last_name" validate:"required,min=1"`
}

// Login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Login response
type LoginResponse struct {
	Token     string        `json:"token"`
	ExpiresAt time.Time     `json:"expires_at"`
	User      *UserResponse `json:"user"`
}

// API Key creation request
type CreateAPIKeyRequest struct {
	TenantID  uuid.UUID  `json:"tenant_id" validate:"required"`
	Name      string     `json:"name" validate:"required,min=1,max=100"`
	Scopes    []string   `json:"scopes" validate:"required,min=1"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// API Key creation response
type CreateAPIKeyResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"` // Only returned once during creation!
	KeyPrefix string    `json:"key_prefix"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// API Key list item (without the actual key)
type APIKeyListItem struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Context keys for middleware
type contextKey string

const (
	UserContextKey   contextKey = "user"
	APIKeyContextKey contextKey = "apikey"
)

// Available scopes for API keys
var ValidScopes = []string{
	"transactions:read",
	"transactions:write",
	"accounts:read",
	"accounts:write",
	"balances:read",
	"reports:read",
	"webhooks:manage",
}

// Helper function to validate scopes
func ValidateScopes(scopes []string) bool {
	scopeMap := make(map[string]bool)
	for _, scope := range ValidScopes {
		scopeMap[scope] = true
	}

	for _, scope := range scopes {
		if !scopeMap[scope] {
			return false
		}
	}
	return true
}