ALTER TABLE transactions
    DROP COLUMN IF EXISTS source_type,
    DROP COLUMN IF EXISTS source_label,
    DROP COLUMN IF EXISTS bank;

ALTER TABLE rules
    DROP COLUMN IF EXISTS sender_emails,
    DROP COLUMN IF EXISTS source_type,
    DROP COLUMN IF EXISTS source_label,
    DROP COLUMN IF EXISTS bank;
