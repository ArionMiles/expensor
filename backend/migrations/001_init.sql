-- Initial schema for Expensor.
-- This file is intentionally idempotent so the migration runner can safely
-- re-apply it if the schema_migrations record is lost.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id VARCHAR(255) UNIQUE NOT NULL,
    amount NUMERIC(19,4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'INR',
    original_amount NUMERIC(19,4),
    original_currency VARCHAR(3),
    exchange_rate NUMERIC(10,6),
    timestamp TIMESTAMPTZ NOT NULL,
    merchant_info TEXT NOT NULL,
    category VARCHAR(100),
    bucket VARCHAR(50),
    source VARCHAR(100) NOT NULL,
    source_type TEXT NOT NULL DEFAULT '',
    source_label TEXT NOT NULL DEFAULT '',
    bank TEXT NOT NULL DEFAULT '',
    description TEXT,
    metadata JSONB DEFAULT '{}',
    muted BOOLEAN NOT NULL DEFAULT false,
    muted_by_merchant BOOLEAN NOT NULL DEFAULT false,
    mute_reason TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS muted BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS muted_by_merchant BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS mute_reason TEXT,
    ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS transaction_labels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID REFERENCES transactions(id) ON DELETE CASCADE,
    label VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(transaction_id, label)
);

CREATE TABLE IF NOT EXISTS app_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO app_config (key, value)
VALUES ('active_reader', '')
ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS labels (
    name TEXT PRIMARY KEY,
    color TEXT NOT NULL DEFAULT '#6366f1',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS categories (
    name TEXT PRIMARY KEY,
    description TEXT,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS buckets (
    name TEXT PRIMARY KEY,
    description TEXT,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO categories (name, is_default) VALUES
    ('Food & Dining', true),
    ('Transport', true),
    ('Shopping', true),
    ('Utilities', true),
    ('Healthcare', true),
    ('Entertainment', true),
    ('Travel', true),
    ('Finance', true)
ON CONFLICT (name) DO NOTHING;

INSERT INTO buckets (name, is_default) VALUES
    ('Needs', true),
    ('Wants', true),
    ('Investments', true),
    ('Income', true)
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    sender_email TEXT NOT NULL DEFAULT '',
    sender_emails TEXT[] NOT NULL DEFAULT '{}',
    subject_contains TEXT NOT NULL DEFAULT '',
    amount_regex TEXT NOT NULL,
    merchant_regex TEXT NOT NULL,
    currency_regex TEXT NOT NULL DEFAULT '',
    transaction_source TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL DEFAULT '',
    source_label TEXT NOT NULL DEFAULT '',
    bank TEXT NOT NULL DEFAULT '',
    predefined BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE rules
    ADD COLUMN IF NOT EXISTS transaction_source TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sender_emails TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS predefined BOOLEAN NOT NULL DEFAULT false;

UPDATE transactions
SET source_label = COALESCE(NULLIF(source_label, ''), source)
WHERE COALESCE(source_label, '') = '';

UPDATE rules
SET sender_emails = ARRAY[sender_email]
WHERE cardinality(sender_emails) = 0 AND sender_email <> '';

UPDATE rules
SET source_label = transaction_source
WHERE source_label = '' AND transaction_source <> '';

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'rules' AND constraint_name = 'rules_name_key'
    ) THEN
        ALTER TABLE rules ADD CONSTRAINT rules_name_key UNIQUE (name);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS muted_merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pattern TEXT NOT NULL UNIQUE,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE muted_merchants
    ADD COLUMN IF NOT EXISTS reason TEXT;

CREATE TABLE IF NOT EXISTS mcc_codes (
    code TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    category TEXT NOT NULL,
    bucket TEXT NOT NULL DEFAULT 'Wants',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS merchant_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fragment TEXT NOT NULL UNIQUE,
    mcc_code TEXT REFERENCES mcc_codes(code) ON DELETE SET NULL,
    category TEXT,
    bucket TEXT,
    source TEXT NOT NULL DEFAULT 'community',
    user_locked BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS label_merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    label TEXT NOT NULL REFERENCES labels(name) ON DELETE CASCADE,
    merchant_pattern TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(label, merchant_pattern)
);

CREATE TABLE IF NOT EXISTS transaction_label_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    source_type TEXT NOT NULL,
    merchant_pattern TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (source_type IN ('manual', 'merchant')),
    UNIQUE(transaction_id, label, source_type, merchant_pattern)
);

INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
SELECT transaction_id, label, 'manual', ''
FROM transaction_labels
ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING;

CREATE TABLE IF NOT EXISTS extraction_diagnostics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'resolved', 'ignored')),
    reader TEXT NOT NULL,
    message_id TEXT,
    source TEXT NOT NULL DEFAULT '',
    sender TEXT NOT NULL DEFAULT '',
    sender_email TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    email_body TEXT NOT NULL DEFAULT '',
    received_at TIMESTAMPTZ,
    snippet TEXT NOT NULL DEFAULT '',
    rule_id UUID,
    rule_name TEXT NOT NULL DEFAULT '',
    amount_regex TEXT NOT NULL DEFAULT '',
    merchant_regex TEXT NOT NULL DEFAULT '',
    currency_regex TEXT NOT NULL DEFAULT '',
    failure_reasons TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS reader_runtime (
    reader TEXT PRIMARY KEY,
    client_secret JSONB,
    oauth_token JSONB,
    config JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS processed_messages (
    message_key TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_merchant ON transactions USING gin(to_tsvector('english', merchant_info));
CREATE INDEX IF NOT EXISTS idx_transactions_description ON transactions USING gin(to_tsvector('english', description));
CREATE INDEX IF NOT EXISTS idx_transactions_currency ON transactions(currency);
CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category);
CREATE INDEX IF NOT EXISTS idx_transactions_bucket ON transactions(bucket);
CREATE INDEX IF NOT EXISTS idx_transactions_muted ON transactions (muted) WHERE muted = true;
CREATE INDEX IF NOT EXISTS idx_transactions_muted_by_merchant ON transactions (muted_by_merchant) WHERE muted_by_merchant = true;
CREATE INDEX IF NOT EXISTS transactions_merchant_trgm_idx ON transactions USING GIN (merchant_info gin_trgm_ops);
CREATE INDEX IF NOT EXISTS transactions_description_trgm_idx ON transactions USING GIN (description gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_transaction_labels_label ON transaction_labels(label);
CREATE INDEX IF NOT EXISTS idx_transaction_labels_transaction_id ON transaction_labels(transaction_id);
CREATE INDEX IF NOT EXISTS idx_merchant_categories_fragment ON merchant_categories (lower(fragment));
CREATE INDEX IF NOT EXISTS extraction_diagnostics_status_created_idx ON extraction_diagnostics (status, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS extraction_diagnostics_open_unique
    ON extraction_diagnostics (reader, message_id, rule_name)
    WHERE status = 'open' AND message_id IS NOT NULL;

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS update_transactions_updated_at ON transactions;
CREATE TRIGGER update_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
