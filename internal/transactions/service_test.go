// internal/transactions/service_test.go
package transactions

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/temmyjay001/ledger-service/internal/storage/queries"
)

func TestCalculateNewBalance(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name           string
		currentBalance decimal.Decimal
		amount         decimal.Decimal
		side           string
		accountType    queries.AccountTypeEnum
		expected       decimal.Decimal
	}{
		// Asset account tests
		{
			name:           "Asset - Debit increases balance",
			currentBalance: decimal.NewFromInt(1000),
			amount:         decimal.NewFromInt(500),
			side:           "debit",
			accountType:    queries.AccountTypeEnumAsset,
			expected:       decimal.NewFromInt(1500),
		},
		{
			name:           "Asset - Credit decreases balance",
			currentBalance: decimal.NewFromInt(1000),
			amount:         decimal.NewFromInt(500),
			side:           "credit",
			accountType:    queries.AccountTypeEnumAsset,
			expected:       decimal.NewFromInt(500),
		},
		// Liability account tests
		{
			name:           "Liability - Credit increases balance",
			currentBalance: decimal.NewFromInt(1000),
			amount:         decimal.NewFromInt(500),
			side:           "credit",
			accountType:    queries.AccountTypeEnumLiability,
			expected:       decimal.NewFromInt(1500),
		},
		{
			name:           "Liability - Debit decreases balance",
			currentBalance: decimal.NewFromInt(1000),
			amount:         decimal.NewFromInt(500),
			side:           "debit",
			accountType:    queries.AccountTypeEnumLiability,
			expected:       decimal.NewFromInt(500),
		},
		// Equity account tests
		{
			name:           "Equity - Credit increases balance",
			currentBalance: decimal.NewFromInt(1000),
			amount:         decimal.NewFromInt(500),
			side:           "credit",
			accountType:    queries.AccountTypeEnumEquity,
			expected:       decimal.NewFromInt(1500),
		},
		{
			name:           "Equity - Debit decreases balance",
			currentBalance: decimal.NewFromInt(1000),
			amount:         decimal.NewFromInt(500),
			side:           "debit",
			accountType:    queries.AccountTypeEnumEquity,
			expected:       decimal.NewFromInt(500),
		},
		// Revenue account tests
		{
			name:           "Revenue - Credit increases balance",
			currentBalance: decimal.NewFromInt(1000),
			amount:         decimal.NewFromInt(500),
			side:           "credit",
			accountType:    queries.AccountTypeEnumRevenue,
			expected:       decimal.NewFromInt(1500),
		},
		{
			name:           "Revenue - Debit decreases balance",
			currentBalance: decimal.NewFromInt(1000),
			amount:         decimal.NewFromInt(500),
			side:           "debit",
			accountType:    queries.AccountTypeEnumRevenue,
			expected:       decimal.NewFromInt(500),
		},
		// Expense account tests
		{
			name:           "Expense - Debit increases balance",
			currentBalance: decimal.NewFromInt(1000),
			amount:         decimal.NewFromInt(500),
			side:           "debit",
			accountType:    queries.AccountTypeEnumExpense,
			expected:       decimal.NewFromInt(1500),
		},
		{
			name:           "Expense - Credit decreases balance",
			currentBalance: decimal.NewFromInt(1000),
			amount:         decimal.NewFromInt(500),
			side:           "credit",
			accountType:    queries.AccountTypeEnumExpense,
			expected:       decimal.NewFromInt(500),
		},
		// Edge cases
		{
			name:           "Zero balance with debit",
			currentBalance: decimal.Zero,
			amount:         decimal.NewFromInt(100),
			side:           "debit",
			accountType:    queries.AccountTypeEnumAsset,
			expected:       decimal.NewFromInt(100),
		},
		{
			name:           "Negative balance scenario",
			currentBalance: decimal.NewFromInt(100),
			amount:         decimal.NewFromInt(200),
			side:           "credit",
			accountType:    queries.AccountTypeEnumAsset,
			expected:       decimal.NewFromInt(-100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.calculateNewBalance(
				tt.currentBalance,
				tt.amount,
				tt.side,
				tt.accountType,
			)
			assert.True(t, result.Equal(tt.expected),
				"Expected %s but got %s", tt.expected.String(), result.String())
		})
	}
}

