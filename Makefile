.PHONY: build run test clean setup deps setup_db scripts-setup create-tenant

# Variables
BINARY_NAME=ledger-service
BUILD_DIR=bin

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build the application
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) cmd/server/main.go

# Run the application
run:
	go run cmd/server/main.go

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)/
	rm -f coverage.out

# Setup development environment
setup: deps
	cp .env.example .env
	@echo "Update .env file with your configuration"

# Lint code
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Development server with hot reload (requires air)
dev:
	air

# Database commands
setup-db:
	./scripts/setup_db.sh

create-tenant:
	./scripts/create_tenant.sh

# Make scripts executable
scripts-setup:
	chmod +x scripts/*.sh