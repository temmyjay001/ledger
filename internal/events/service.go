package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

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
	return &Service{db: db}
}

// PublishTransactionPosted publishes a transaction.posted event
func (s *Service) PublishTransactionPosted(
	ctx context.Context,
	qtx *queries.Queries,
	tenantID uuid.UUID,
	transaction queries.Transaction,
	lines []queries.TransactionLine,
	accounts map[uuid.UUID]queries.Account) error {

	// Calculate total amount (sum of all debits or credits)
	totalAmount := decimal.Zero
	eventLines := make([]TransactionLineEvent, len(lines))
	currency := ""

	for i, line := range lines {
		account, exists := accounts[line.AccountID]
		if !exists {
			return fmt.Errorf("account not found for line ID: %s", line.AccountID)
		}

		eventLines[i] = TransactionLineEvent{
			ID:          line.ID.String(),
			AccountID:   account.ID.String(),
			AccountCode: account.Code,
			AccountName: account.Name,
			Amount:      line.Amount,
			Side:        string(line.Side),
			Currency:    line.Currency,
			Metadata:    line.Metadata,
		}

		// set currency from first line and sum amounts
		if currency == "" {
			currency = line.Currency
		}

		if line.Side == queries.TransactionSideEnumDebit {
			totalAmount = totalAmount.Add(line.Amount)
		}
	}

	// create event payload
	eventPayload := &TransactionPostedEvent{
		TransactionID:  transaction.ID.String(),
		IdempotencyKey: transaction.IdempotencyKey,
		Description:    transaction.Description,
		Lines:          eventLines,
		PostedAt:       transaction.PostedAt,
		Currency:       currency,
		TotalAmount:    totalAmount,
		Metadata:       transaction.Metadata,
	}

	if transaction.Reference.Valid {
		eventPayload.Reference = &transaction.Reference.String
	}

	// serialize event data
	eventData, err := json.Marshal(eventPayload)
	if err != nil {
		return fmt.Errorf("failed to serialize transaction event: %w", err)
	}

	// create metadata
	metadata := EventMetadata{
		Source: "api", // TODO: Extract from context
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to serialize event metadata: %w", err)
	}
	// Create event record
	_, err = qtx.CreateEvent(ctx, queries.CreateEventParams{
		TenantID:      tenantID,
		AggregateID:   transaction.ID,
		AggregateType: AggregateTypeTransaction,
		EventType:     EventTypeTransactionPosted,
		EventVersion:  1,
		EventData:     eventData,
		Metadata:      metadataBytes,
	})

	if err != nil {
		return fmt.Errorf("failed to create transaction posted event: %w", err)
	}

	log.Printf("Published transaction.posted event for transaction %s", transaction.ID)

	return nil
}

// PublishBalanceUpdated published a balance.updated event
func (s *Service) PublishBalanceUpdated(
	ctx context.Context,
	qtx *queries.Queries,
	tenantID uuid.UUID,
	account queries.Account,
	oldBalance decimal.Decimal,
	newBalance decimal.Decimal,
	transactionID uuid.UUID,
	currency string,
	version int64,
) error {
	balanceChange := newBalance.Sub(oldBalance)

	// Create event payload
	eventPayload := BalanceUpdatedEvent{
		AccountID:       account.ID.String(),
		AccountCode:     account.Code,
		AccountName:     account.Name,
		Currency:        currency,
		PreviousBalance: oldBalance,
		NewBalance:      newBalance,
		BalanceChange:   balanceChange,
		UpdatedBy:       transactionID.String(),
		UpdatedAt:       time.Now().UTC(),
		Version:         version,
	}

	// Serialize event data
	eventData, err := json.Marshal(eventPayload)
	if err != nil {
		return fmt.Errorf("failed to serialize balance event: %w", err)
	}

	// Create metadata
	metadata := EventMetadata{
		Source: "api", // TODO: Extract from context
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to serialize event metadata: %w", err)
	}

	// Create event record
	_, err = qtx.CreateEvent(ctx, queries.CreateEventParams{
		TenantID:      tenantID,
		AggregateID:   account.ID,
		AggregateType: AggregateTypeAccount,
		EventType:     EventTypeBalanceUpdated,
		EventVersion:  1, // Could be incremented for multiple balance updates
		EventData:     eventData,
		Metadata:      metadataBytes,
	})

	if err != nil {
		return fmt.Errorf("failed to create balance updated event: %w", err)
	}

	log.Printf("Published balance.updated event for account %s (%s)", account.Code, account.ID)

	return nil
}

// GetEventsByAggregate retrieves events for a specific aggregate (transaction/account)
func (s *Service) GetEventsByAggregate(ctx context.Context, tenantID uuid.UUID, aggregateID uuid.UUID) ([]queries.Event, error) {
	return s.db.Queries.GetEventsByAggregate(ctx, queries.GetEventsByAggregateParams{
		TenantID:    tenantID,
		AggregateID: aggregateID,
	})
}

// GetEventsByType retrieves events by type with pagination
func (s *Service) GetEventsByType(ctx context.Context, tenantID uuid.UUID, eventType string, limit int32, offset int32) ([]queries.Event, error) {
	return s.db.Queries.GetEventsByType(ctx, queries.GetEventsByTypeParams{
		TenantID:  tenantID,
		EventType: eventType,
		Limit:     limit,
		Offset:    offset,
	})
}

// GetEventStream retrieves events after a specific sequence number
func (s *Service) GetEventStream(ctx context.Context, afterSequence int64, limit int32) ([]queries.Event, error) {
	return s.db.Queries.GetEventsAfterSequence(ctx, queries.GetEventsAfterSequenceParams{
		SequenceNumber: pgtype.Int8{Int64: afterSequence, Valid: true},
		Limit:          limit,
	})
}
