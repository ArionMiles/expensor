-- Add structured source fields and exact multi-sender rule storage.

ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank TEXT NOT NULL DEFAULT '';

UPDATE transactions
SET source_label = COALESCE(NULLIF(source_label, ''), source)
WHERE COALESCE(source_label, '') = '';

ALTER TABLE rules
    ADD COLUMN IF NOT EXISTS sender_emails TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank TEXT NOT NULL DEFAULT '';

UPDATE rules
SET sender_emails = ARRAY[sender_email]
WHERE cardinality(sender_emails) = 0 AND sender_email <> '';

UPDATE rules
SET source_label = transaction_source
WHERE source_label = '' AND transaction_source <> '';
