-- Fintech Ledger Service - Tenant Schema Migration
-- Creates schema-per-tenant structure and functions

-- ============================================================================
-- ENUMS FOR TENANT SCHEMAS
-- ============================================================================

-- Account types enum for double-entry accounting
CREATE TYPE account_type AS ENUM ('asset', 'liability', 'equity', 'revenue', 'expense');
CREATE TYPE transaction_side AS ENUM ('debit', 'credit');
CREATE TYPE transaction_status AS ENUM ('pending', 'posted', 'failed');

-- ============================================================================
-- TENANT SCHEMA CREATION FUNCTION
-- ============================================================================

-- Function to create a new tenant schema with all required tables
CREATE OR REPLACE FUNCTION create_tenant_schema(tenant_schema TEXT)
RETURNS VOID AS $$
BEGIN
    -- Create the schema
    EXECUTE format('CREATE SCHEMA %I', tenant_schema);
    
    -- Create accounts table
    EXECUTE format('
        CREATE TABLE %I.accounts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            code TEXT NOT NULL,
            name TEXT NOT NULL,
            account_type account_type NOT NULL,
            currency CHAR(3) DEFAULT ''NGN'',
            metadata JSONB DEFAULT ''{}''::jsonb,
            is_active BOOLEAN DEFAULT true,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
            
            CONSTRAINT unique_account_code UNIQUE(code)
        );
    ', tenant_schema);

    -- Create transactions table
    EXECUTE format('
        CREATE TABLE %I.transactions (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            idempotency_key TEXT UNIQUE NOT NULL,
            description TEXT NOT NULL,
            reference TEXT,
            status transaction_status DEFAULT ''pending'',
            posted_at TIMESTAMPTZ,
            metadata JSONB DEFAULT ''{}''::jsonb,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW()
        );
    ', tenant_schema);

    -- Create transaction_lines table
    EXECUTE format('
        CREATE TABLE %I.transaction_lines (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            transaction_id UUID NOT NULL REFERENCES %I.transactions(id),
            account_id UUID NOT NULL REFERENCES %I.accounts(id),
            amount NUMERIC(20,4) NOT NULL CHECK (amount > 0),
            side transaction_side NOT NULL,
            currency CHAR(3) DEFAULT ''NGN'',
            metadata JSONB DEFAULT ''{}''::jsonb,
            created_at TIMESTAMPTZ DEFAULT NOW()
        );
    ', tenant_schema, tenant_schema, tenant_schema);

    -- Create account_balances table
    EXECUTE format('
        CREATE TABLE %I.account_balances (
            account_id UUID PRIMARY KEY REFERENCES %I.accounts(id),
            balance NUMERIC(20,4) DEFAULT 0,
            version BIGINT DEFAULT 1,
            updated_at TIMESTAMPTZ DEFAULT NOW()
        );
    ', tenant_schema, tenant_schema);

    -- Create CBN compliance tracking table
    EXECUTE format('
        CREATE TABLE %I.cbr_reports (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            report_type TEXT NOT NULL,
            report_date DATE NOT NULL,
            data JSONB NOT NULL,
            submitted_at TIMESTAMPTZ,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            
            CONSTRAINT unique_report_date UNIQUE(report_type, report_date)
        );
    ', tenant_schema);

    -- Create indexes for performance
    EXECUTE format('CREATE INDEX idx_%I_accounts_type ON %I.accounts(account_type)', 
        replace(tenant_schema, '-', '_'), tenant_schema);
    EXECUTE format('CREATE INDEX idx_%I_accounts_currency ON %I.accounts(currency)', 
        replace(tenant_schema, '-', '_'), tenant_schema);
    EXECUTE format('CREATE INDEX idx_%I_accounts_code ON %I.accounts(code)', 
        replace(tenant_schema, '-', '_'), tenant_schema);
    
    EXECUTE format('CREATE INDEX idx_%I_transactions_status ON %I.transactions(status)', 
        replace(tenant_schema, '-', '_'), tenant_schema);
    EXECUTE format('CREATE INDEX idx_%I_transactions_posted_at ON %I.transactions(posted_at)', 
        replace(tenant_schema, '-', '_'), tenant_schema);
    EXECUTE format('CREATE INDEX idx_%I_transactions_reference ON %I.transactions(reference)', 
        replace(tenant_schema, '-', '_'), tenant_schema);
    
    EXECUTE format('CREATE INDEX idx_%I_transaction_lines_transaction ON %I.transaction_lines(transaction_id)', 
        replace(tenant_schema, '-', '_'), tenant_schema);
    EXECUTE format('CREATE INDEX idx_%I_transaction_lines_account ON %I.transaction_lines(account_id)', 
        replace(tenant_schema, '-', '_'), tenant_schema);
    EXECUTE format('CREATE INDEX idx_%I_transaction_lines_side ON %I.transaction_lines(side)', 
        replace(tenant_schema, '-', '_'), tenant_schema);

    -- Insert default accounts for fintech operations
    EXECUTE format('
        INSERT INTO %I.accounts (code, name, account_type, currency) VALUES
        (''cash'', ''Cash'', ''asset'', ''NGN''),
        (''customer_deposits'', ''Customer Deposits'', ''liability'', ''NGN''),
        (''revenue'', ''Revenue'', ''revenue'', ''NGN''),
        (''fees_earned'', ''Fees Earned'', ''revenue'', ''NGN''),
        (''operational_expenses'', ''Operational Expenses'', ''expense'', ''NGN''),
        (''float_account'', ''Float Account'', ''asset'', ''NGN''),
        (''settlement_account'', ''Settlement Account'', ''asset'', ''NGN'');
    ', tenant_schema);

    -- Initialize balances for default accounts
    EXECUTE format('
        INSERT INTO %I.account_balances (account_id, balance, version)
        SELECT id, 0, 1 FROM %I.accounts;
    ', tenant_schema, tenant_schema);

END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- HELPER FUNCTIONS
-- ============================================================================

-- Function to drop a tenant schema (for cleanup/testing)
CREATE OR REPLACE FUNCTION drop_tenant_schema(tenant_schema TEXT)
RETURNS VOID AS $$
BEGIN
    EXECUTE format('DROP SCHEMA IF EXISTS %I CASCADE', tenant_schema);
END;
$$ LANGUAGE plpgsql;

-- Function to list all tenant schemas
CREATE OR REPLACE FUNCTION list_tenant_schemas()
RETURNS TABLE(schema_name TEXT, tenant_id UUID, tenant_name TEXT) AS $$
BEGIN
    RETURN QUERY
    SELECT t.schema_name, t.id, t.name
    FROM tenants t
    WHERE t.is_active = true
    ORDER BY t.created_at;
END;
$$ LANGUAGE plpgsql;