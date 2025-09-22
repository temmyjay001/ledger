#!/bin/bash
# scripts/test-db.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Database configuration
DB_USER=$(whoami)
DB_NAME_TEST="ledger_test"

echo -e "${YELLOW}Setting up test database: $DB_NAME_TEST${NC}"

# Drop and recreate test database for clean state
dropdb --if-exists $DB_NAME_TEST
createdb $DB_NAME_TEST

# Setup extensions
psql $DB_NAME_TEST -c "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";" >/dev/null
psql $DB_NAME_TEST -c "CREATE EXTENSION IF NOT EXISTS \"btree_gin\";" >/dev/null

# Run migrations on test database
TEST_DATABASE_URL="postgres://$DB_USER@localhost:5432/$DB_NAME_TEST?sslmode=disable"
migrate -path migrations -database "$TEST_DATABASE_URL" up

echo -e "${GREEN}✓ Test database setup completed!${NC}"