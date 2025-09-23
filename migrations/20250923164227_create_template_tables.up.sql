-- sql/migrations/20250923164227_create_template_tables.up.sql

-- Template tables for sqlc generation - these match tenant schema structure exactly
-- sqlc will use these for validation, but our app will use tenant schemas

CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    account_type account_type_enum NOT NULL,
    parent_id UUID REFERENCES accounts(id),
    currency CHAR(3) NOT NULL DEFAULT 'NGN',
    metadata JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    idempotency_key TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    reference TEXT,
    status transaction_status_enum DEFAULT 'pending',
    posted_at TIMESTAMPTZ DEFAULT NOW(),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS transaction_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id),
    amount NUMERIC(20,4) NOT NULL CHECK (amount > 0),
    side transaction_side_enum NOT NULL,
    currency CHAR(3) NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS account_balances (
    account_id UUID PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    currency CHAR(3) NOT NULL,
    balance NUMERIC(20,4) NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(account_id, currency)
);

-- Create indexes to match tenant schemas
CREATE INDEX IF NOT EXISTS idx_accounts_type ON accounts(account_type);
CREATE INDEX IF NOT EXISTS idx_accounts_active ON accounts(is_active);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_transactions_posted_at ON transactions(posted_at);
CREATE INDEX IF NOT EXISTS idx_transaction_lines_account ON transaction_lines(account_id);
CREATE INDEX IF NOT EXISTS idx_transaction_lines_transaction ON transaction_lines(transaction_id);

-- Add comments so we know these are templates
COMMENT ON TABLE accounts IS 'Template table for sqlc generation - actual data is in tenant schemas';
COMMENT ON TABLE transactions IS 'Template table for sqlc generation - actual data is in tenant schemas';
COMMENT ON TABLE transaction_lines IS 'Template table for sqlc generation - actual data is in tenant schemas';
COMMENT ON TABLE account_balances IS 'Template table for sqlc generation - actual data is in tenant schemas';