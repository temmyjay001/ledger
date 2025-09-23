package accounts

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Errors
var (
	ErrAccountNotFound        = errors.New("account not found")
	ErrAccountCodeExists      = errors.New("account code already exists")
	ErrInvalidAccountCode     = errors.New("invalid account code format")
	ErrInvalidParentAccount   = errors.New("invalid parent account")
	ErrAccountHasChildren     = errors.New("cannot delete account with child accounts")
	ErrAccountHasBalances     = errors.New("cannot delete account with non-zero balances")
	ErrInvalidCurrency        = errors.New("invalid currency code")
	ErrInvalidAccountType     = errors.New("invalid account type")
	ErrBalanceVersionConflict = errors.New("balance version conflict - concurrent update detected")
)

// Account Types
const (
	AccountTypeAsset     = "asset"
	AccountTypeLiability = "liability"
	AccountTypeEquity    = "equity"
	AccountTypeRevenue   = "revenue"
	AccountTypeExpense   = "expense"
)

var ValidAccountTypes = []string{
	AccountTypeAsset,
	AccountTypeLiability,
	AccountTypeEquity,
	AccountTypeRevenue,
	AccountTypeExpense,
}

var ValidCurrencies = []string{"NGN", "USD", "EUR", "GBP", "ZAR", "GHS", "XOF", "XAF", "KES", "UGX"}

// Request Types

