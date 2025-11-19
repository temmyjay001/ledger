// internal/transactions/integration_test.go
// +build integration

package transactions

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/temmyjay001/ledger-service/internal/events"
	"github.com/temmyjay001/ledger-service/internal/storage/queries"
	"github.com/temmyjay001/ledger-service/internal/testutil"
)

func TestIntegration_CreateSimpleTransaction(t *testing.T) {
	testutil.SkipIfShort(t)
	
	// Setup
	db := testutil.SetupTestDB(t)
	tenantSlug := testutil.RandomSlug()
	tenant := testutil.CreateTestTenant(t, db, tenantSlug)
	
	t.Cleanup(func() {
		testutil.CleanupTestTenant(t, db, tenantSlug)
	})

	// Create test accounts
	cashAccount := testutil.CreateTestAccount(t, db, tenantSlug, "1000", "Cash", queries.AccountTypeEnumAsset)
	revenueAccount := testutil.CreateTestAccount(t, db, tenantSlug, "4000", "Revenue", queries.AccountTypeEnumRevenue)

	// Create services
	eventService := events.NewService(db)
	service := NewService(db, eventService)

	// Test: Create a simple transaction (debit cash, credit revenue)
	req := CreateTransactionRequest{
		IdempotencyKey: "test-idem-" + testutil.RandomString(10),
		Description:    "Test revenue transaction",
		Reference:      "TEST-001",
		AccountCode:    cashAccount.Code,
		Amount:         decimal.NewFromInt(1000),
		Side:           "debit",
		Currency:       "NGN",
	}

	ctx := context.Background()
	response, err := service.CreateSimpleTransaction(ctx, tenantSlug, req)

	// Assertions
	require.NoError(t, err)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, req.IdempotencyKey, response.IdempotencyKey)
	assert.Equal(t, req.Description, response.Description)
	assert.Equal(t, "posted", response.Status)

	// Verify balance was updated
	testutil.AssertAccountBalance(t, db, tenantSlug, cashAccount.ID, "NGN", decimal.NewFromInt(1000))

	// Test: Idempotency - creating same transaction again should return existing
	response2, err := service.CreateSimpleTransaction(ctx, tenantSlug, req)
	require.NoError(t, err)
	assert.Equal(t, response.ID, response2.ID, "Should return same transaction for duplicate idempotency key")

	// Verify events were created
	err = db.SetSearchPath(ctx, "public")
	require.NoError(t, err)
	
	events, err := db.Queries.GetEventsByAggregate(ctx, queries.GetEventsByAggregateParams{
		TenantID:    tenant.ID,
		AggregateID: testutil.MustParseUUID(response.ID),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, events, "Should have created event records")
}

func TestIntegration_ListTransactionsByDateRange(t *testing.T) {
	testutil.SkipIfShort(t)

	// Setup
	db := testutil.SetupTestDB(t)
	tenantSlug := testutil.RandomSlug()
	testutil.CreateTestTenant(t, db, tenantSlug)
	
	t.Cleanup(func() {
		testutil.CleanupTestTenant(t, db, tenantSlug)
	})

	// Create test account
	cashAccount := testutil.CreateTestAccount(t, db, tenantSlug, "1000", "Cash", queries.AccountTypeEnumAsset)

	// Create services
	eventService := events.NewService(db)
	service := NewService(db, eventService)

	ctx := context.Background()

	// Create transactions over different dates
	baseTime := time.Now().Add(-72 * time.Hour) // 3 days ago

	for i := 0; i < 5; i++ {
		req := CreateTransactionRequest{
			IdempotencyKey: testutil.RandomString(20),
			Description:    fmt.Sprintf("Transaction %d", i+1),
			AccountCode:    cashAccount.Code,
			Amount:         decimal.NewFromInt(int64((i + 1) * 100)),
			Side:           "debit",
			Currency:       "NGN",
		}

		_, err := service.CreateSimpleTransaction(ctx, tenantSlug, req)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
	}

	// Query by date range
	startDate := baseTime.Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")

	listReq := ListTransactionsRequest{
		Limit:     10,
		Offset:    0,
		StartDate: startDate,
		EndDate:   endDate,
	}

	response, err := service.ListTransactions(ctx, tenantSlug, listReq)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(response.Transactions), 5)
}

