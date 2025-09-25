// internal/transactions/service.go
package transactions

import (
	"context"
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

// CreateSimpleTransaction creates a single-entry transaction
func (s *Service) CreateSimpleTransaction(ctx context.Context, tenantSlug string, req CreateTransactionRequest) (*TransactionResponse, error) {
	// Set tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	// Check idempotency
	existing, err := s.db.Queries.GetTransactionByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil {
		log.Printf("Transaction with idempotency key %s already exists", req.IdempotencyKey)
		return s.transactionToResponse(existing)
	}

	// Start database transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create transaction queries with tx
	qtx := s.db.Queries.WithTx(tx)

	// Validate account exists
	account, err := qtx.GetAccountByCode(ctx, req.AccountCode)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	// Create transaction record
	reference := pgtype.Text{}
	if req.Reference != "" {
		reference = pgtype.Text{String: req.Reference, Valid: true}
	}

	transaction, err := qtx.CreateTransaction(ctx, queries.CreateTransactionParams{
		IdempotencyKey: req.IdempotencyKey,
		Description:    req.Description,
		Reference:      reference,
		Metadata:       req.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Create transaction line
	_, err = qtx.CreateTransactionLine(ctx, queries.CreateTransactionLineParams{
		TransactionID: transaction.ID,
		AccountID:     account.ID,
		Amount:        req.Amount,
		Side:          queries.TransactionSideEnum(req.Side),
		Currency:      req.Currency,
		Metadata:      req.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction line: %w", err)
	}

	// Update account balance with optimistic locking
	if err := s.updateAccountBalance(ctx, qtx, account.ID, req.Amount, req.Side, req.Currency); err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	// Mark transaction as posted
	transaction, err = qtx.UpdateTransactionStatus(ctx, queries.UpdateTransactionStatusParams{
		ID:     transaction.ID,
		Status: queries.NullTransactionStatusEnum{TransactionStatusEnum: queries.TransactionStatusEnumPosted, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to post transaction: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Simple transaction created successfully: %s", transaction.ID)
	return s.transactionToResponse(transaction)
}

// CreateDoubleEntryTransaction creates a double-entry transaction
func (s *Service) CreateDoubleEntryTransaction(ctx context.Context, tenantSlug string, req CreateDoubleEntryRequest) (*TransactionResponse, error) {
	// Set tenant schema
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	// Validate double-entry balance
	if err := s.validateDoubleEntryBalance(req.Entries); err != nil {
		return nil, err
	}

	// Validate currency consistency
	if err := s.validateCurrencyConsistency(req.Entries); err != nil {
		return nil, err
	}

	// Check idempotency
	existing, err := s.db.Queries.GetTransactionByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil {
		log.Printf("Transaction with idempotency key %s already exists", req.IdempotencyKey)
		return s.transactionToResponse(existing)
	}

	// Start database transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.db.Queries.WithTx(tx)

	// Validate all accounts exist
	accountMap := make(map[string]queries.Account)
	for _, entry := range req.Entries {
		account, err := qtx.GetAccountByCode(ctx, entry.AccountCode)
		if err != nil {
			return nil, fmt.Errorf("account %s not found: %w", entry.AccountCode, err)
		}
		accountMap[entry.AccountCode] = account
	}

	// Create transaction record
	reference := pgtype.Text{}
	if req.Reference != "" {
		reference = pgtype.Text{String: req.Reference, Valid: true}
	}

	transaction, err := qtx.CreateTransaction(ctx, queries.CreateTransactionParams{
		IdempotencyKey: req.IdempotencyKey,
		Description:    req.Description,
		Reference:      reference,
		Metadata:       req.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Create transaction lines and update balances
	for _, entry := range req.Entries {
		account := accountMap[entry.AccountCode]

		// Create transaction line
		_, err = qtx.CreateTransactionLine(ctx, queries.CreateTransactionLineParams{
			TransactionID: transaction.ID,
			AccountID:     account.ID,
			Amount:        entry.Amount,
			Side:          queries.TransactionSideEnum(entry.Side),
			Currency:      entry.Currency,
			Metadata:      entry.Metadata,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create transaction line for account %s: %w", entry.AccountCode, err)
		}

		// Update account balance
		if err := s.updateAccountBalance(ctx, qtx, account.ID, entry.Amount, entry.Side, entry.Currency); err != nil {
			return nil, fmt.Errorf("failed to update balance for account %s: %w", entry.AccountCode, err)
		}
	}

	// Mark transaction as posted
	transaction, err = qtx.UpdateTransactionStatus(ctx, queries.UpdateTransactionStatusParams{
		ID:     transaction.ID,
		Status: queries.NullTransactionStatusEnum{TransactionStatusEnum: queries.TransactionStatusEnumPosted, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to post transaction: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Double-entry transaction created successfully: %s", transaction.ID)
	return s.transactionToResponse(transaction)
}

// GetTransaction retrieves a single transaction by ID
func (s *Service) GetTransaction(ctx context.Context, tenantSlug string, id uuid.UUID) (*TransactionResponse, error) {
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	transaction, err := s.db.Queries.GetTransactionByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	return s.transactionToResponse(transaction)
}

// GetTransactionLines retrieves lines for a transaction
func (s *Service) GetTransactionLines(ctx context.Context, tenantSlug string, transactionID uuid.UUID) ([]TransactionLineResponse, error) {
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	lines, err := s.db.Queries.GetTransactionLines(ctx, transactionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction lines: %w", err)
	}

	var response []TransactionLineResponse
	for _, line := range lines {
		response = append(response, TransactionLineResponse{
			ID:          line.ID.String(),
			AccountID:   line.AccountID.String(),
			AccountCode: line.AccountCode,
			AccountName: line.AccountName,
			Amount:      line.Amount,
			Side:        string(line.Side),
			Currency:    line.Currency,
			Metadata:    line.Metadata,
			CreatedAt:   line.CreatedAt,
		})
	}

	return response, nil
}

// ListTransactions retrieves transactions with filtering
func (s *Service) ListTransactions(ctx context.Context, tenantSlug string, req ListTransactionsRequest) (*TransactionListResponse, error) {
	if err := s.db.SetSearchPath(ctx, "tenant_"+tenantSlug); err != nil {
		return nil, fmt.Errorf("failed to set tenant schema: %w", err)
	}
	defer s.db.SetSearchPath(ctx, "public")

	var transactions []queries.Transaction
	var err error

	// Apply different query strategies based on filters
	if req.AccountCode != "" && req.StartDate != "" && req.EndDate != "" {
		// Account + Date range
		startDate, _ := time.Parse("2006-01-02", req.StartDate)
		endDate, _ := time.Parse("2006-01-02", req.EndDate)

		transactions, err = s.db.Queries.ListTransactionsByAccountAndDateRange(ctx, queries.ListTransactionsByAccountAndDateRangeParams{
			Code:       req.AccountCode,
			PostedAt:   startDate,
			PostedAt_2: endDate,
			Limit:      int32(req.Limit),
			Offset:     int32(req.Offset),
		})
	} else if req.AccountCode != "" {
		// Account only
		transactions, err = s.db.Queries.ListTransactionsByAccount(ctx, queries.ListTransactionsByAccountParams{
			Code:   req.AccountCode,
			Limit:  int32(req.Limit),
			Offset: int32(req.Offset),
		})
	} else if req.StartDate != "" && req.EndDate != "" {
		// Date range only
		startDate, _ := time.Parse("2006-01-02", req.StartDate)
		endDate, _ := time.Parse("2006-01-02", req.EndDate)

		transactions, err = s.db.Queries.ListTransactionsByDateRange(ctx, queries.ListTransactionsByDateRangeParams{
			PostedAt:   startDate,
			PostedAt_2: endDate,
			Limit:      int32(req.Limit),
			Offset:     int32(req.Offset),
		})
	} else {
		// No filters
		transactions, err = s.db.Queries.ListTransactions(ctx, queries.ListTransactionsParams{
			Limit:  int32(req.Limit),
			Offset: int32(req.Offset),
		})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	var response []TransactionResponse
	for _, t := range transactions {
		txnResp, err := s.transactionToResponse(t)
		if err != nil {
			log.Printf("Failed to convert transaction to response: %v", err)
			continue
		}
		response = append(response, *txnResp)
	}

	return &TransactionListResponse{
		Transactions: response,
		Pagination: PaginationInfo{
			Total:   int64(len(response)), // TODO: Implement proper count query
			Limit:   req.Limit,
			Offset:  req.Offset,
			HasMore: len(response) == req.Limit,
		},
	}, nil
}

// Helper functions
func (s *Service) updateAccountBalance(ctx context.Context, qtx *queries.Queries, accountID uuid.UUID, amount decimal.Decimal, side, currency string) error {
	// Get current balance with version for optimistic locking
	balance, err := qtx.GetAccountBalanceForUpdate(ctx, queries.GetAccountBalanceForUpdateParams{
		AccountID: accountID,
		Currency:  currency,
	})
	if err != nil {
		// Create balance if it doesn't exist
		_, err = qtx.CreateAccountBalance(ctx, queries.CreateAccountBalanceParams{
			AccountID: accountID,
			Currency:  currency,
			Balance:   decimal.Zero,
		})
		if err != nil {
			return fmt.Errorf("failed to create balance: %w", err)
		}

		// Retry getting balance
		balance, err = qtx.GetAccountBalanceForUpdate(ctx, queries.GetAccountBalanceForUpdateParams{
			AccountID: accountID,
			Currency:  currency,
		})
		if err != nil {
			return fmt.Errorf("failed to get balance after creation: %w", err)
		}
	}

	// Calculate new balance based on debit/credit
	var newBalance decimal.Decimal
	if side == "debit" {
		newBalance = balance.Balance.Add(amount)
	} else {
		newBalance = balance.Balance.Sub(amount)
	}

	// Update with optimistic locking
	_, err = qtx.UpdateAccountBalance(ctx, queries.UpdateAccountBalanceParams{
		AccountID: accountID,
		Currency:  currency,
		Balance:   newBalance,
		Version:   balance.Version,
	})
	if err != nil {
		return fmt.Errorf("failed to update balance (possible version conflict): %w", err)
	}

	return nil
}

func (s *Service) validateDoubleEntryBalance(entries []TransactionLineEntry) error {
	if len(entries) < 2 {
		return ErrEmptyTransactionLines
	}

	debitTotal := decimal.Zero
	creditTotal := decimal.Zero

	for _, entry := range entries {
		if entry.Side == "debit" {
			debitTotal = debitTotal.Add(entry.Amount)
		} else {
			creditTotal = creditTotal.Add(entry.Amount)
		}
	}

	if !debitTotal.Equal(creditTotal) {
		return ErrUnbalancedTransaction
	}

	return nil
}

func (s *Service) validateCurrencyConsistency(entries []TransactionLineEntry) error {
	if len(entries) == 0 {
		return ErrEmptyTransactionLines
	}

	baseCurrency := entries[0].Currency
	for _, entry := range entries[1:] {
		if entry.Currency != baseCurrency {
			return ErrInvalidCurrency
		}
	}

	return nil
}

func (s *Service) transactionToResponse(t queries.Transaction) (*TransactionResponse, error) {
	response := &TransactionResponse{
		ID:             t.ID.String(),
		IdempotencyKey: t.IdempotencyKey,
		Description:    t.Description,
		Status:         string(t.Status.TransactionStatusEnum),
		PostedAt:       t.PostedAt,
		Metadata:       t.Metadata,
		CreatedAt:      t.CreatedAt,
	}

	if t.Reference.Valid {
		response.Reference = &t.Reference.String
	}

	return response, nil
}
