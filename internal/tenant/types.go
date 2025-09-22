// internal/tenant/types.go
package tenant

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors
var (
	ErrTenantNotFound           = errors.New("tenant not found")
	ErrTenantSlugExists         = errors.New("tenant slug already exists")
	ErrInvalidSlug              = errors.New("invalid slug: must be 3-50 characters, lowercase letters, numbers, and hyphens only")
	ErrInsufficientPermissions  = errors.New("insufficient permissions")
	ErrInvalidScopes            = errors.New("invalid scopes provided")
)

// Request types

type CreateTenantRequest struct {
	Name         string `json:"name" validate:"required,min=1,max=100"`
	Slug         string `json:"slug" validate:"required,min=3,max=50"`
	BusinessType string `json:"business_type,omitempty" validate:"omitempty,oneof=wallet lending remittance payments trading crypto other"`
	CountryCode  string `json:"country_code,omitempty" validate:"omitempty,len=2"`
	BaseCurrency string `json:"base_currency,omitempty" validate:"omitempty,len=3"`
	Timezone     string `json:"timezone,omitempty"`
}

type CreateAPIKeyRequest struct {
	Name      string     `json:"name" validate:"required,min=1,max=100"`
	Scopes    []string   `json:"scopes" validate:"required,min=1"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Response types

type TenantResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	BusinessType string    `json:"business_type,omitempty"`
	CountryCode  string    `json:"country_code"`
	BaseCurrency string    `json:"base_currency"`
	Timezone     string    `json:"timezone"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateAPIKeyResponse struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Key       string     `json:"key"` // Only returned once during creation!
	KeyPrefix string     `json:"key_prefix"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type APIKeyListItem struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}