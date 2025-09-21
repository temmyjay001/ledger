#!/bin/bash
set -e

# Database setup script for Fintech Ledger Service

DB_URL=${DATABASE_URL:-"postgres://postgres:password@localhost:5432/ledger_service?sslmode=disable"}

echo "Setting up Fintech Ledger Service database..."

# Run migrations in order
echo "Creating global tables..."
psql "$DB_URL" -f internal/database/migrations/001_global_tables.sql

echo "Setting up tenant schema functions..."
psql "$DB_URL" -f internal/database/migrations/002_tenant_schema.sql

echo "Database setup complete!"
echo "To create a tenant, run: make create-tenant"