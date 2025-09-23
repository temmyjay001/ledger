-- migrations/20250922090121_initial_schema.up.sql

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "btree_gin";

-- Custom types
CREATE TYPE account_type_enum AS ENUM (
    'asset',
    'liability', 
    'equity',
    'revenue',
    'expense'
);

CREATE TYPE transaction_side_enum AS ENUM (
    'debit',
    'credit'
);

CREATE TYPE transaction_status_enum AS ENUM (
    'pending',
    'posted',
    'failed'
);

CREATE TYPE user_status_enum AS ENUM (
    'active',
    'suspended',
    'pending_verification'
);

CREATE TYPE user_role_enum AS ENUM (
    'admin',
    'developer',
    'readonly',
    'auditor'
);

-- =====================================================
-- GLOBAL TABLES (shared across all tenants)
-- =====================================================

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT NOT NULL UNIQUE,
    email_verified BOOLEAN DEFAULT false,
    password_hash TEXT NOT NULL,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    status user_status_enum DEFAULT 'pending_verification',
    last_login_at TIMESTAMPTZ,
    failed_login_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Tenants (customer organizations)
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE, -- URL-friendly identifier
    business_type TEXT, -- 'wallet', 'lending', 'remittance', etc.
    country_code CHAR(2) DEFAULT 'NG', -- ISO country code
    base_currency CHAR(3) DEFAULT 'NGN', -- ISO currency code
    timezone TEXT DEFAULT 'Africa/Lagos',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Tenant user memberships
CREATE TABLE tenant_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role user_role_enum NOT NULL,
    permissions JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(tenant_id, user_id)
);

-- API keys for tenant authentication
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL, -- 'production-backend', 'staging-analytics'
    key_hash TEXT NOT NULL UNIQUE, -- hashed API key
    key_prefix TEXT NOT NULL, -- first 8 chars for identification
    scopes TEXT[] NOT NULL DEFAULT '{}', -- ['transactions:write', 'balances:read']
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    CONSTRAINT api_keys_tenant_name_unique UNIQUE(tenant_id, name)
);

-- Global event store
CREATE TABLE events (
    event_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    aggregate_id UUID NOT NULL, -- transaction_id, account_id, etc.
    aggregate_type TEXT NOT NULL, -- 'transaction', 'account', 'balance'
    event_type TEXT NOT NULL, -- 'transaction_posted', 'balance_updated'
    event_version INTEGER NOT NULL DEFAULT 1,
    event_data JSONB NOT NULL,
    metadata JSONB DEFAULT '{}', -- correlation_id, user_id, source
    created_at TIMESTAMPTZ DEFAULT NOW(),
    sequence_number BIGSERIAL -- Global ordering
);

-- Webhook delivery tracking
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    event_id UUID NOT NULL REFERENCES events(event_id) ON DELETE CASCADE,
    webhook_url TEXT NOT NULL,
    http_status_code INTEGER,
    response_body TEXT,
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    next_retry_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- =====================================================
-- TENANT-SPECIFIC TABLE TEMPLATE
-- (These will be created dynamically per tenant)
-- =====================================================

-- Function to create tenant schema and tables
CREATE OR REPLACE FUNCTION create_tenant_schema(tenant_slug TEXT) 
RETURNS VOID AS $$
DECLARE
    schema_name TEXT := 'tenant_' || tenant_slug;
BEGIN
    -- Create schema
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);
    
    -- Create accounts table (referencing enums from public schema)
    EXECUTE format('
        CREATE TABLE %I.accounts (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            code TEXT NOT NULL UNIQUE,
            name TEXT NOT NULL,
            account_type public.account_type_enum NOT NULL,
            parent_id UUID REFERENCES %I.accounts(id),
            currency CHAR(3) NOT NULL DEFAULT ''NGN'',
            metadata JSONB DEFAULT ''{}'',
            is_active BOOLEAN DEFAULT true,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name, schema_name);
    
    -- Create transactions table
    EXECUTE format('
        CREATE TABLE %I.transactions (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            idempotency_key TEXT NOT NULL UNIQUE,
            description TEXT NOT NULL,
            reference TEXT,
            status public.transaction_status_enum DEFAULT ''pending'',
            posted_at TIMESTAMPTZ DEFAULT NOW(),
            metadata JSONB DEFAULT ''{}'',
            created_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name);
    
    -- Create transaction_lines table
    EXECUTE format('
        CREATE TABLE %I.transaction_lines (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            transaction_id UUID NOT NULL REFERENCES %I.transactions(id) ON DELETE CASCADE,
            account_id UUID NOT NULL REFERENCES %I.accounts(id),
            amount NUMERIC(20,4) NOT NULL CHECK (amount > 0),
            side public.transaction_side_enum NOT NULL,
            currency CHAR(3) NOT NULL,
            metadata JSONB DEFAULT ''{}'',
            created_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name, schema_name, schema_name);
    
    -- Create account_balances table
    EXECUTE format('
        CREATE TABLE %I.account_balances (
            account_id UUID PRIMARY KEY REFERENCES %I.accounts(id) ON DELETE CASCADE,
            currency CHAR(3) NOT NULL,
            balance NUMERIC(20,4) NOT NULL DEFAULT 0,
            version BIGINT NOT NULL DEFAULT 0,
            updated_at TIMESTAMPTZ DEFAULT NOW(),
            
            UNIQUE(account_id, currency)
        )', schema_name, schema_name);
    
    -- Create indexes for performance
    EXECUTE format('CREATE INDEX idx_%I_accounts_type ON %I.accounts(account_type)', 
                   replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX idx_%I_transactions_status ON %I.transactions(status)', 
                   replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX idx_%I_transactions_posted_at ON %I.transactions(posted_at)', 
                   replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX idx_%I_transaction_lines_account ON %I.transaction_lines(account_id)', 
                   replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX idx_%I_transaction_lines_transaction ON %I.transaction_lines(transaction_id)', 
                   replace(schema_name, '-', '_'), schema_name);
    
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- INDEXES FOR PERFORMANCE
-- =====================================================

-- Users
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);

-- Tenants
CREATE INDEX idx_tenants_slug ON tenants(slug);
CREATE INDEX idx_tenants_country ON tenants(country_code);

-- Tenant users
CREATE INDEX idx_tenant_users_tenant ON tenant_users(tenant_id);
CREATE INDEX idx_tenant_users_user ON tenant_users(user_id);

-- API keys
CREATE INDEX idx_api_keys_tenant ON api_keys(tenant_id);
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix);

-- Events (critical for performance)
CREATE INDEX idx_events_tenant_created ON events(tenant_id, created_at);
CREATE INDEX idx_events_aggregate ON events(aggregate_id, event_version);
CREATE INDEX idx_events_sequence ON events(sequence_number);
CREATE INDEX idx_events_type ON events(tenant_id, event_type);

-- Webhooks
CREATE INDEX idx_webhook_deliveries_tenant ON webhook_deliveries(tenant_id);
CREATE INDEX idx_webhook_deliveries_retry ON webhook_deliveries(next_retry_at) 
    WHERE next_retry_at IS NOT NULL;

-- =====================================================
-- FUNCTIONS AND TRIGGERS
-- =====================================================

-- Update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Add updated_at triggers
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tenants_updated_at 
    BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =====================================================
-- ROW LEVEL SECURITY (Future enhancement)
-- =====================================================

-- Enable RLS on sensitive tables (commented out for now)
-- ALTER TABLE events ENABLE ROW LEVEL SECURITY;
-- ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;