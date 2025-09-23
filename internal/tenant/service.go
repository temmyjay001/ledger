package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/temmyjay001/ledger-service/internal/auth"
	"github.com/temmyjay001/ledger-service/internal/storage"
	"github.com/temmyjay001/ledger-service/internal/storage/queries"
)

type Service struct {
	db          *storage.DB
	authService *auth.Service
}

func NewService(db *storage.DB, authService *auth.Service) *Service {
	return &Service{
		db:          db,
		authService: authService,
	}
}

func (s *Service) CreateTenant(ctx context.Context, userID uuid.UUID, req CreateTenantRequest) (*TenantResponse, error) {
	log.Printf("Creating tenant for user %s with request: %+v", userID, req)

	// Validate and sanitize slug
	slug, err := s.validateAndSanitizeSlug(req.Slug)
	if err != nil {
		log.Printf("Slug validation failed: %v", err)
		return nil, err
	}
	log.Printf("Validated slug: %s", slug)

	// Check if slug already exists
	_, err = s.db.Queries.GetTenantBySlug(ctx, slug)
	if err == nil {
		log.Printf("Tenant slug already exists: %s", slug)
		return nil, ErrTenantSlugExists
	}
	log.Printf("Slug is available: %s", slug)

	// Set defaults
	countryCode := pgtype.Text{String: "NG", Valid: true}
	if req.CountryCode != "" {
		countryCode = pgtype.Text{String: req.CountryCode, Valid: true}
	}

	baseCurrency := pgtype.Text{String: "NGN", Valid: true}
	if req.BaseCurrency != "" {
		baseCurrency = pgtype.Text{String: req.BaseCurrency, Valid: true}
	}

	timezone := pgtype.Text{String: "Africa/Lagos", Valid: true}
	if req.Timezone != "" {
		timezone = pgtype.Text{String: req.Timezone, Valid: true}
	}

	businessType := pgtype.Text{}
	if req.BusinessType != "" {
		businessType = pgtype.Text{String: req.BusinessType, Valid: true}
	}

	log.Printf("Prepared tenant data - Name: %s, Slug: %s, Country: %s", req.Name, slug, countryCode.String)

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		log.Printf("Failed to start transaction: %v", err)
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.db.Queries.WithTx(tx)

	// Create tenant
	log.Printf("Creating tenant in database...")
	tenant, err := qtx.CreateTenant(ctx, queries.CreateTenantParams{
		Name:         req.Name,
		Slug:         slug,
		BusinessType: businessType,
		CountryCode:  countryCode,
		BaseCurrency: baseCurrency,
		Timezone:     timezone,
	})
	if err != nil {
		log.Printf("Failed to create tenant in database: %v", err)
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}
	log.Printf("Tenant created successfully with ID: %s", tenant.ID)

	// Add user as admin
	log.Printf("Adding user %s as admin to tenant %s", userID, tenant.ID)
	permissions, _ := json.Marshal(map[string]interface{}{
		"all": true,
	})

	_, err = qtx.AddUserToTenant(ctx, queries.AddUserToTenantParams{
		TenantID:    tenant.ID,
		UserID:      userID,
		Role:        queries.UserRoleEnumAdmin,
		Permissions: permissions,
	})
	if err != nil {
		log.Printf("Failed to add user to tenant: %v", err)
		return nil, fmt.Errorf("failed to add user to tenant: %w", err)
	}
	log.Printf("User added to tenant successfully")

	// Create tenant schema in database
	log.Printf("Creating tenant schema for slug: %s", slug)
	if err := s.CreateTenantSchema(ctx, slug); err != nil {
		log.Printf("Failed to create tenant schema: %v", err)
		return nil, fmt.Errorf("failed to create tenant schema: %w", err)
	}
	log.Printf("Tenant schema created successfully")

	// Commit transaction
	log.Printf("Committing transaction...")
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	log.Printf("Transaction committed successfully")

	log.Printf("Tenant creation completed successfully: %+v", tenant)
	return s.tenantToResponse(tenant), nil
}

func (s *Service) ListUserTenants(ctx context.Context, userID uuid.UUID) ([]*TenantResponse, error) {
	tenants, err := s.db.Queries.ListTenantsByUser(ctx, userID)

	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}

	var response []*TenantResponse
	for _, tenant := range tenants {
		response = append(response, s.tenantToResponse(tenant))
	}

	return response, nil

}

