// internal/accounts/service.go
package accounts

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	"github.com/temmyjay001/ledger-service/internal/storage"
	"github.com/temmyjay001/ledger-service/internal/storage/queries"
)

type Service struct {
	db *storage.DB
}

func NewService(db *storage.DB) *Service {
	return &Service{
		db: db,
	}
}

// CreateAccount creates a new account in the tenant's schema
func (s *Service) CreateAccount(ctx context.Context, tenantSlug string, req CreateAccountRequest) (*AccountResponse, error) {
	// Validate input
	if err := ValidateAccountCode(req.Code); err != nil {
		return nil, err
	}

	if !IsValidAccountType(req.AccountType) {
		return nil, ErrInvalidAccountType
	}

	// Set default currency if not provided
	currency := req.Currency
	if currency == "" {
		currency = "NGN" // Default to Naira
	}

	if !IsValidCurrency(currency) {
		return nil, ErrInvalidCurrency
	}

	// Switch to tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	log.Printf("Creating account in tenant schema: tenant_%s", tenantSlug)

	// Check if account code already exists
	exists, err := s.db.Queries.ValidateAccountCode(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to validate account code: %w", err)
	}
	if exists {
		return nil, ErrAccountCodeExists
	}

	// Validate parent account if specified
	var parentID *uuid.UUID
	if req.ParentCode != "" {
		parent, err := s.db.Queries.GetAccountByCode(ctx, req.ParentCode)
		if err != nil {
			return nil, ErrInvalidParentAccount
		}
		parentID = &parent.ID
	}

	// Prepare metadata
	var metadata json.RawMessage
	if req.Metadata != nil {
		metadataBytes, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadata = json.RawMessage(metadataBytes)
	}

	// Create account
	account, err := s.db.Queries.CreateAccount(ctx, queries.CreateAccountParams{
		Code:        req.Code,
		Name:        req.Name,
		AccountType: queries.AccountTypeEnum(req.AccountType),
		ParentID:    parentID,
		Currency:    currency,
		Metadata:    metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	// Initialize balance for the account's default currency
	_, err = s.db.Queries.CreateAccountBalance(ctx, queries.CreateAccountBalanceParams{
		AccountID: account.ID,
		Currency:  currency,
		Balance:   decimal.Zero,
	})
	if err != nil {
		log.Printf("Failed to create initial balance for account %s: %v", account.ID, err)
		// Don't fail account creation if balance creation fails
	}

	log.Printf("Account created successfully: %s (%s)", account.Code, account.Name)
	return s.accountToResponse(account, req.ParentCode)
}

// ListAccounts returns accounts based on filters
func (s *Service) ListAccounts(ctx context.Context, tenantSlug string, req ListAccountsRequest) ([]*AccountResponse, error) {
	// Switch to tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	var accounts []queries.Account
	var err error

	// Apply filters
	if req.AccountType != "" {
		if !IsValidAccountType(req.AccountType) {
			return nil, ErrInvalidAccountType
		}
		accounts, err = s.db.Queries.ListAccountsByType(ctx, queries.AccountTypeEnum(req.AccountType))
	} else if req.ParentCode != "" {
		accounts, err = s.db.Queries.ListAccountsByParentCode(ctx, req.ParentCode)
	} else if req.Search != "" {
		limit := req.Limit
		if limit == 0 {
			limit = 100
		}
		accounts, err = s.db.Queries.SearchAccounts(ctx, queries.SearchAccountsParams{
			Column1: pgtype.Text{String: req.Search, Valid: true},
			Limit:   int32(limit),
		})
	} else {
		accounts, err = s.db.Queries.ListAccounts(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}

	// Convert to response format
	var response []*AccountResponse
	for _, account := range accounts {
		resp, err := s.accountToResponse(account, "")
		if err != nil {
			log.Printf("Failed to convert account to response: %v", err)
			continue
		}
		response = append(response, resp)
	}

	return response, nil
}

// GetAccountByID retrieves a specific account by ID
func (s *Service) GetAccountByID(ctx context.Context, tenantSlug string, accountID uuid.UUID) (*AccountResponse, error) {
	// Switch to tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	account, err := s.db.Queries.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, ErrAccountNotFound
	}

	return s.accountToResponse(account, "")
}

// GetAccountByCode retrieves a specific account by code
func (s *Service) GetAccountByCode(ctx context.Context, tenantSlug string, code string) (*AccountResponse, error) {
	// Switch to tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	account, err := s.db.Queries.GetAccountByCode(ctx, code)
	if err != nil {
		return nil, ErrAccountNotFound
	}

	return s.accountToResponse(account, "")
}

// UpdateAccount updates an existing account
func (s *Service) UpdateAccount(ctx context.Context, tenantSlug string, accountID uuid.UUID, req UpdateAccountRequest) (*AccountResponse, error) {
	// Switch to tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	// Prepare optional fields
	var name string
	if req.Name != "" {
		name = pgtype.Text{String: req.Name, Valid: true}.String
	}

	var metadata json.RawMessage
	if req.Metadata != nil {
		metadataBytes, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadata = json.RawMessage(metadataBytes)
	}

	// Update account
	account, err := s.db.Queries.UpdateAccount(ctx, queries.UpdateAccountParams{
		ID:       accountID,
		Name:     name,
		Metadata: metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update account: %w", err)
	}

	return s.accountToResponse(account, "")
}

// DeactivateAccount soft deletes an account
func (s *Service) DeactivateAccount(ctx context.Context, tenantSlug string, accountID uuid.UUID) error {
	// Switch to tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	// Check if account has children
	children, err := s.db.Queries.ListAccountsByParent(ctx, &accountID)
	if err != nil {
		return fmt.Errorf("failed to check for child accounts: %w", err)
	}
	if len(children) > 0 {
		return ErrAccountHasChildren
	}

	// Check if account has non-zero balances
	balances, err := s.db.Queries.GetAccountBalances(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to check account balances: %w", err)
	}

	for _, balance := range balances {
		if !balance.Balance.IsZero() {
			return ErrAccountHasBalances
		}
	}

	// Deactivate account
	_, err = s.db.Queries.DeactivateAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to deactivate account: %w", err)
	}

	return nil
}

// GetAccountBalance retrieves the balance for a specific account and currency
func (s *Service) GetAccountBalance(ctx context.Context, tenantSlug string, accountID uuid.UUID, currency string) (*AccountBalanceResponse, error) {
	// Switch to tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	balance, err := s.db.Queries.GetAccountBalance(ctx, queries.GetAccountBalanceParams{
		AccountID: accountID,
		Currency:  currency,
	})
	if err != nil {
		// If balance doesn't exist, return zero balance
		return &AccountBalanceResponse{
			Currency: currency,
			Balance:  decimal.Zero,
			Version:  0,
		}, nil
	}

	return &AccountBalanceResponse{
		Currency:  balance.Currency,
		Balance:   balance.Balance,
		Version:   balance.Version,
		UpdatedAt: balance.UpdatedAt, // Now directly assignable
	}, nil
}