func TestIntegration_CreateDoubleEntryTransaction(t *testing.T) {
	testutil.SkipIfShort(t)

	// Setup
	db := testutil.SetupTestDB(t)
	tenantSlug := testutil.RandomSlug()
	tenant := testutil.CreateTestTenant(t, db, tenantSlug)
	
	t.Cleanup(func() {
		testutil.CleanupTestTenant(t, db, tenantSlug)
	})

	// Create test accounts
	cashAccount := testutil.CreateTestAccount(t, db, tenantSlug, "1000", "Cash", queries.AccountTypeEnumAsset)
	inventoryAccount := testutil.CreateTestAccount(t, db, tenantSlug, "1200", "Inventory", queries.AccountTypeEnumAsset)
	payableAccount := testutil.CreateTestAccount(t, db, tenantSlug, "2000", "Accounts Payable", queries.AccountTypeEnumLiability)

	// Create services
	eventService := events.NewService(db)
	service := NewService(db, eventService)

	// Test: Purchase inventory on credit
	// Debit: Inventory 5000
	// Credit: Accounts Payable 4500
	// Credit: Cash 500 (down payment)
	req := CreateDoubleEntryRequest{
		IdempotencyKey: "test-de-" + testutil.RandomString(10),
		Description:    "Purchase inventory",
		Reference:      "PO-001",
		Entries: []TransactionLineEntry{
			{
				AccountCode: inventoryAccount.Code,
				Amount:      decimal.NewFromInt(5000),
				Side:        "debit",
				Currency:    "NGN",
			},
			{
				AccountCode: payableAccount.Code,
				Amount:      decimal.NewFromInt(4500),
				Side:        "credit",
				Currency:    "NGN",
			},
			{
				AccountCode: cashAccount.Code,
				Amount:      decimal.NewFromInt(500),
				Side:        "credit",
				Currency:    "NGN",
			},
		},
	}

	ctx := context.Background()
	response, err := service.CreateDoubleEntryTransaction(ctx, tenantSlug, req)

	// Assertions
	require.NoError(t, err)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, "posted", response.Status)

	// Verify all balances were updated correctly
	testutil.AssertAccountBalance(t, db, tenantSlug, inventoryAccount.ID, "NGN", decimal.NewFromInt(5000))
	testutil.AssertAccountBalance(t, db, tenantSlug, payableAccount.ID, "NGN", decimal.NewFromInt(4500))
	testutil.AssertAccountBalance(t, db, tenantSlug, cashAccount.ID, "NGN", decimal.NewFromInt(-500))
}

func TestIntegration_UnbalancedTransactionFails(t *testing.T) {
	testutil.SkipIfShort(t)

	// Setup
	db := testutil.SetupTestDB(t)
	tenantSlug := testutil.RandomSlug()
	testutil.CreateTestTenant(t, db, tenantSlug)
	
	t.Cleanup(func() {
		testutil.CleanupTestTenant(t, db, tenantSlug)
	})

	// Create test accounts
	cashAccount := testutil.CreateTestAccount(t, db, tenantSlug, "1000", "Cash", queries.AccountTypeEnumAsset)
	revenueAccount := testutil.CreateTestAccount(t, db, tenantSlug, "4000", "Revenue", queries.AccountTypeEnumRevenue)

	// Create services
	eventService := events.NewService(db)
	service := NewService(db, eventService)

	// Test: Unbalanced transaction should fail
	req := CreateDoubleEntryRequest{
		IdempotencyKey: "test-unbalanced-" + testutil.RandomString(10),
		Description:    "Unbalanced transaction",
		Entries: []TransactionLineEntry{
			{
				AccountCode: cashAccount.Code,
				Amount:      decimal.NewFromInt(1000),
				Side:        "debit",
				Currency:    "NGN",
			},
			{
				AccountCode: revenueAccount.Code,
				Amount:      decimal.NewFromInt(900), // Doesn't balance!
				Side:        "credit",
				Currency:    "NGN",
			},
		},
	}

	ctx := context.Background()
	_, err := service.CreateDoubleEntryTransaction(ctx, tenantSlug, req)

	// Should fail with unbalanced error
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnbalancedTransaction)

	// Verify no balances were changed
	testutil.AssertAccountBalance(t, db, tenantSlug, cashAccount.ID, "NGN", decimal.Zero)
	testutil.AssertAccountBalance(t, db, tenantSlug, revenueAccount.ID, "NGN", decimal.Zero)
}

