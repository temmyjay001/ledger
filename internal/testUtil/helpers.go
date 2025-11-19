// internal/testutil/helpers.go
package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/temmyjay001/ledger-service/internal/config"
	"github.com/temmyjay001/ledger-service/internal/storage"
	"github.com/temmyjay001/ledger-service/internal/storage/queries"
)

// TestDB creates and returns a test database connection
func SetupTestDB(t *testing.T) *storage.DB {
	cfg := &config.Config{
		DatabaseURL:            GetTestDatabaseURL(),
		DatabaseMaxConnections: 5,
		DatabaseMaxIdleTime:    time.Minute * 5,
		JWTSecret:             "test-jwt-secret",
		APIKeySecret:          "test-api-secret",
	}

	db, err := storage.NewPostgresDB(cfg)
	require.NoError(t, err, "Failed to connect to test database")

	// Clean up function
	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// GetTestDatabaseURL returns the test database URL from environment or default
func GetTestDatabaseURL() string {
	// Default test database URL
	return "postgres://postgres:postgres@localhost:5432/ledger_test?sslmode=disable"
}

// CreateTestTenant creates a tenant for testing
func CreateTestTenant(t *testing.T, db *storage.DB, slug string) queries.Tenant {
	ctx := context.Background()

	tenant, err := db.Queries.CreateTenant(ctx, queries.CreateTenantParams{
		Name:         "Test Tenant " + slug,
		Slug:         slug,
		BusinessType: pgtype.Text{String: "wallet", Valid: true},
		CountryCode:  pgtype.Text{String: "NG", Valid: true},
		BaseCurrency: pgtype.Text{String: "NGN", Valid: true},
		Timezone:     pgtype.Text{String: "Africa/Lagos", Valid: true},
	})
	require.NoError(t, err)

	// Create tenant schema
	_, err = db.Exec(ctx, "SELECT create_tenant_schema($1)", slug)
	require.NoError(t, err)

	return tenant
}

// CreateTestUser creates a user for testing
func CreateTestUser(t *testing.T, db *storage.DB, email string) queries.User {
	ctx := context.Background()

	user, err := db.Queries.CreateUser(ctx, queries.CreateUserParams{
		Email:        email,
		PasswordHash: "test-hash",
		FirstName:    "Test",
		LastName:     "User",
	})
	require.NoError(t, err)

	return user
}

// CreateTestAccount creates an account in a tenant schema for testing
func CreateTestAccount(t *testing.T, db *storage.DB, tenantSlug, code, name string, accountType queries.AccountTypeEnum) queries.Account {
	ctx := context.Background()

	// Set tenant schema
	err := db.SetSearchPath(ctx, "tenant_"+tenantSlug)
	require.NoError(t, err)
	defer db.SetSearchPath(ctx, "public")

	account, err := db.Queries.CreateAccount(ctx, queries.CreateAccountParams{
		Code:        code,
		Name:        name,
		AccountType: accountType,
		Currency:    "NGN",
		Metadata:    json.RawMessage("{}"),
	})
	require.NoError(t, err)

	// Create initial balance
	_, err = db.Queries.CreateAccountBalance(ctx, queries.CreateAccountBalanceParams{
		AccountID: account.ID,
		Currency:  "NGN",
		Balance:   decimal.Zero,
	})
	require.NoError(t, err)

	return account
}

// CreateTestTransaction creates a transaction for testing
func CreateTestTransaction(t *testing.T, db *storage.DB, tenantSlug string, idempotencyKey string) queries.Transaction {
	ctx := context.Background()

	// Set tenant schema
	err := db.SetSearchPath(ctx, "tenant_"+tenantSlug)
	require.NoError(t, err)
	defer db.SetSearchPath(ctx, "public")

	transaction, err := db.Queries.CreateTransaction(ctx, queries.CreateTransactionParams{
		IdempotencyKey: idempotencyKey,
		Description:    "Test transaction",
		Reference:      pgtype.Text{String: "TEST-REF", Valid: true},
		Metadata:       json.RawMessage("{}"),
	})
	require.NoError(t, err)

	return transaction
}

// CreateTestTransactionLine creates a transaction line for testing
func CreateTestTransactionLine(t *testing.T, db *storage.DB, tenantSlug string, transactionID, accountID uuid.UUID, amount decimal.Decimal, side queries.TransactionSideEnum) queries.TransactionLine {
	ctx := context.Background()

	// Set tenant schema
	err := db.SetSearchPath(ctx, "tenant_"+tenantSlug)
	require.NoError(t, err)
	defer db.SetSearchPath(ctx, "public")

	line, err := db.Queries.CreateTransactionLine(ctx, queries.CreateTransactionLineParams{
		TransactionID: transactionID,
		AccountID:     accountID,
		Amount:        amount,
		Side:          side,
		Currency:      "NGN",
		Metadata:      json.RawMessage("{}"),
	})
	require.NoError(t, err)

	return line
}

// AssertAccountBalance checks that an account has the expected balance
func AssertAccountBalance(t *testing.T, db *storage.DB, tenantSlug string, accountID uuid.UUID, currency string, expectedBalance decimal.Decimal) {
	ctx := context.Background()

	err := db.SetSearchPath(ctx, "tenant_"+tenantSlug)
	require.NoError(t, err)
	defer db.SetSearchPath(ctx, "public")

	balance, err := db.Queries.GetAccountBalance(ctx, queries.GetAccountBalanceParams{
		AccountID: accountID,
		Currency:  currency,
	})
	require.NoError(t, err)

	require.True(t, balance.Balance.Equal(expectedBalance),
		"Expected balance %s but got %s", expectedBalance.String(), balance.Balance.String())
}

// CleanupTestTenant removes a test tenant and its schema
func CleanupTestTenant(t *testing.T, db *storage.DB, slug string) {
	ctx := context.Background()

	// Drop schema
	_, err := db.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS tenant_%s CASCADE", slug))
	if err != nil {
		t.Logf("Warning: Failed to drop tenant schema: %v", err)
	}

	// Delete tenant record (this will cascade to related tables)
	_, err = db.Exec(ctx, "DELETE FROM tenants WHERE slug = $1", slug)
	if err != nil {
		t.Logf("Warning: Failed to delete tenant record: %v", err)
	}
}

// RandomString generates a random string for testing
func RandomString(length int) string {
	return uuid.New().String()[:length]
}

// RandomEmail generates a random email for testing
func RandomEmail() string {
	return fmt.Sprintf("test_%s@example.com", uuid.New().String()[:8])
}

// RandomSlug generates a random slug for testing
func RandomSlug() string {
	return fmt.Sprintf("test-%s", uuid.New().String()[:8])
}

// TestConfig returns a test configuration
func TestConfig() *config.Config {
	return &config.Config{
		Host:                   "localhost",
		Port:                   "8080",
		Env:                    "test",
		DatabaseURL:            GetTestDatabaseURL(),
		DatabaseMaxConnections: 5,
		DatabaseMaxIdleTime:    time.Minute * 5,
		RedisURL:              "redis://localhost:6379/1",
		JWTSecret:             "test-jwt-secret-key-for-testing",
		APIKeySecret:          "test-api-key-secret-for-testing",
		WebhookTimeout:        30 * time.Second,
		WebhookMaxRetries:     3,
	}
}

// Fixtures for common test data

// FixtureAssetAccount returns a fixture asset account
func FixtureAssetAccount() queries.CreateAccountParams {
	return queries.CreateAccountParams{
		Code:        "1000",
		Name:        "Cash",
		AccountType: queries.AccountTypeEnumAsset,
		Currency:    "NGN",
		Metadata:    json.RawMessage("{}"),
	}
}

// FixtureLiabilityAccount returns a fixture liability account
func FixtureLiabilityAccount() queries.CreateAccountParams {
	return queries.CreateAccountParams{
		Code:        "2000",
		Name:        "Customer Deposits",
		AccountType: queries.AccountTypeEnumLiability,
		Currency:    "NGN",
		Metadata:    json.RawMessage("{}"),
	}
}

// FixtureRevenueAccount returns a fixture revenue account
func FixtureRevenueAccount() queries.CreateAccountParams {
	return queries.CreateAccountParams{
		Code:        "4000",
		Name:        "Service Revenue",
		AccountType: queries.AccountTypeEnumRevenue,
		Currency:    "NGN",
		Metadata:    json.RawMessage("{}"),
	}
}

// FixtureExpenseAccount returns a fixture expense account
func FixtureExpenseAccount() queries.CreateAccountParams {
	return queries.CreateAccountParams{
		Code:        "5000",
		Name:        "Operating Expenses",
		AccountType: queries.AccountTypeEnumExpense,
		Currency:    "NGN",
		Metadata:    json.RawMessage("{}"),
	}
}

// SkipIfShort skips the test if running in short mode
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
}

// WaitForCondition waits for a condition to be true or times out
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Timeout waiting for condition: %s", message)
}

// MustParseUUID parses a UUID string and panics if invalid
func MustParseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic(fmt.Sprintf("invalid UUID: %s", s))
	}
	return id
}