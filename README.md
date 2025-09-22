# Fintech Ledger Service

A distributed ledger service for financial transactions with multi-tenant support.

## Features

- Multi-tenant architecture with schema isolation
- Event sourcing for audit trails
- RESTful API with proper error handling
- Database migrations

## Quick Start

```bash
# Copy environment file
cp .env.example .env

# Setup database
make setup-db

# Install dependencies
go mod tidy

# Generate sqlc code
make sqlc

# Run migrations
make migrate-up

# Start development server with hot reload
make dev

# Run tests
make test

# Reset database (careful!)
make reset-db
```

## API Documentation

See [docs/api.md](docs/api.md) for detailed API documentation.