func (s *Service) CreateAPIKey(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, req CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	// check if user  has admin access to this tenant
	tenantUser, err := s.db.Queries.GetTenantUser(ctx, queries.GetTenantUserParams{
		TenantID: tenantID,
		UserID:   userID,
	})
	if err != nil {
		return nil, ErrTenantNotFound
	}

	if tenantUser.Role != queries.UserRoleEnumAdmin && tenantUser.Role != queries.UserRoleEnumDeveloper {
		return nil, ErrInsufficientPermissions
	}

	// validate scopes
	if !auth.ValidateScopes(req.Scopes) {
		return nil, ErrInvalidScopes
	}

	// Create API key
	apiKeyResp, err := s.authService.GenerateAPIKey(ctx, auth.CreateAPIKeyRequest{
		TenantID:  tenantID,
		Name:      req.Name,
		Scopes:    req.Scopes,
		ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	return &CreateAPIKeyResponse{
		ID:        apiKeyResp.ID,
		Name:      apiKeyResp.Name,
		Key:       apiKeyResp.Key, // Only returned once!
		KeyPrefix: apiKeyResp.KeyPrefix,
		Scopes:    apiKeyResp.Scopes,
		ExpiresAt: apiKeyResp.ExpiresAt,
		CreatedAt: apiKeyResp.CreatedAt,
	}, nil
}

// ListAPIKeys returns all API keys for a tenant (without the actual keys)
func (s *Service) ListAPIKeys(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID) ([]*APIKeyListItem, error) {
	// Check if user has access to this tenant
	_, err := s.db.Queries.GetTenantUser(ctx, queries.GetTenantUserParams{
		TenantID: tenantID,
		UserID:   userID,
	})
	if err != nil {
		return nil, ErrTenantNotFound
	}

	// Get API keys
	apiKeys, err := s.db.Queries.ListTenantAPIKeys(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	var response []*APIKeyListItem
	for _, key := range apiKeys {
		var expiresAt *time.Time
		if key.ExpiresAt.Valid {
			expiresAt = &key.ExpiresAt.Time
		}

		var lastUsedAt *time.Time
		if key.LastUsedAt.Valid {
			lastUsedAt = &key.LastUsedAt.Time
		}

		response = append(response, &APIKeyListItem{
			ID:         key.ID,
			Name:       key.Name,
			KeyPrefix:  key.KeyPrefix,
			Scopes:     key.Scopes,
			ExpiresAt:  expiresAt,
			LastUsedAt: lastUsedAt,
			CreatedAt:  key.CreatedAt,
		})
	}

	return response, nil
}

// DeleteAPIKey deletes an API key
func (s *Service) DeleteAPIKey(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, keyID uuid.UUID) error {
	// Check if user has admin access to this tenant
	tenantUser, err := s.db.Queries.GetTenantUser(ctx, queries.GetTenantUserParams{
		TenantID: tenantID,
		UserID:   userID,
	})
	if err != nil {
		return ErrTenantNotFound
	}

	if tenantUser.Role != queries.UserRoleEnumAdmin && tenantUser.Role != queries.UserRoleEnumDeveloper {
		return ErrInsufficientPermissions
	}

	// Delete API key
	err = s.db.Queries.DeleteAPIKey(ctx, queries.DeleteAPIKeyParams{
		ID:       keyID,
		TenantID: tenantID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	return nil
}

// GetTenant returns a specific tenant if user has access
func (s *Service) GetTenant(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID) (*TenantResponse, error) {
	// Check if user has access to this tenant
	_, err := s.db.Queries.GetTenantUser(ctx, queries.GetTenantUserParams{
		TenantID: tenantID,
		UserID:   userID,
	})
	if err != nil {
		return nil, ErrTenantNotFound
	}

	// Get tenant
	tenant, err := s.db.Queries.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, ErrTenantNotFound
	}

	return s.tenantToResponse(tenant), nil
}

func (s *Service) validateAndSanitizeSlug(slug string) (string, error) {
	if slug == "" {
		return "", ErrInvalidSlug
	}

	// convert to lowercase and replace spaces/specials chars with hyphens
	slug = strings.ToLower(slug)
	slug = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if len(slug) < 3 || len(slug) > 50 {
		return "", ErrInvalidSlug
	}

	return slug, nil
}

func (s *Service) CreateTenantSchema(ctx context.Context, tenantSlug string) error {
	// call the database function to create the tenant schema
	_, err := s.db.Exec(ctx, "SELECT create_tenant_schema($1)", tenantSlug)

	return err
}

func (s *Service) tenantToResponse(tenant queries.Tenant) *TenantResponse {
	response := &TenantResponse{
		ID:           tenant.ID,
		Name:         tenant.Name,
		Slug:         tenant.Slug,
		CountryCode:  tenant.CountryCode.String,
		BaseCurrency: tenant.BaseCurrency.String,
		Timezone:     tenant.Timezone.String,
		CreatedAt:    tenant.CreatedAt,
		UpdatedAt:    tenant.UpdatedAt,
	}

	if tenant.BusinessType.Valid {
		response.BusinessType = tenant.BusinessType.String
	}

	return response
}