func TestIntegration_ConcurrentTransactions(t *testing.T) {
	testutil.SkipIfShort(t)

	// Setup
	db := testutil.SetupTestDB(t)
	tenantSlug := testutil.RandomSlug()
	testutil.CreateTestTenant(t, db, tenantSlug)
	
	t.Cleanup(func() {
		testutil.CleanupTestTenant(t, db, tenantSlug)
	})

	// Create test account
	cashAccount := testutil.CreateTestAccount(t, db, tenantSlug, "1000", "Cash", queries.AccountTypeEnumAsset)

	// Create services
	eventService := events.NewService(db)
	service := NewService(db, eventService)

	// Run multiple concurrent transactions
	numTransactions := 10
	amount := decimal.NewFromInt(100)

	done := make(chan error, numTransactions)

	for i := 0; i < numTransactions; i++ {
		go func(index int) {
			req := CreateTransactionRequest{
				IdempotencyKey: testutil.RandomString(20),
				Description:    "Concurrent transaction",
				AccountCode:    cashAccount.Code,
				Amount:         amount,
				Side:           "debit",
				Currency:       "NGN",
			}

			ctx := context.Background()
			_, err := service.CreateSimpleTransaction(ctx, tenantSlug, req)
			done <- err
		}(i)
	}

	// Wait for all transactions to complete
	for i := 0; i < numTransactions; i++ {
		err := <-done
		require.NoError(t, err)
	}

	// Verify final balance is correct
	expectedBalance := amount.Mul(decimal.NewFromInt(int64(numTransactions)))
	testutil.AssertAccountBalance(t, db, tenantSlug, cashAccount.ID, "NGN", expectedBalance)
}

func TestIntegration_TransactionHistory(t *testing.T) {
	testutil.SkipIfShort(t)

	// Setup
	db := testutil.SetupTestDB(t)
	tenantSlug := testutil.RandomSlug()
	testutil.CreateTestTenant(t, db, tenantSlug)
	
	t.Cleanup(func() {
		testutil.CleanupTestTenant(t, db, tenantSlug)
	})

	// Create test account
	cashAccount := testutil.CreateTestAccount(t, db, tenantSlug, "1000", "Cash", queries.AccountTypeEnumAsset)

	// Create services
	eventService := events.NewService(db)
	service := NewService(db, eventService)

	// Create multiple transactions
	ctx := context.Background()
	numTransactions := 5

	for i := 0; i < numTransactions; i++ {
		req := CreateTransactionRequest{
			IdempotencyKey: testutil.RandomString(20),
			Description:    "Transaction " + string(rune(i+1)),
			AccountCode:    cashAccount.Code,
			Amount:         decimal.NewFromInt(int64((i + 1) * 100)),
			Side:           "debit",
			Currency:       "NGN",
		}

		_, err := service.CreateSimpleTransaction(ctx, tenantSlug, req)
		require.NoError(t, err)

		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// List transactions
	listReq := ListTransactionsRequest{
		Limit:       10,
		Offset:      0,
		AccountCode: cashAccount.Code,
	}

	response, err := service.ListTransactions(ctx, tenantSlug, listReq)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(response.Transactions), numTransactions)

	// Verify transactions are ordered by creation time (desc)
	for i := 0; i < len(response.Transactions)-1; i++ {
		assert.True(t, 
			response.Transactions[i].CreatedAt.After(response.Transactions[i+1].CreatedAt) ||
			response.Transactions[i].CreatedAt.Equal(response.Transactions[i+1].CreatedAt),
			"Transactions should be ordered by creation time descending")
	}
}