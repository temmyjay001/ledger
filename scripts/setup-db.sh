#!/bin/bash
# scripts/setup-db.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Database configuration
DB_USER=$(whoami)
DB_NAME_DEV="ledger_dev"
DB_NAME_TEST="ledger_test"

echo -e "${YELLOW}Setting up databases for ledger service...${NC}"

# Check if PostgreSQL is running
if ! pg_isready >/dev/null 2>&1; then
    echo -e "${RED}Error: PostgreSQL is not running${NC}"
    echo "Please start PostgreSQL and try again"
    exit 1
fi

# Function to create database if it doesn't exist
create_database() {
    local db_name=$1
    echo -e "${YELLOW}Checking database: $db_name${NC}"
    
    if psql -lqt | cut -d \| -f 1 | grep -qw $db_name; then
        echo -e "${GREEN}Database $db_name already exists${NC}"
    else
        echo -e "${YELLOW}Creating database: $db_name${NC}"
        createdb $db_name
        echo -e "${GREEN}Database $db_name created successfully${NC}"
    fi
}

# Create development database
create_database $DB_NAME_DEV

# Create test database
create_database $DB_NAME_TEST

# Create necessary extensions
echo -e "${YELLOW}Setting up database extensions...${NC}"

for db in $DB_NAME_DEV $DB_NAME_TEST; do
    echo "Setting up extensions for $db"
    psql $db -c "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";" >/dev/null
    psql $db -c "CREATE EXTENSION IF NOT EXISTS \"btree_gin\";" >/dev/null
done

echo -e "${GREEN}✓ Database setup completed successfully!${NC}"
echo ""
echo "Connection strings:"
echo "Development: postgres://$DB_USER@localhost:5432/$DB_NAME_DEV?sslmode=disable"
echo "Test:        postgres://$DB_USER@localhost:5432/$DB_NAME_TEST?sslmode=disable"
echo ""
echo "Next steps:"
echo "1. Copy .env.example to .env and update DATABASE_URL"
echo "2. Run 'make migrate-up' to apply migrations"
echo "3. Run 'make dev' to start the development server"