-- Best-effort rollback for tenant-scoped private data.
-- Tenant-specific rows are removed before global uniqueness is restored.

DELETE FROM extraction_diagnostics WHERE tenant_id IS NOT NULL;
DELETE FROM label_merchants WHERE tenant_id IS NOT NULL;
DELETE FROM merchant_categories WHERE tenant_id IS NOT NULL;
DELETE FROM muted_merchants WHERE tenant_id IS NOT NULL;
DELETE FROM rules WHERE tenant_id IS NOT NULL;
DELETE FROM buckets WHERE tenant_id IS NOT NULL;
DELETE FROM categories WHERE tenant_id IS NOT NULL;
DELETE FROM labels WHERE tenant_id IS NOT NULL;
DELETE FROM app_config WHERE tenant_id IS NOT NULL;
DELETE FROM transactions WHERE tenant_id IS NOT NULL;

DROP INDEX IF EXISTS idx_extraction_diagnostics_tenant_status_created;
DROP INDEX IF EXISTS idx_transactions_tenant_bucket;
DROP INDEX IF EXISTS idx_transactions_tenant_category;
DROP INDEX IF EXISTS idx_transactions_tenant_timestamp;
DROP INDEX IF EXISTS extraction_diagnostics_open_tenant_unique;
DROP INDEX IF EXISTS extraction_diagnostics_open_legacy_unique;
DROP INDEX IF EXISTS label_merchants_tenant_mapping_key;
DROP INDEX IF EXISTS label_merchants_legacy_mapping_key;
DROP INDEX IF EXISTS merchant_categories_tenant_fragment_key;
DROP INDEX IF EXISTS merchant_categories_legacy_fragment_key;
DROP INDEX IF EXISTS muted_merchants_tenant_pattern_key;
DROP INDEX IF EXISTS muted_merchants_legacy_pattern_key;
DROP INDEX IF EXISTS rules_tenant_user_name_key;
DROP INDEX IF EXISTS rules_legacy_user_name_key;
DROP INDEX IF EXISTS rules_predefined_name_key;
DROP INDEX IF EXISTS buckets_tenant_name_key;
DROP INDEX IF EXISTS buckets_legacy_name_key;
DROP INDEX IF EXISTS categories_tenant_name_key;
DROP INDEX IF EXISTS categories_legacy_name_key;
DROP INDEX IF EXISTS labels_tenant_name_key;
DROP INDEX IF EXISTS labels_legacy_name_key;
DROP INDEX IF EXISTS app_config_tenant_key;
DROP INDEX IF EXISTS app_config_legacy_key;
DROP INDEX IF EXISTS transactions_tenant_message_id_key;
DROP INDEX IF EXISTS transactions_legacy_message_id_key;

ALTER TABLE transactions ADD CONSTRAINT transactions_message_id_key UNIQUE (message_id);
ALTER TABLE app_config ADD CONSTRAINT app_config_pkey PRIMARY KEY (key);
ALTER TABLE labels ADD CONSTRAINT labels_pkey PRIMARY KEY (name);
ALTER TABLE categories ADD CONSTRAINT categories_pkey PRIMARY KEY (name);
ALTER TABLE buckets ADD CONSTRAINT buckets_pkey PRIMARY KEY (name);
ALTER TABLE rules ADD CONSTRAINT rules_name_key UNIQUE (name);
ALTER TABLE muted_merchants ADD CONSTRAINT muted_merchants_pattern_key UNIQUE (pattern);
ALTER TABLE merchant_categories ADD CONSTRAINT merchant_categories_fragment_key UNIQUE (fragment);
ALTER TABLE label_merchants ADD CONSTRAINT label_merchants_label_fkey
    FOREIGN KEY (label) REFERENCES labels(name) ON DELETE CASCADE;

CREATE UNIQUE INDEX IF NOT EXISTS extraction_diagnostics_open_unique
    ON extraction_diagnostics (reader, message_id, rule_name)
    WHERE status = 'open' AND message_id IS NOT NULL;

ALTER TABLE extraction_diagnostics DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE label_merchants DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE merchant_categories DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE muted_merchants DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE rules DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE buckets DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE categories DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE labels DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE app_config DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE transactions DROP COLUMN IF EXISTS tenant_id;