func TestValidateDoubleEntryBalance(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name    string
		entries []TransactionLineEntry
		wantErr error
	}{
		{
			name: "Balanced transaction",
			entries: []TransactionLineEntry{
				{
					AccountCode: "1000",
					Amount:      decimal.NewFromInt(1000),
					Side:        "debit",
					Currency:    "NGN",
				},
				{
					AccountCode: "2000",
					Amount:      decimal.NewFromInt(1000),
					Side:        "credit",
					Currency:    "NGN",
				},
			},
			wantErr: nil,
		},
		{
			name: "Multiple entries balanced",
			entries: []TransactionLineEntry{
				{
					AccountCode: "1000",
					Amount:      decimal.NewFromInt(1000),
					Side:        "debit",
					Currency:    "NGN",
				},
				{
					AccountCode: "1001",
					Amount:      decimal.NewFromInt(500),
					Side:        "debit",
					Currency:    "NGN",
				},
				{
					AccountCode: "2000",
					Amount:      decimal.NewFromInt(1500),
					Side:        "credit",
					Currency:    "NGN",
				},
			},
			wantErr: nil,
		},
		{
			name: "Unbalanced transaction",
			entries: []TransactionLineEntry{
				{
					AccountCode: "1000",
					Amount:      decimal.NewFromInt(1000),
					Side:        "debit",
					Currency:    "NGN",
				},
				{
					AccountCode: "2000",
					Amount:      decimal.NewFromInt(900),
					Side:        "credit",
					Currency:    "NGN",
				},
			},
			wantErr: ErrUnbalancedTransaction,
		},
		{
			name: "Too few entries",
			entries: []TransactionLineEntry{
				{
					AccountCode: "1000",
					Amount:      decimal.NewFromInt(1000),
					Side:        "debit",
					Currency:    "NGN",
				},
			},
			wantErr: ErrEmptyTransactionLines,
		},
		{
			name:    "Empty entries",
			entries: []TransactionLineEntry{},
			wantErr: ErrEmptyTransactionLines,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateDoubleEntryBalance(tt.entries)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCurrencyConsistency(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name    string
		entries []TransactionLineEntry
		wantErr error
	}{
		{
			name: "Consistent currencies",
			entries: []TransactionLineEntry{
				{
					AccountCode: "1000",
					Amount:      decimal.NewFromInt(1000),
					Side:        "debit",
					Currency:    "NGN",
				},
				{
					AccountCode: "2000",
					Amount:      decimal.NewFromInt(1000),
					Side:        "credit",
					Currency:    "NGN",
				},
			},
			wantErr: nil,
		},
		{
			name: "Inconsistent currencies",
			entries: []TransactionLineEntry{
				{
					AccountCode: "1000",
					Amount:      decimal.NewFromInt(1000),
					Side:        "debit",
					Currency:    "NGN",
				},
				{
					AccountCode: "2000",
					Amount:      decimal.NewFromInt(1000),
					Side:        "credit",
					Currency:    "USD",
				},
			},
			wantErr: ErrInvalidCurrency,
		},
		{
			name:    "Empty entries",
			entries: []TransactionLineEntry{},
			wantErr: ErrEmptyTransactionLines,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateCurrencyConsistency(tt.entries)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTransactionToResponse(t *testing.T) {
	service := &Service{}
	transactionID := uuid.New()
	reference := "REF-001"

	transaction := queries.Transaction{
		ID:             transactionID,
		IdempotencyKey: "idem-key-123",
		Description:    "Test transaction",
		Reference:      pgtype.Text{String: reference, Valid: true},
		Status: queries.NullTransactionStatusEnum{
			TransactionStatusEnum: queries.TransactionStatusEnumPosted,
			Valid:                 true,
		},
	}

	response, err := service.transactionToResponse(transaction)

	assert.NoError(t, err)
	assert.Equal(t, transactionID.String(), response.ID)
	assert.Equal(t, "idem-key-123", response.IdempotencyKey)
	assert.Equal(t, "Test transaction", response.Description)
	assert.NotNil(t, response.Reference)
	assert.Equal(t, reference, *response.Reference)
	assert.Equal(t, "posted", response.Status)
}

func BenchmarkCalculateNewBalance(b *testing.B) {
	service := &Service{}
	currentBalance := decimal.NewFromInt(1000)
	amount := decimal.NewFromInt(500)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.calculateNewBalance(
			currentBalance,
			amount,
			"debit",
			queries.AccountTypeEnumAsset,
		)
	}
}

func BenchmarkValidateDoubleEntryBalance(b *testing.B) {
	service := &Service{}
	entries := []TransactionLineEntry{
		{
			AccountCode: "1000",
			Amount:      decimal.NewFromInt(1000),
			Side:        "debit",
			Currency:    "NGN",
		},
		{
			AccountCode: "2000",
			Amount:      decimal.NewFromInt(1000),
			Side:        "credit",
			Currency:    "NGN",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.validateDoubleEntryBalance(entries)
	}
}

// Table-driven test for complex double-entry scenarios
func TestComplexDoubleEntryScenarios(t *testing.T) {
	service := &Service{}

	scenarios := []struct {
		name        string
		description string
		entries     []TransactionLineEntry
		shouldPass  bool
	}{
		{
			name:        "Simple purchase",
			description: "Buying inventory with cash",
			entries: []TransactionLineEntry{
				{AccountCode: "INVENTORY", Amount: decimal.NewFromInt(1000), Side: "debit", Currency: "NGN"},
				{AccountCode: "CASH", Amount: decimal.NewFromInt(1000), Side: "credit", Currency: "NGN"},
			},
			shouldPass: true,
		},
		{
			name:        "Salary payment",
			description: "Paying employee salary",
			entries: []TransactionLineEntry{
				{AccountCode: "SALARY_EXP", Amount: decimal.NewFromInt(5000), Side: "debit", Currency: "NGN"},
				{AccountCode: "CASH", Amount: decimal.NewFromInt(4500), Side: "credit", Currency: "NGN"},
				{AccountCode: "TAX_PAYABLE", Amount: decimal.NewFromInt(500), Side: "credit", Currency: "NGN"},
			},
			shouldPass: true,
		},
		{
			name:        "Revenue recognition",
			description: "Recording service revenue",
			entries: []TransactionLineEntry{
				{AccountCode: "CASH", Amount: decimal.NewFromInt(10000), Side: "debit", Currency: "NGN"},
				{AccountCode: "SERVICE_REV", Amount: decimal.NewFromInt(10000), Side: "credit", Currency: "NGN"},
			},
			shouldPass: true,
		},
		{
			name:        "Loan disbursement",
			description: "Disbursing a loan",
			entries: []TransactionLineEntry{
				{AccountCode: "LOANS_RECV", Amount: decimal.NewFromInt(100000), Side: "debit", Currency: "NGN"},
				{AccountCode: "FEE_REVENUE", Amount: decimal.NewFromInt(5000), Side: "credit", Currency: "NGN"},
				{AccountCode: "CASH", Amount: decimal.NewFromInt(95000), Side: "credit", Currency: "NGN"},
			},
			shouldPass: true,
		},
		{
			name:        "Unbalanced payment",
			description: "Transaction doesn't balance",
			entries: []TransactionLineEntry{
				{AccountCode: "EXPENSE", Amount: decimal.NewFromInt(1000), Side: "debit", Currency: "NGN"},
				{AccountCode: "CASH", Amount: decimal.NewFromInt(900), Side: "credit", Currency: "NGN"},
			},
			shouldPass: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			err := service.validateDoubleEntryBalance(scenario.entries)
			if scenario.shouldPass {
				assert.NoError(t, err, "Scenario: %s", scenario.description)
			} else {
				assert.Error(t, err, "Scenario: %s", scenario.description)
			}
		})
	}
}
