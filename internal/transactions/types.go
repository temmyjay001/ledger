// internal/transactions/types.go
package transactions

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// Custom errors
var (
	ErrTransactionNotFound     = errors.New("transaction not found")
	ErrInvalidAccountCode      = errors.New("invalid account code")
	ErrUnbalancedTransaction   = errors.New("debits must equal credits")
	ErrDuplicateIdempotencyKey = errors.New("idempotency key already exists")
	ErrInvalidCurrency         = errors.New("all entries must use the same currency")
	ErrEmptyTransactionLines   = errors.New("transaction must have at least one entry")
)

// Simple Transaction Request
type CreateTransactionRequest struct {
	IdempotencyKey string          `json:"idempotency_key" validate:"required,max=255"`
	Description    string          `json:"description" validate:"required,max=500"`
	Reference      string          `json:"reference,omitempty" validate:"omitempty,max=255"`
	AccountCode    string          `json:"account_code" validate:"required"`
	Amount         decimal.Decimal `json:"amount" validate:"required,dgt=0"`
	Side           string          `json:"side" validate:"required,oneof=debit credit"`
	Currency       string          `json:"currency" validate:"required,len=3"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

// Double-Entry Transaction Request
type TransactionLineEntry struct {
	AccountCode string          `json:"account_code" validate:"required"`
	Amount      decimal.Decimal `json:"amount" validate:"required,dgt=0"`
	Side        string          `json:"side" validate:"required,oneof=debit credit"`
	Currency    string          `json:"currency" validate:"required,len=3"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

type CreateDoubleEntryRequest struct {
	IdempotencyKey string                 `json:"idempotency_key" validate:"required,max=255"`
	Description    string                 `json:"description" validate:"required,max=500"`
	Reference      string                 `json:"reference,omitempty" validate:"omitempty,max=255"`
	Entries        []TransactionLineEntry `json:"entries" validate:"required,min=2,dive"`
	Metadata       json.RawMessage        `json:"metadata,omitempty"`
}

// List Transactions Request
type ListTransactionsRequest struct {
	Limit       int    `validate:"min=1,max=100"`
	Offset      int    `validate:"min=0"`
	AccountCode string `validate:"omitempty"`
	StartDate   string `validate:"omitempty,datetime=2006-01-02"`
	EndDate     string `validate:"omitempty,datetime=2006-01-02"`
}

// Response Types
type TransactionResponse struct {
	ID             string                    `json:"id"`
	IdempotencyKey string                    `json:"idempotency_key"`
	Description    string                    `json:"description"`
	Reference      *string                   `json:"reference,omitempty"`
	Status         string                    `json:"status"`
	PostedAt       time.Time                 `json:"posted_at"`
	Metadata       json.RawMessage           `json:"metadata,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	Lines          []TransactionLineResponse `json:"lines,omitempty"`
}

type TransactionLineResponse struct {
	ID          string          `json:"id"`
	AccountID   string          `json:"account_id"`
	AccountCode string          `json:"account_code"`
	AccountName string          `json:"account_name"`
	Amount      decimal.Decimal `json:"amount"`
	Side        string          `json:"side"`
	Currency    string          `json:"currency"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

type TransactionListResponse struct {
	Transactions []TransactionResponse `json:"transactions"`
	Pagination   PaginationInfo        `json:"pagination"`
}

type PaginationInfo struct {
	Total   int64 `json:"total"`
	Limit   int   `json:"limit"`
	Offset  int   `json:"offset"`
	HasMore bool  `json:"has_more"`
}

// Balance History Types (for account enhancements)
type BalanceHistoryEntry struct {
	Balance     decimal.Decimal `json:"balance"`
	Version     int64           `json:"version"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Description string          `json:"description,omitempty"`
	Reference   string          `json:"reference,omitempty"`
}

type BalanceHistoryResponse struct {
	AccountID string                `json:"account_id"`
	Currency  string                `json:"currency"`
	Days      int                   `json:"days"`
	History   []BalanceHistoryEntry `json:"history"`
}

// Balance Summary Types
type BalanceSummaryEntry struct {
	AccountID   string          `json:"account_id"`
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	AccountType string          `json:"account_type"`
	Currency    string          `json:"currency"`
	Balance     decimal.Decimal `json:"balance"`
	Version     int64           `json:"version"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type BalanceSummaryResponse struct {
	Currency string                `json:"currency,omitempty"`
	Balances []BalanceSummaryEntry `json:"balances"`
	Total    int                   `json:"total"`
}
