#!/bin/bash
set -e

# Tenant creation script

DB_URL=${DATABASE_URL:-"postgres://postgres:password@localhost:5432/ledger_service?sslmode=disable"}

# Get tenant details
read -p "Enter tenant name: " TENANT_NAME
read -p "Enter tenant schema (e.g., tenant_abc123): " TENANT_SCHEMA

# Generate a temporary API key hash (replace with proper hashing in production)
API_KEY_HASH=$(echo -n "temp_key_$TENANT_SCHEMA" | sha256sum | cut -d' ' -f1)

echo "Creating tenant: $TENANT_NAME with schema: $TENANT_SCHEMA"

# Create tenant record and schema
psql "$DB_URL" << EOF
INSERT INTO tenants (name, schema_name, api_key_hash) 
VALUES ('$TENANT_NAME', '$TENANT_SCHEMA', '$API_KEY_HASH');

SELECT create_tenant_schema('$TENANT_SCHEMA');
EOF

echo "Tenant created successfully!"
echo "Schema: $TENANT_SCHEMA"
echo "API Key Hash: $API_KEY_HASH"