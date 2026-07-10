-- Tenant-scope private Expensor data while preserving NULL-tenant legacy rows.
-- The temporary legacy NULL tenant path is intentionally removable after existing installs migrate.

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE app_config ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE labels ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE categories ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE rules ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE muted_merchants ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE merchant_categories ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE label_merchants ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE extraction_diagnostics ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE label_merchants DROP CONSTRAINT IF EXISTS label_merchants_label_fkey;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_message_id_key;
ALTER TABLE app_config DROP CONSTRAINT IF EXISTS app_config_pkey;
ALTER TABLE labels DROP CONSTRAINT IF EXISTS labels_pkey;
ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_pkey;
ALTER TABLE buckets DROP CONSTRAINT IF EXISTS buckets_pkey;
ALTER TABLE rules DROP CONSTRAINT IF EXISTS rules_name_key;
ALTER TABLE muted_merchants DROP CONSTRAINT IF EXISTS muted_merchants_pattern_key;
ALTER TABLE merchant_categories DROP CONSTRAINT IF EXISTS merchant_categories_fragment_key;

DROP INDEX IF EXISTS extraction_diagnostics_open_unique;

CREATE UNIQUE INDEX IF NOT EXISTS transactions_legacy_message_id_key
    ON transactions (message_id)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS transactions_tenant_message_id_key
    ON transactions (tenant_id, message_id)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS app_config_legacy_key
    ON app_config (key)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS app_config_tenant_key
    ON app_config (tenant_id, key)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS labels_legacy_name_key
    ON labels (name)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS labels_tenant_name_key
    ON labels (tenant_id, name)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS categories_legacy_name_key
    ON categories (name)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS categories_tenant_name_key
    ON categories (tenant_id, name)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS buckets_legacy_name_key
    ON buckets (name)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS buckets_tenant_name_key
    ON buckets (tenant_id, name)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS rules_predefined_name_key
    ON rules (name)
    WHERE tenant_id IS NULL AND predefined = true;

CREATE UNIQUE INDEX IF NOT EXISTS rules_legacy_user_name_key
    ON rules (name)
    WHERE tenant_id IS NULL AND predefined = false;

CREATE UNIQUE INDEX IF NOT EXISTS rules_tenant_user_name_key
    ON rules (tenant_id, name)
    WHERE tenant_id IS NOT NULL AND predefined = false;

CREATE UNIQUE INDEX IF NOT EXISTS muted_merchants_legacy_pattern_key
    ON muted_merchants (pattern)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS muted_merchants_tenant_pattern_key
    ON muted_merchants (tenant_id, pattern)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS merchant_categories_legacy_fragment_key
    ON merchant_categories (fragment)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS merchant_categories_tenant_fragment_key
    ON merchant_categories (tenant_id, fragment)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS label_merchants_legacy_mapping_key
    ON label_merchants (label, merchant_pattern)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS label_merchants_tenant_mapping_key
    ON label_merchants (tenant_id, label, merchant_pattern)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS extraction_diagnostics_open_legacy_unique
    ON extraction_diagnostics (reader, message_id, rule_name)
    WHERE tenant_id IS NULL AND status = 'open' AND message_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS extraction_diagnostics_open_tenant_unique
    ON extraction_diagnostics (tenant_id, reader, message_id, rule_name)
    WHERE tenant_id IS NOT NULL AND status = 'open' AND message_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_transactions_tenant_timestamp ON transactions (tenant_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_tenant_category ON transactions (tenant_id, category);
CREATE INDEX IF NOT EXISTS idx_transactions_tenant_bucket ON transactions (tenant_id, bucket);
CREATE INDEX IF NOT EXISTS idx_extraction_diagnostics_tenant_status_created
    ON extraction_diagnostics (tenant_id, status, created_at DESC);