type CreateAccountRequest struct {
	Code        string                 `json:"code" validate:"required,min=1,max=20"`
	Name        string                 `json:"name" validate:"required,min=1,max=255"`
	AccountType string                 `json:"account_type" validate:"required,oneof=asset liability equity revenue expense"`
	ParentCode  string                 `json:"parent_code,omitempty"`
	Currency    string                 `json:"currency,omitempty" validate:"omitempty,len=3"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type UpdateAccountRequest struct {
	Name     string                 `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ListAccountsRequest struct {
	AccountType string `json:"account_type,omitempty" validate:"omitempty,oneof=asset liability equity revenue expense"`
	ParentCode  string `json:"parent_code,omitempty"`
	Currency    string `json:"currency,omitempty" validate:"omitempty,len=3"`
	Search      string `json:"search,omitempty"`
	Limit       int    `json:"limit,omitempty" validate:"omitempty,min=1,max=1000"`
}

// Response Types

type AccountResponse struct {
	ID          uuid.UUID              `json:"id"`
	Code        string                 `json:"code"`
	Name        string                 `json:"name"`
	AccountType string                 `json:"account_type"`
	ParentID    *uuid.UUID             `json:"parent_id,omitempty"`
	ParentCode  string                 `json:"parent_code,omitempty"`
	Currency    string                 `json:"currency"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Level       int                    `json:"level,omitempty"`
	Path        string                 `json:"path,omitempty"`
	IsActive    bool                   `json:"is_active"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type AccountBalanceResponse struct {
	Currency  string          `json:"currency"`
	Balance   decimal.Decimal `json:"balance"`
	Version   int64           `json:"version"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type AccountWithBalanceResponse struct {
	AccountResponse
	Balance         *decimal.Decimal `json:"balance,omitempty"`
	BalanceCurrency string           `json:"balance_currency,omitempty"`
	BalanceVersion  int64            `json:"balance_version,omitempty"`
	BalanceUpdatedAt *time.Time      `json:"balance_updated_at,omitempty"`
}

type AccountWithBalancesResponse struct {
	AccountResponse
	Balances []AccountBalanceResponse `json:"balances"`
}

type AccountStatsResponse struct {
	TotalAccounts     int64 `json:"total_accounts"`
	AssetAccounts     int64 `json:"asset_accounts"`
	LiabilityAccounts int64 `json:"liability_accounts"`
	EquityAccounts    int64 `json:"equity_accounts"`
	RevenueAccounts   int64 `json:"revenue_accounts"`
	ExpenseAccounts   int64 `json:"expense_accounts"`
	CurrenciesCount   int64 `json:"currencies_count"`
}

type BalanceSummaryResponse struct {
	AccountType    string          `json:"account_type"`
	Currency       string          `json:"currency"`
	TotalBalance   decimal.Decimal `json:"total_balance"`
	AccountCount   int64           `json:"account_count"`
}

// Internal Types for Business Logic

type AccountHierarchyItem struct {
	Account  AccountResponse
	Children []AccountHierarchyItem
}

type BalanceUpdate struct {
	AccountID uuid.UUID
	Currency  string
	Amount    decimal.Decimal
	Version   int64
}

// Nigerian Fintech Templates

type ChartOfAccountsTemplate struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Accounts    []CreateAccountRequest    `json:"accounts"`
}

// Default chart of accounts templates for different business types
func GetChartOfAccountsTemplate(businessType string) ChartOfAccountsTemplate {
	switch businessType {
	case "wallet":
		return getWalletTemplate()
	case "payments":
		return getPaymentProcessorTemplate()
	case "lending":
		return getLendingTemplate()
	case "trading":
		return getTradingTemplate()
	default:
		return getBasicTemplate()
	}
}

func getWalletTemplate() ChartOfAccountsTemplate {
	return ChartOfAccountsTemplate{
		Name:        "Digital Wallet Chart of Accounts",
		Description: "Standard chart of accounts for Nigerian digital wallet companies",
		Accounts: []CreateAccountRequest{
			// Assets
			{Code: "1000", Name: "Assets", AccountType: AccountTypeAsset, Currency: "NGN"},
			{Code: "1100", Name: "Cash and Bank Accounts", AccountType: AccountTypeAsset, ParentCode: "1000", Currency: "NGN"},
			{Code: "1101", Name: "Settlement Account - NGN", AccountType: AccountTypeAsset, ParentCode: "1100", Currency: "NGN"},
			{Code: "1102", Name: "Settlement Account - USD", AccountType: AccountTypeAsset, ParentCode: "1100", Currency: "USD"},
			{Code: "1200", Name: "Customer Receivables", AccountType: AccountTypeAsset, ParentCode: "1000", Currency: "NGN"},
			{Code: "1201", Name: "Customer Wallet Pool - NGN", AccountType: AccountTypeAsset, ParentCode: "1200", Currency: "NGN"},
			{Code: "1202", Name: "Customer Wallet Pool - USD", AccountType: AccountTypeAsset, ParentCode: "1200", Currency: "USD"},
			
			// Liabilities
			{Code: "2000", Name: "Liabilities", AccountType: AccountTypeLiability, Currency: "NGN"},
			{Code: "2001", Name: "Customer Deposits - NGN", AccountType: AccountTypeLiability, ParentCode: "2000", Currency: "NGN"},
			{Code: "2002", Name: "Customer Deposits - USD", AccountType: AccountTypeLiability, ParentCode: "2000", Currency: "USD"},
			{Code: "2100", Name: "Regulatory Reserves", AccountType: AccountTypeLiability, ParentCode: "2000", Currency: "NGN"},
			{Code: "2101", Name: "CBN Required Reserves", AccountType: AccountTypeLiability, ParentCode: "2100", Currency: "NGN"},
			
			// Equity
			{Code: "3000", Name: "Equity", AccountType: AccountTypeEquity, Currency: "NGN"},
			{Code: "3001", Name: "Share Capital", AccountType: AccountTypeEquity, ParentCode: "3000", Currency: "NGN"},
			{Code: "3101", Name: "Retained Earnings", AccountType: AccountTypeEquity, ParentCode: "3000", Currency: "NGN"},
			
			// Revenue
			{Code: "4000", Name: "Revenue", AccountType: AccountTypeRevenue, Currency: "NGN"},
			{Code: "4001", Name: "Transaction Fee Revenue", AccountType: AccountTypeRevenue, ParentCode: "4000", Currency: "NGN"},
			{Code: "4002", Name: "FX Spread Revenue", AccountType: AccountTypeRevenue, ParentCode: "4000", Currency: "NGN"},
			{Code: "4101", Name: "Interest Income", AccountType: AccountTypeRevenue, ParentCode: "4000", Currency: "NGN"},
			
			// Expenses
			{Code: "5000", Name: "Expenses", AccountType: AccountTypeExpense, Currency: "NGN"},
			{Code: "5001", Name: "Operational Expenses", AccountType: AccountTypeExpense, ParentCode: "5000", Currency: "NGN"},
			{Code: "5101", Name: "Bank Charges", AccountType: AccountTypeExpense, ParentCode: "5001", Currency: "NGN"},
			{Code: "5102", Name: "Regulatory Fees", AccountType: AccountTypeExpense, ParentCode: "5001", Currency: "NGN"},
		},
	}
}

func getPaymentProcessorTemplate() ChartOfAccountsTemplate {
	return ChartOfAccountsTemplate{
		Name:        "Payment Processor Chart of Accounts",
		Description: "Standard chart of accounts for Nigerian payment processing companies",
		Accounts: []CreateAccountRequest{
			// Assets
			{Code: "1000", Name: "Assets", AccountType: AccountTypeAsset, Currency: "NGN"},
			{Code: "1100", Name: "Cash and Bank Accounts", AccountType: AccountTypeAsset, ParentCode: "1000", Currency: "NGN"},
			{Code: "1101", Name: "Settlement Account - NGN", AccountType: AccountTypeAsset, ParentCode: "1100", Currency: "NGN"},
			{Code: "1300", Name: "Merchant Receivables", AccountType: AccountTypeAsset, ParentCode: "1000", Currency: "NGN"},
			{Code: "1301", Name: "Merchant Settlement Pool - NGN", AccountType: AccountTypeAsset, ParentCode: "1300", Currency: "NGN"},
			
			// Liabilities
			{Code: "2000", Name: "Liabilities", AccountType: AccountTypeLiability, Currency: "NGN"},
			{Code: "2300", Name: "Merchant Balances", AccountType: AccountTypeLiability, ParentCode: "2000", Currency: "NGN"},
			{Code: "2301", Name: "Merchant Balances - NGN", AccountType: AccountTypeLiability, ParentCode: "2300", Currency: "NGN"},
			
			// Revenue
			{Code: "4000", Name: "Revenue", AccountType: AccountTypeRevenue, Currency: "NGN"},
			{Code: "4001", Name: "Processing Fee Revenue", AccountType: AccountTypeRevenue, ParentCode: "4000", Currency: "NGN"},
			{Code: "4002", Name: "Interchange Revenue", AccountType: AccountTypeRevenue, ParentCode: "4000", Currency: "NGN"},
		},
	}
}

func getLendingTemplate() ChartOfAccountsTemplate {
	return ChartOfAccountsTemplate{
		Name:        "Lending Platform Chart of Accounts",
		Description: "Standard chart of accounts for Nigerian lending companies",
		Accounts: []CreateAccountRequest{
			// Assets
			{Code: "1000", Name: "Assets", AccountType: AccountTypeAsset, Currency: "NGN"},
			{Code: "1400", Name: "Loans and Advances", AccountType: AccountTypeAsset, ParentCode: "1000", Currency: "NGN"},
			{Code: "1401", Name: "Loans Outstanding - NGN", AccountType: AccountTypeAsset, ParentCode: "1400", Currency: "NGN"},
			{Code: "1402", Name: "Interest Receivable", AccountType: AccountTypeAsset, ParentCode: "1400", Currency: "NGN"},
			
			// Revenue
			{Code: "4000", Name: "Revenue", AccountType: AccountTypeRevenue, Currency: "NGN"},
			{Code: "4101", Name: "Interest Income", AccountType: AccountTypeRevenue, ParentCode: "4000", Currency: "NGN"},
			{Code: "4102", Name: "Processing Fee Revenue", AccountType: AccountTypeRevenue, ParentCode: "4000", Currency: "NGN"},
			
			// Expenses
			{Code: "5000", Name: "Expenses", AccountType: AccountTypeExpense, Currency: "NGN"},
			{Code: "5200", Name: "Credit Costs", AccountType: AccountTypeExpense, ParentCode: "5000", Currency: "NGN"},
			{Code: "5201", Name: "Loan Loss Provisions", AccountType: AccountTypeExpense, ParentCode: "5200", Currency: "NGN"},
		},
	}
}

func getTradingTemplate() ChartOfAccountsTemplate {
	return ChartOfAccountsTemplate{
		Name:        "Trading Platform Chart of Accounts", 
		Description: "Standard chart of accounts for Nigerian trading/investment platforms",
		Accounts: []CreateAccountRequest{
			// Assets
			{Code: "1000", Name: "Assets", AccountType: AccountTypeAsset, Currency: "NGN"},
			{Code: "1500", Name: "Customer Trading Balances", AccountType: AccountTypeAsset, ParentCode: "1000", Currency: "NGN"},
			{Code: "1501", Name: "Customer Cash Pool - NGN", AccountType: AccountTypeAsset, ParentCode: "1500", Currency: "NGN"},
			{Code: "1502", Name: "Customer Cash Pool - USD", AccountType: AccountTypeAsset, ParentCode: "1500", Currency: "USD"},
			
			// Revenue
			{Code: "4000", Name: "Revenue", AccountType: AccountTypeRevenue, Currency: "NGN"},
			{Code: "4003", Name: "Trading Fee Revenue", AccountType: AccountTypeRevenue, ParentCode: "4000", Currency: "NGN"},
			{Code: "4004", Name: "Spread Revenue", AccountType: AccountTypeRevenue, ParentCode: "4000", Currency: "NGN"},
		},
	}
}

func getBasicTemplate() ChartOfAccountsTemplate {
	return ChartOfAccountsTemplate{
		Name:        "Basic Chart of Accounts",
		Description: "Minimal chart of accounts for general financial services",
		Accounts: []CreateAccountRequest{
			// Assets
			{Code: "1000", Name: "Assets", AccountType: AccountTypeAsset, Currency: "NGN"},
			{Code: "1101", Name: "Cash - NGN", AccountType: AccountTypeAsset, ParentCode: "1000", Currency: "NGN"},
			
			// Liabilities
			{Code: "2000", Name: "Liabilities", AccountType: AccountTypeLiability, Currency: "NGN"},
			
			// Equity
			{Code: "3000", Name: "Equity", AccountType: AccountTypeEquity, Currency: "NGN"},
			
			// Revenue
			{Code: "4000", Name: "Revenue", AccountType: AccountTypeRevenue, Currency: "NGN"},
			
			// Expenses
			{Code: "5000", Name: "Expenses", AccountType: AccountTypeExpense, Currency: "NGN"},
		},
	}
}

// Validation helpers

func IsValidAccountType(accountType string) bool {
	for _, valid := range ValidAccountTypes {
		if accountType == valid {
			return true
		}
	}
	return false
}

func IsValidCurrency(currency string) bool {
	if currency == "" {
		return false
	}
	for _, valid := range ValidCurrencies {
		if currency == valid {
			return true
		}
	}
	return false
}

func ValidateAccountCode(code string) error {
	if code == "" {
		return ErrInvalidAccountCode
	}
	if len(code) > 20 {
		return ErrInvalidAccountCode
	}
	// Account codes should be alphanumeric with optional hyphens/underscores
	for _, r := range code {
		if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '-' || r == '_') {
			return ErrInvalidAccountCode
		}
	}
	return nil
}