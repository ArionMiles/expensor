DROP INDEX IF EXISTS processed_messages_tenant_key;
DROP INDEX IF EXISTS processed_messages_legacy_key;
DROP INDEX IF EXISTS reader_runtime_tenant_reader_key;
DROP INDEX IF EXISTS reader_runtime_legacy_reader_key;

ALTER TABLE processed_messages DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE reader_runtime DROP COLUMN IF EXISTS oauth_token_ciphertext;
ALTER TABLE reader_runtime DROP COLUMN IF EXISTS client_secret_ciphertext;
ALTER TABLE reader_runtime DROP COLUMN IF EXISTS tenant_id;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'processed_messages'::regclass
          AND conname = 'processed_messages_pkey'
    ) THEN
        ALTER TABLE processed_messages ADD PRIMARY KEY (message_key);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'reader_runtime'::regclass
          AND conname = 'reader_runtime_pkey'
    ) THEN
        ALTER TABLE reader_runtime ADD PRIMARY KEY (reader);
    END IF;
END $$;
