-- Fintech Ledger Service - Global Tables Migration
-- Creates shared tables across all tenants

-- ============================================================================
-- GLOBAL TABLES (shared across all tenants)
-- ============================================================================

-- Global event stream for event sourcing
CREATE TABLE IF NOT EXISTS events (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    aggregate_id UUID NOT NULL, -- transaction_id or account_id
    aggregate_type TEXT NOT NULL,
    event_type TEXT NOT NULL,
    event_version INTEGER NOT NULL DEFAULT 1,
    event_data JSONB NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    sequence_number BIGSERIAL
);

-- Optimize for event stream queries
CREATE INDEX idx_events_tenant_aggregate ON events(tenant_id, aggregate_id);
CREATE INDEX idx_events_sequence ON events(sequence_number);
CREATE INDEX idx_events_created_at ON events(created_at);
CREATE INDEX idx_events_tenant_type ON events(tenant_id, event_type);

-- Webhook delivery tracking
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    event_id UUID NOT NULL REFERENCES events(event_id),
    url TEXT NOT NULL,
    status_code INTEGER,
    attempts INTEGER DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_webhook_deliveries_tenant ON webhook_deliveries(tenant_id);
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status_code);

-- Tenant management with authentication
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    schema_name TEXT UNIQUE NOT NULL,
    api_key_hash TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    compliance_tier TEXT DEFAULT 'basic', -- basic, standard, enterprise
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_tenants_schema ON tenants(schema_name);
CREATE INDEX idx_tenants_active ON tenants(is_active);

-- Global constraints
ALTER TABLE webhook_deliveries ADD CONSTRAINT fk_webhook_tenant 
    FOREIGN KEY (tenant_id) REFERENCES tenants(id);