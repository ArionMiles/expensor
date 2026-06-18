-- Tenant-scope reader runtime and processed message state.
-- Secret/token plaintext JSONB columns remain for legacy import compatibility, but new writes use ciphertext bytea columns.

ALTER TABLE reader_runtime ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE reader_runtime ADD COLUMN IF NOT EXISTS client_secret_ciphertext BYTEA;
ALTER TABLE reader_runtime ADD COLUMN IF NOT EXISTS oauth_token_ciphertext BYTEA;

ALTER TABLE processed_messages ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE reader_runtime DROP CONSTRAINT IF EXISTS reader_runtime_pkey;
ALTER TABLE processed_messages DROP CONSTRAINT IF EXISTS processed_messages_pkey;

CREATE UNIQUE INDEX IF NOT EXISTS reader_runtime_legacy_reader_key
    ON reader_runtime (reader)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS reader_runtime_tenant_reader_key
    ON reader_runtime (tenant_id, reader)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS processed_messages_legacy_key
    ON processed_messages (message_key)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS processed_messages_tenant_key
    ON processed_messages (tenant_id, message_key)
    WHERE tenant_id IS NOT NULL;
