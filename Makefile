# Ledger Service Makefile

.PHONY: help dev build test clean setup-db reset-db migrate-up migrate-down sqlc

# Default environment
ENV ?= development

# Database settings
DB_NAME_DEV = ledger_dev
DB_NAME_TEST = ledger_test
DB_USER = $(shell whoami)
DATABASE_URL = postgres://$(DB_USER):postgres@localhost:5432/$(DB_NAME_DEV)?sslmode=disable
TEST_DATABASE_URL = postgres://$(DB_USER)@localhost:5432/$(DB_NAME_TEST)?sslmode=disable

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev: ## Start development server with hot reload
	@echo "Starting development server..."
	$(shell go env GOPATH)/bin/air

build: ## Build the application
	@echo "Building application..."
	go build -o bin/ledger-server cmd/server/main.go

test: ## Run tests
	@echo "Running tests..."
	ENV=test go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	ENV=test go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

setup-db: ## Setup development and test databases
	@echo "Setting up databases..."
	@./scripts/setup-db.sh

reset-db: ## Reset development database (WARNING: destructive)
	@echo "Resetting development database..."
	@./scripts/reset-db.sh

migrate-up: ## Run database migrations up
	@echo "Running migrations up..."
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down: ## Run database migrations down
	@echo "Running migrations down..."
	migrate -path migrations -database "$(DATABASE_URL)" down

migrate-create: ## Create new migration (usage: make migrate-create name=migration_name)
	@echo "Creating migration: $(name)"
	migrate create -ext sql -dir migrations $(name)

sqlc: ## Generate sqlc code
	@echo "Generating sqlc code..."
	sqlc generate

lint: ## Run linters
	@echo "Running linters..."
	golangci-lint run

format: ## Format code
	@echo "Formatting code..."
	gofmt -s -w .
	goimports -w .

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t ledger-service .

# Integration test with fresh database
test-integration: ## Run integration tests with fresh test database
	@echo "Setting up test database..."
	@ENV=test ./scripts/setup-db.sh
	@echo "Running integration tests..."
	ENV=test go test -v -tags=integration ./...

# Database connection test
db-ping: ## Test database connection
	@echo "Testing database connection..."
	@psql $(DATABASE_URL) -c "SELECT 1;" > /dev/null && echo "✓ Database connection successful" || echo "✗ Database connection failed"

# Show database URL for debugging
db-url: ## Show database URL
	@echo "Development DB: $(DATABASE_URL)"
	@echo "Test DB: $(TEST_DATABASE_URL)"