// GetAccountBalances retrieves all balances for a specific account
func (s *Service) GetAccountBalances(ctx context.Context, tenantSlug string, accountID uuid.UUID) ([]*AccountBalanceResponse, error) {
	// Switch to tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	balances, err := s.db.Queries.GetAccountBalances(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balances: %w", err)
	}

	var response []*AccountBalanceResponse
	for _, balance := range balances {
		response = append(response, &AccountBalanceResponse{
			Currency:  balance.Currency,
			Balance:   balance.Balance,
			Version:   balance.Version,
			UpdatedAt: balance.UpdatedAt, // Now directly assignable
		})
	}

	return response, nil
}

// GetAccountHierarchy returns the complete chart of accounts hierarchy
func (s *Service) GetAccountHierarchy(ctx context.Context, tenantSlug string) ([]*AccountResponse, error) {
	// Switch to tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	accounts, err := s.db.Queries.GetAccountHierarchy(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get account hierarchy: %w", err)
	}

	var response []*AccountResponse
	for _, account := range accounts {
		resp, err := s.accountToResponseWithHierarchy(account)
		if err != nil {
			log.Printf("Failed to convert account to response: %v", err)
			continue
		}
		response = append(response, resp)
	}

	return response, nil
}

// GetAccountStats returns statistics about the chart of accounts
func (s *Service) GetAccountStats(ctx context.Context, tenantSlug string) (*AccountStatsResponse, error) {
	// Switch to tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	stats, err := s.db.Queries.GetAccountStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get account stats: %w", err)
	}

	return &AccountStatsResponse{
		TotalAccounts:     stats.TotalAccounts,
		AssetAccounts:     stats.AssetAccounts,
		LiabilityAccounts: stats.LiabilityAccounts,
		EquityAccounts:    stats.EquityAccounts,
		RevenueAccounts:   stats.RevenueAccounts,
		ExpenseAccounts:   stats.ExpenseAccounts,
		CurrenciesCount:   stats.CurrenciesCount,
	}, nil
}

