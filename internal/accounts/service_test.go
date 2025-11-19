// internal/accounts/service_test.go
package accounts

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/temmyjay001/ledger-service/internal/storage/queries"
)

// MockQueries is a mock for queries.Queries
type MockQueries struct {
	mock.Mock
}

func (m *MockQueries) CreateAccount(ctx context.Context, arg queries.CreateAccountParams) (queries.Account, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(queries.Account), args.Error(1)
}

func (m *MockQueries) ValidateAccountCode(ctx context.Context, code string) (bool, error) {
	args := m.Called(ctx, code)
	return args.Bool(0), args.Error(1)
}

func (m *MockQueries) GetAccountByCode(ctx context.Context, code string) (queries.Account, error) {
	args := m.Called(ctx, code)
	return args.Get(0).(queries.Account), args.Error(1)
}

func (m *MockQueries) GetAccountByID(ctx context.Context, id uuid.UUID) (queries.Account, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(queries.Account), args.Error(1)
}

func (m *MockQueries) ListAccounts(ctx context.Context) ([]queries.Account, error) {
	args := m.Called(ctx)
	return args.Get(0).([]queries.Account), args.Error(1)
}

func (m *MockQueries) CreateAccountBalance(ctx context.Context, arg queries.CreateAccountBalanceParams) (queries.AccountBalance, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(queries.AccountBalance), args.Error(1)
}

func (m *MockQueries) GetAccountBalance(ctx context.Context, arg queries.GetAccountBalanceParams) (queries.AccountBalance, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(queries.AccountBalance), args.Error(1)
}

// MockDB is a mock for storage.DB
type MockDB struct {
	mock.Mock
	queries *MockQueries
}

func (m *MockDB) SetSearchPath(ctx context.Context, schema string) error {
	args := m.Called(ctx, schema)
	return args.Error(0)
}

func TestValidateAccountCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"Valid numeric code", "1000", false},
		{"Valid alphanumeric code", "CASH-001", false},
		{"Valid with underscore", "ACC_001", false},
		{"Empty code", "", true},
		{"Too long code", "THIS_IS_A_VERY_LONG_ACCOUNT_CODE_THAT_EXCEEDS_TWENTY_CHARACTERS", true},
		{"Invalid characters", "ACC@001", true},
		{"Valid short code", "001", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAccountCode(tt.code)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsValidAccountType(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		want        bool
	}{
		{"Valid asset type", AccountTypeAsset, true},
		{"Valid liability type", AccountTypeLiability, true},
		{"Valid equity type", AccountTypeEquity, true},
		{"Valid revenue type", AccountTypeRevenue, true},
		{"Valid expense type", AccountTypeExpense, true},
		{"Invalid type", "invalid", false},
		{"Empty type", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidAccountType(tt.accountType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidCurrency(t *testing.T) {
	tests := []struct {
		name     string
		currency string
		want     bool
	}{
		{"Valid NGN", "NGN", true},
		{"Valid USD", "USD", true},
		{"Valid EUR", "EUR", true},
		{"Invalid currency", "XXX", false},
		{"Empty currency", "", false},
		{"Lowercase currency", "ngn", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidCurrency(tt.currency)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAccountToResponse(t *testing.T) {
	accountID := uuid.New()
	parentID := uuid.New()
	now := time.Now()

	metadata := map[string]interface{}{
		"description": "Test account",
		"tags":        []string{"test", "sample"},
	}
	metadataBytes, _ := json.Marshal(metadata)

	account := queries.Account{
		ID:          accountID,
		Code:        "1000",
		Name:        "Cash",
		AccountType: queries.AccountTypeEnumAsset,
		ParentID:    &parentID,
		Currency:    "NGN",
		Metadata:    metadataBytes,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	service := &Service{}
	response, err := service.accountToResponse(account, "0000")

	assert.NoError(t, err)
	assert.Equal(t, accountID, response.ID)
	assert.Equal(t, "1000", response.Code)
	assert.Equal(t, "Cash", response.Name)
	assert.Equal(t, "asset", response.AccountType)
	assert.Equal(t, &parentID, response.ParentID)
	assert.Equal(t, "0000", response.ParentCode)
	assert.Equal(t, "NGN", response.Currency)
	assert.True(t, response.IsActive)
	assert.Equal(t, now, response.CreatedAt)
	assert.Equal(t, now, response.UpdatedAt)
	assert.NotNil(t, response.Metadata)
	assert.Equal(t, "Test account", response.Metadata["description"])
}

func TestConvertNumeric(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected decimal.Decimal
	}{
		{
			name:     "Valid decimal",
			input:    decimal.NewFromFloat(123.45),
			expected: decimal.NewFromFloat(123.45),
		},
		{
			name:     "Zero decimal",
			input:    decimal.Zero,
			expected: decimal.Zero,
		},
		{
			name:     "Nil value",
			input:    nil,
			expected: decimal.Zero,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertNumeric(tt.input)
			assert.True(t, result.Equal(tt.expected))
		})
	}
}

func TestGetChartOfAccountsTemplate(t *testing.T) {
	tests := []struct {
		name         string
		businessType string
		wantName     string
		minAccounts  int
	}{
		{
			name:         "Wallet template",
			businessType: "wallet",
			wantName:     "Digital Wallet Chart of Accounts",
			minAccounts:  15,
		},
		{
			name:         "Payments template",
			businessType: "payments",
			wantName:     "Payment Processor Chart of Accounts",
			minAccounts:  5,
		},
		{
			name:         "Lending template",
			businessType: "lending",
			wantName:     "Lending Platform Chart of Accounts",
			minAccounts:  5,
		},
		{
			name:         "Trading template",
			businessType: "trading",
			wantName:     "Trading Platform Chart of Accounts",
			minAccounts:  4,
		},
		{
			name:         "Basic template",
			businessType: "unknown",
			wantName:     "Basic Chart of Accounts",
			minAccounts:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := GetChartOfAccountsTemplate(tt.businessType)
			assert.Equal(t, tt.wantName, template.Name)
			assert.NotEmpty(t, template.Description)
			assert.GreaterOrEqual(t, len(template.Accounts), tt.minAccounts)

			// Verify all accounts have required fields
			for _, account := range template.Accounts {
				assert.NotEmpty(t, account.Code)
				assert.NotEmpty(t, account.Name)
				assert.NotEmpty(t, account.AccountType)
				assert.True(t, IsValidAccountType(account.AccountType))
			}
		})
	}
}

func TestAccountHierarchy(t *testing.T) {
	// Test that wallet template has proper hierarchy
	template := GetChartOfAccountsTemplate("wallet")

	// Build a map of parent codes
	parentCodes := make(map[string]bool)
	for _, account := range template.Accounts {
		if account.ParentCode != "" {
			parentCodes[account.ParentCode] = true
		}
	}

	// Verify that all parent codes exist in the template
	accountCodes := make(map[string]bool)
	for _, account := range template.Accounts {
		accountCodes[account.Code] = true
	}

	for parentCode := range parentCodes {
		assert.True(t, accountCodes[parentCode], 
			"Parent code %s should exist in accounts", parentCode)
	}
}

func BenchmarkValidateAccountCode(b *testing.B) {
	codes := []string{
		"1000",
		"CASH-001",
		"ACC_RECEIVABLE",
		"",
		"INVALID@CODE",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateAccountCode(codes[i%len(codes)])
	}
}

func BenchmarkIsValidAccountType(b *testing.B) {
	types := []string{
		AccountTypeAsset,
		AccountTypeLiability,
		AccountTypeEquity,
		"invalid",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsValidAccountType(types[i%len(types)])
	}
}