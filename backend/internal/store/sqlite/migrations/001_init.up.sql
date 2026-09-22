CREATE TABLE users (
    id TEXT PRIMARY KEY NOT NULL,
    email TEXT NOT NULL,
    password_hash TEXT,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin','user')),
    avatar_key TEXT NOT NULL DEFAULT 'default',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',updated_at,'+0 days') IS substr(updated_at,1,19)),
    disabled_at TEXT CHECK (disabled_at IS NULL OR (disabled_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',disabled_at,'+0 days') IS substr(disabled_at,1,19)))
) STRICT;

CREATE UNIQUE INDEX users_email_casefold_unique ON users (expensor_casefold(email));

CREATE TABLE sessions (
    id TEXT PRIMARY KEY NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    expires_at TEXT NOT NULL CHECK (expires_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',expires_at,'+0 days') IS substr(expires_at,1,19)),
    last_used_at TEXT CHECK (last_used_at IS NULL OR (last_used_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',last_used_at,'+0 days') IS substr(last_used_at,1,19))),
    revoked_at TEXT CHECK (revoked_at IS NULL OR (revoked_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',revoked_at,'+0 days') IS substr(revoked_at,1,19)))
) STRICT;

CREATE UNIQUE INDEX sessions_token_hash_unique ON sessions (token_hash);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_active_lookup_idx ON sessions (token_hash) WHERE revoked_at IS NULL;

CREATE TABLE access_tokens (
    id TEXT PRIMARY KEY NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    expires_at TEXT CHECK (expires_at IS NULL OR (expires_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',expires_at,'+0 days') IS substr(expires_at,1,19))),
    last_used_at TEXT CHECK (last_used_at IS NULL OR (last_used_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',last_used_at,'+0 days') IS substr(last_used_at,1,19))),
    revoked_at TEXT CHECK (revoked_at IS NULL OR (revoked_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',revoked_at,'+0 days') IS substr(revoked_at,1,19)))
) STRICT;

CREATE UNIQUE INDEX access_tokens_token_hash_unique ON access_tokens (token_hash);
CREATE INDEX access_tokens_user_id_idx ON access_tokens (user_id);
CREATE INDEX access_tokens_active_lookup_idx ON access_tokens (token_hash) WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX access_tokens_active_name_unique ON access_tokens (user_id,name) WHERE revoked_at IS NULL;

CREATE TABLE account_setup_tokens (
    id TEXT PRIMARY KEY NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    expires_at TEXT NOT NULL CHECK (expires_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',expires_at,'+0 days') IS substr(expires_at,1,19)),
    used_at TEXT CHECK (used_at IS NULL OR (used_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',used_at,'+0 days') IS substr(used_at,1,19)))
) STRICT;

CREATE UNIQUE INDEX account_setup_tokens_token_hash_unique ON account_setup_tokens (token_hash);
CREATE INDEX account_setup_tokens_user_id_idx ON account_setup_tokens (user_id);

CREATE TABLE transactions (
    id TEXT PRIMARY KEY NOT NULL,
    tenant_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL,
    amount INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'INR',
    original_amount INTEGER,
    original_currency TEXT,
    exchange_rate INTEGER,
    timestamp TEXT NOT NULL CHECK (timestamp GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',timestamp,'+0 days') IS substr(timestamp,1,19)),
    merchant_info TEXT NOT NULL,
    category TEXT,
    bucket TEXT,
    source TEXT NOT NULL,
    source_type TEXT NOT NULL DEFAULT '',
    source_label TEXT NOT NULL DEFAULT '',
    bank TEXT NOT NULL DEFAULT '',
    description TEXT,
    metadata TEXT NOT NULL DEFAULT '{}' CHECK (metadata IS NULL OR json_valid(metadata)),
    muted INTEGER NOT NULL DEFAULT 0 CHECK (muted IN (0,1)),
    muted_by_merchant INTEGER NOT NULL DEFAULT 0 CHECK (muted_by_merchant IN (0,1)),
    mute_reason TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',updated_at,'+0 days') IS substr(updated_at,1,19))
) STRICT;

CREATE UNIQUE INDEX transactions_tenant_message_id_key ON transactions (tenant_id,message_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX transactions_tenant_timestamp_idx ON transactions (tenant_id,timestamp DESC);
CREATE INDEX transactions_currency_idx ON transactions (currency);
CREATE INDEX transactions_tenant_category_idx ON transactions (tenant_id,category);
CREATE INDEX transactions_tenant_bucket_idx ON transactions (tenant_id,bucket);
CREATE INDEX transactions_muted_idx ON transactions (tenant_id,muted) WHERE muted=1;
CREATE INDEX transactions_muted_by_merchant_idx ON transactions (tenant_id,muted_by_merchant) WHERE muted_by_merchant=1;

CREATE TABLE transaction_labels (
    id TEXT PRIMARY KEY NOT NULL,
    transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19))
) STRICT;

CREATE UNIQUE INDEX transaction_labels_unique ON transaction_labels (transaction_id,label);
CREATE INDEX transaction_labels_label_idx ON transaction_labels (label);
CREATE INDEX transaction_labels_transaction_id_idx ON transaction_labels (transaction_id);

CREATE TABLE transaction_label_sources (
    id TEXT PRIMARY KEY NOT NULL,
    transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('manual','merchant')),
    merchant_pattern TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19))
) STRICT;

CREATE UNIQUE INDEX transaction_label_sources_unique ON transaction_label_sources (transaction_id,label,source_type,merchant_pattern);
CREATE INDEX transaction_label_sources_transaction_idx ON transaction_label_sources (transaction_id,label);

CREATE TABLE app_config (
    tenant_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL
) STRICT;

CREATE UNIQUE INDEX app_config_global_key ON app_config (key) WHERE tenant_id IS NULL;
CREATE UNIQUE INDEX app_config_tenant_key ON app_config (tenant_id,key) WHERE tenant_id IS NOT NULL;

CREATE TABLE labels (
    tenant_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '#6366f1',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19))
) STRICT;

CREATE UNIQUE INDEX labels_tenant_name_key ON labels (tenant_id,name) WHERE tenant_id IS NOT NULL;

CREATE TABLE categories (
    tenant_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0,1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19))
) STRICT;

CREATE UNIQUE INDEX categories_global_name_key ON categories (name) WHERE tenant_id IS NULL;
CREATE UNIQUE INDEX categories_tenant_name_key ON categories (tenant_id,name) WHERE tenant_id IS NOT NULL;

CREATE TABLE buckets (
    tenant_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0,1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19))
) STRICT;

CREATE UNIQUE INDEX buckets_global_name_key ON buckets (name) WHERE tenant_id IS NULL;
CREATE UNIQUE INDEX buckets_tenant_name_key ON buckets (tenant_id,name) WHERE tenant_id IS NOT NULL;

CREATE TABLE rules (
    id TEXT PRIMARY KEY NOT NULL,
    tenant_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    sender_email TEXT NOT NULL DEFAULT '',
    sender_emails TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(sender_emails) AND json_type(sender_emails)='array'),
    subject_contains TEXT NOT NULL DEFAULT '',
    amount_regex TEXT NOT NULL,
    merchant_regex TEXT NOT NULL,
    currency_regex TEXT NOT NULL DEFAULT '',
    transaction_source TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL DEFAULT '',
    source_label TEXT NOT NULL DEFAULT '',
    bank TEXT NOT NULL DEFAULT '',
    predefined INTEGER NOT NULL DEFAULT 0 CHECK (predefined IN (0,1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',updated_at,'+0 days') IS substr(updated_at,1,19)),
    CHECK ((predefined=1 AND tenant_id IS NULL) OR (predefined=0 AND tenant_id IS NOT NULL))
) STRICT;

CREATE UNIQUE INDEX rules_predefined_name_key ON rules (name) WHERE tenant_id IS NULL AND predefined=1;
CREATE UNIQUE INDEX rules_tenant_user_name_key ON rules (tenant_id,name) WHERE tenant_id IS NOT NULL AND predefined=0;

CREATE TABLE muted_merchants (
    id TEXT PRIMARY KEY NOT NULL,
    tenant_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pattern TEXT NOT NULL,
    reason TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19))
) STRICT;

CREATE UNIQUE INDEX muted_merchants_tenant_pattern_key ON muted_merchants (tenant_id,pattern) WHERE tenant_id IS NOT NULL;

CREATE TABLE mcc_codes (
    code TEXT PRIMARY KEY NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL,
    bucket TEXT NOT NULL DEFAULT 'Wants',
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',updated_at,'+0 days') IS substr(updated_at,1,19))
) STRICT;

CREATE TABLE merchant_categories (
    id TEXT PRIMARY KEY NOT NULL,
    tenant_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    fragment TEXT NOT NULL,
    mcc_code TEXT REFERENCES mcc_codes(code) ON DELETE SET NULL,
    category TEXT,
    bucket TEXT,
    source TEXT NOT NULL DEFAULT 'community',
    user_locked INTEGER NOT NULL DEFAULT 0 CHECK (user_locked IN (0,1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',updated_at,'+0 days') IS substr(updated_at,1,19))
) STRICT;

CREATE UNIQUE INDEX merchant_categories_global_fragment_key ON merchant_categories (fragment) WHERE tenant_id IS NULL;
CREATE UNIQUE INDEX merchant_categories_tenant_fragment_key ON merchant_categories (tenant_id,fragment) WHERE tenant_id IS NOT NULL;

CREATE TABLE label_merchants (
    id TEXT PRIMARY KEY NOT NULL,
    tenant_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    merchant_pattern TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19))
) STRICT;

CREATE UNIQUE INDEX label_merchants_tenant_mapping_key ON label_merchants (tenant_id,label,merchant_pattern) WHERE tenant_id IS NOT NULL;

CREATE TABLE extraction_diagnostics (
    id TEXT PRIMARY KEY NOT NULL,
    tenant_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','resolved','ignored')),
    reader TEXT NOT NULL,
    message_id TEXT,
    source TEXT NOT NULL DEFAULT '',
    sender TEXT NOT NULL DEFAULT '',
    sender_email TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    email_body TEXT NOT NULL DEFAULT '',
    received_at TEXT CHECK (received_at IS NULL OR (received_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',received_at,'+0 days') IS substr(received_at,1,19))),
    snippet TEXT NOT NULL DEFAULT '',
    rule_id TEXT REFERENCES rules(id) ON DELETE SET NULL,
    rule_name TEXT NOT NULL DEFAULT '',
    amount_regex TEXT NOT NULL DEFAULT '',
    merchant_regex TEXT NOT NULL DEFAULT '',
    currency_regex TEXT NOT NULL DEFAULT '',
    failure_reasons TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(failure_reasons) AND json_type(failure_reasons)='array'),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',updated_at,'+0 days') IS substr(updated_at,1,19)),
    resolved_at TEXT CHECK (resolved_at IS NULL OR (resolved_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',resolved_at,'+0 days') IS substr(resolved_at,1,19)))
) STRICT;

CREATE UNIQUE INDEX extraction_diagnostics_open_tenant_unique
    ON extraction_diagnostics (tenant_id,reader,message_id,rule_name)
    WHERE tenant_id IS NOT NULL AND status='open' AND message_id IS NOT NULL;
CREATE INDEX extraction_diagnostics_tenant_status_created_idx
    ON extraction_diagnostics (tenant_id,status,created_at DESC);

CREATE TABLE reader_runtime (
    tenant_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reader TEXT NOT NULL,
    client_secret_ciphertext BLOB,
    oauth_token_ciphertext BLOB,
    config TEXT CHECK (config IS NULL OR json_valid(config)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',updated_at,'+0 days') IS substr(updated_at,1,19))
) STRICT;

CREATE UNIQUE INDEX reader_runtime_tenant_reader_key ON reader_runtime (tenant_id,reader) WHERE tenant_id IS NOT NULL;

CREATE TABLE processed_messages (
    tenant_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_key TEXT NOT NULL,
    processed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (processed_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',processed_at,'+0 days') IS substr(processed_at,1,19))
) STRICT;

CREATE UNIQUE INDEX processed_messages_tenant_key ON processed_messages (tenant_id,message_key) WHERE tenant_id IS NOT NULL;

CREATE TABLE scheduler_config (
    id INTEGER PRIMARY KEY NOT NULL DEFAULT 1 CHECK (id=1),
    max_concurrent_scans INTEGER NOT NULL DEFAULT 4 CHECK (max_concurrent_scans BETWEEN 1 AND 64),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',updated_at,'+0 days') IS substr(updated_at,1,19))
) STRICT;

INSERT INTO scheduler_config (id,max_concurrent_scans) VALUES (1,4);

CREATE TABLE tenant_scanning_state (
    tenant_id TEXT PRIMARY KEY NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    active_reader TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
    state TEXT NOT NULL DEFAULT 'stopped' CHECK (state IN ('queued','starting','running','backing_off','needs_auth','reader_not_configured','paused','stopped')),
    reason_code TEXT NOT NULL DEFAULT '',
    public_message TEXT NOT NULL DEFAULT '',
    last_started_at TEXT CHECK (last_started_at IS NULL OR (last_started_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',last_started_at,'+0 days') IS substr(last_started_at,1,19))),
    last_stopped_at TEXT CHECK (last_stopped_at IS NULL OR (last_stopped_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',last_stopped_at,'+0 days') IS substr(last_stopped_at,1,19))),
    last_failed_at TEXT CHECK (last_failed_at IS NULL OR (last_failed_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',last_failed_at,'+0 days') IS substr(last_failed_at,1,19))),
    next_retry_at TEXT CHECK (next_retry_at IS NULL OR (next_retry_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',next_retry_at,'+0 days') IS substr(next_retry_at,1,19))),
    retry_count INTEGER NOT NULL DEFAULT 0 CHECK (retry_count>=0),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',updated_at,'+0 days') IS substr(updated_at,1,19))
) STRICT;

CREATE INDEX tenant_scanning_state_runnable_idx ON tenant_scanning_state (enabled,state,next_retry_at);

CREATE TABLE llm_provider_runtime (
    tenant_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(config)),
    credentials_ciphertext BLOB,
    active INTEGER NOT NULL DEFAULT 0 CHECK (active IN (0,1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',created_at,'+0 days') IS substr(created_at,1,19)),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f000Z','now'))
        CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z' AND strftime('%Y-%m-%dT%H:%M:%S',updated_at,'+0 days') IS substr(updated_at,1,19))
) STRICT;

CREATE UNIQUE INDEX llm_provider_runtime_tenant_provider_key ON llm_provider_runtime (tenant_id,provider) WHERE tenant_id IS NOT NULL;
CREATE UNIQUE INDEX llm_provider_runtime_tenant_active_key ON llm_provider_runtime (tenant_id) WHERE tenant_id IS NOT NULL AND active=1;

INSERT INTO categories (tenant_id,name,is_default) VALUES
    (NULL,'Food & Dining',1),
    (NULL,'Transport',1),
    (NULL,'Shopping',1),
    (NULL,'Utilities',1),
    (NULL,'Healthcare',1),
    (NULL,'Entertainment',1),
    (NULL,'Travel',1),
    (NULL,'Finance',1);

INSERT INTO buckets (tenant_id,name,is_default) VALUES
    (NULL,'Needs',1),
    (NULL,'Wants',1),
    (NULL,'Investments',1),
    (NULL,'Income',1);

CREATE VIRTUAL TABLE transactions_fts USING fts5(
    transaction_id UNINDEXED,
    merchant,
    description
);

CREATE TRIGGER transactions_fts_insert AFTER INSERT ON transactions BEGIN
    INSERT INTO transactions_fts (transaction_id,merchant,description)
    VALUES (new.id,new.merchant_info,coalesce(new.description,''));
END;

CREATE TRIGGER transactions_fts_update AFTER UPDATE OF merchant_info,description ON transactions BEGIN
    DELETE FROM transactions_fts WHERE transaction_id=old.id;
    INSERT INTO transactions_fts (transaction_id,merchant,description)
    VALUES (new.id,new.merchant_info,coalesce(new.description,''));
END;

CREATE TRIGGER transactions_fts_delete AFTER DELETE ON transactions BEGIN
    DELETE FROM transactions_fts WHERE transaction_id=old.id;
END;
