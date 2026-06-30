ALTER TABLE access_tokens DROP CONSTRAINT IF EXISTS access_tokens_user_id_name_key;

CREATE UNIQUE INDEX IF NOT EXISTS access_tokens_active_name_unique
    ON access_tokens (user_id, name)
    WHERE revoked_at IS NULL;
