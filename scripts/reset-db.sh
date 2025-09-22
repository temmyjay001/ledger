#!/bin/bash
# scripts/reset-db.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Database configuration
DB_USER=$(whoami)
DB_NAME_DEV="ledger_dev"

echo -e "${RED}WARNING: This will completely reset the development database!${NC}"
echo -e "${YELLOW}Database: $DB_NAME_DEV${NC}"
echo ""
read -p "Are you sure you want to continue? (y/N): " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Operation cancelled"
    exit 0
fi

echo -e "${YELLOW}Resetting database: $DB_NAME_DEV${NC}"

# Drop and recreate database
dropdb --if-exists $DB_NAME_DEV
createdb $DB_NAME_DEV

# Recreate extensions
echo -e "${YELLOW}Setting up database extensions...${NC}"
psql $DB_NAME_DEV -c "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";" >/dev/null
psql $DB_NAME_DEV -c "CREATE EXTENSION IF NOT EXISTS \"btree_gin\";" >/dev/null

echo -e "${GREEN}✓ Database reset completed successfully!${NC}"
echo ""
echo "Next steps:"
echo "1. Run 'make migrate-up' to apply migrations"
echo "2. Run 'make dev' to start the development server"