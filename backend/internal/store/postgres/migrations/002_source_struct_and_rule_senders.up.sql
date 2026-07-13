-- Add structured source fields and exact multi-sender rule storage.

ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank TEXT NOT NULL DEFAULT '';

UPDATE transactions
SET source_label = COALESCE(NULLIF(source_label, ''), source)
WHERE COALESCE(source_label, '') = '';

UPDATE transactions
SET bank = CASE
        WHEN source_label ILIKE '%hdfc%' THEN 'HDFC'
        WHEN source_label ILIKE '%icici%' THEN 'ICICI'
        WHEN source_label ILIKE '%axis%' THEN 'Axis'
        ELSE bank
    END,
    source_type = CASE
        WHEN source_label ILIKE '%credit card%' THEN 'Credit Card'
        WHEN source_label ILIKE '%debit card%' THEN 'Debit Card'
        WHEN source_label ILIKE '%upi%' THEN 'UPI'
        WHEN source_label ILIKE '%netbanking%' OR source_label ILIKE '%net banking%' THEN 'NetBanking'
        WHEN source_label ILIKE '%mobile%' OR source_label ILIKE '%imobile%' THEN 'Mobile App'
        ELSE source_type
    END
WHERE (bank = '' OR source_type = '') AND source_label <> '';

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

UPDATE rules
SET bank = CASE
        WHEN source_label ILIKE '%hdfc%' THEN 'HDFC'
        WHEN source_label ILIKE '%icici%' THEN 'ICICI'
        WHEN source_label ILIKE '%axis%' THEN 'Axis'
        ELSE bank
    END,
    source_type = CASE
        WHEN source_label ILIKE '%credit card%' THEN 'Credit Card'
        WHEN source_label ILIKE '%debit card%' THEN 'Debit Card'
        WHEN source_label ILIKE '%upi%' THEN 'UPI'
        WHEN source_label ILIKE '%netbanking%' OR source_label ILIKE '%net banking%' THEN 'NetBanking'
        WHEN source_label ILIKE '%mobile%' OR source_label ILIKE '%imobile%' THEN 'Mobile App'
        ELSE source_type
    END
WHERE (bank = '' OR source_type = '') AND source_label <> '';
