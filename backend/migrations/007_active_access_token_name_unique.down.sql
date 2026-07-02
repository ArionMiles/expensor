DROP INDEX IF EXISTS access_tokens_active_name_unique;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'access_tokens_user_id_name_key'
    ) THEN
        ALTER TABLE access_tokens ADD CONSTRAINT access_tokens_user_id_name_key UNIQUE (user_id, name);
    END IF;
END $$;