// SetupChartOfAccounts creates a default chart of accounts for a tenant
func (s *Service) SetupChartOfAccounts(ctx context.Context, tenantSlug string, businessType string) error {
	template := GetChartOfAccountsTemplate(businessType)

	log.Printf("Setting up chart of accounts for tenant %s with business type %s", tenantSlug, businessType)

	// Create accounts in order (parents first, then children)
	for _, accountReq := range template.Accounts {
		_, err := s.CreateAccount(ctx, tenantSlug, accountReq)
		if err != nil {
			log.Printf("Failed to create account %s (%s): %v", accountReq.Code, accountReq.Name, err)
			return fmt.Errorf("failed to create account %s: %w", accountReq.Code, err)
		}
	}

	log.Printf("Successfully set up chart of accounts for tenant %s", tenantSlug)
	return nil
}

// Helper methods

func (s *Service) accountToResponse(account queries.Account, parentCode string) (*AccountResponse, error) {
	// Parse metadata
	var metadata map[string]interface{}
	if len(account.Metadata) > 0 {
		if err := json.Unmarshal(account.Metadata, &metadata); err != nil {
			log.Printf("Failed to unmarshal account metadata: %v", err)
		}
	}

	return &AccountResponse{
		ID:          account.ID,
		Code:        account.Code,
		Name:        account.Name,
		AccountType: string(account.AccountType),
		ParentID:    account.ParentID, // Now directly assignable
		ParentCode:  parentCode,
		Currency:    account.Currency,
		Metadata:    metadata,
		IsActive:    account.IsActive,  // Now directly assignable
		CreatedAt:   account.CreatedAt, // Now directly assignable
		UpdatedAt:   account.UpdatedAt, // Now directly assignable
	}, nil
}

func (s *Service) accountToResponseWithHierarchy(account queries.GetAccountHierarchyRow) (*AccountResponse, error) {
	// Parse metadata
	var metadata map[string]interface{}
	if len(account.Metadata) > 0 {
		if err := json.Unmarshal(account.Metadata, &metadata); err != nil {
			log.Printf("Failed to unmarshal account metadata: %v", err)
		}
	}

	return &AccountResponse{
		ID:          account.ID,
		Code:        account.Code,
		Name:        account.Name,
		AccountType: string(account.AccountType),
		ParentID:    account.ParentID,
		Currency:    account.Currency,
		Metadata:    metadata,
		Level:       int(account.Level),
		Path:        account.Path,
		IsActive:    account.IsActive,
		CreatedAt:   account.CreatedAt,
		UpdatedAt:   account.UpdatedAt,
	}, nil
}
