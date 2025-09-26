// internal/events/types.go
package events

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// Event payload wrapper for different event types
type EventPayload struct {
	TransactionPosted *TransactionPostedEvent `json:"transaction_posted,omitempty"`
	BalanceUpdated    *BalanceUpdatedEvent    `json:"balance_updated,omitempty"`
}

// TransactionPostedEvent represents a transaction that was successfully posted
type TransactionPostedEvent struct {
	TransactionID  string                 `json:"transaction_id"`
	IdempotencyKey string                 `json:"idempotency_key"`
	Description    string                 `json:"description"`
	Reference      *string                `json:"reference,omitempty"`
	Lines          []TransactionLineEvent `json:"lines"`
	PostedAt       time.Time              `json:"posted_at"`
	Currency       string                 `json:"currency"`
	TotalAmount    decimal.Decimal        `json:"total_amount"`
	Metadata       json.RawMessage        `json:"metadata,omitempty"`
}

// TransactionLineEvent represents individual transaction line details
type TransactionLineEvent struct {
	ID          string          `json:"id"`
	AccountID   string          `json:"account_id"`
	AccountCode string          `json:"account_code"`
	AccountName string          `json:"account_name"`
	Amount      decimal.Decimal `json:"amount"`
	Side        string          `json:"side"` // "debit" or "credit"
	Currency    string          `json:"currency"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// BalanceUpdatedEvent represents a balance change
type BalanceUpdatedEvent struct {
	AccountID       string          `json:"account_id"`
	AccountCode     string          `json:"account_code"`
	AccountName     string          `json:"account_name"`
	Currency        string          `json:"currency"`
	PreviousBalance decimal.Decimal `json:"previous_balance"`
	NewBalance      decimal.Decimal `json:"new_balance"`
	BalanceChange   decimal.Decimal `json:"balance_change"`
	UpdatedBy       string          `json:"updated_by"` // transaction_id that caused the change
	UpdatedAt       time.Time       `json:"updated_at"`
	Version         int64           `json:"version"`
}

// EventMetadata contains contextual information about the event
type EventMetadata struct {
	UserID        *string `json:"user_id,omitempty"`
	APIKeyID      *string `json:"api_key_id,omitempty"`
	SourceIP      *string `json:"source_ip,omitempty"`
	CorrelationID *string `json:"correlation_id,omitempty"`
	Source        string  `json:"source"` // "api", "batch", "system", etc.
}

// Event types constants
const (
	EventTypeTransactionPosted = "transaction.posted"
	EventTypeBalanceUpdated    = "balance.updated"
	EventTypeAccountCreated    = "account.created"
	EventTypeAccountUpdated    = "account.updated"
)

// Aggregate types constants
const (
	AggregateTypeTransaction = "transaction"
	AggregateTypeAccount     = "account"
	AggregateTypeBalance     = "balance"
)