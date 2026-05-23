-- Backfill old transaction rows to the v2 structured source fields.
--
-- This runs after 002 for databases that already recorded the first structured
-- source migration but still have legacy source strings or blank bank/type
-- fields on existing transactions.

CREATE TEMP TABLE transaction_source_v2_migration (
    legacy_label TEXT PRIMARY KEY,
    source_type TEXT NOT NULL,
    source_label TEXT NOT NULL,
    bank TEXT NOT NULL
) ON COMMIT DROP;

INSERT INTO transaction_source_v2_migration (legacy_label, source_type, source_label, bank)
VALUES
    ('Credit Card - ICICI', 'Credit Card', 'ICICI Credit Card', 'ICICI'),
    ('Credit Card - HDFC', 'Credit Card', 'HDFC Credit Card', 'HDFC'),
    ('UPI - HDFC', 'UPI', 'HDFC UPI', 'HDFC'),
    ('Credit Card - Axis', 'Credit Card', 'Axis Bank Credit Card', 'Axis'),
    ('iMobile - ICICI', 'Mobile App', 'ICICI iMobile', 'ICICI'),
    ('Debit Card - ICICI', 'Debit Card', 'ICICI Debit Card', 'ICICI');

UPDATE transactions AS t
SET source_type = CASE WHEN t.source_type = '' THEN v.source_type ELSE t.source_type END,
    source_label = CASE
        WHEN t.source_label = '' OR t.source_label = v.legacy_label THEN v.source_label
        ELSE t.source_label
    END,
    bank = CASE WHEN t.bank = '' THEN v.bank ELSE t.bank END
FROM transaction_source_v2_migration AS v
WHERE t.source = v.legacy_label
   OR t.source_label = v.legacy_label;

UPDATE transactions
SET source_label = COALESCE(NULLIF(source_label, ''), source)
WHERE source_label = ''
  AND source <> '';

UPDATE transactions
SET bank = CASE
        WHEN bank = '' AND source_label ILIKE '%hdfc%' THEN 'HDFC'
        WHEN bank = '' AND source_label ILIKE '%icici%' THEN 'ICICI'
        WHEN bank = '' AND source_label ILIKE '%axis%' THEN 'Axis'
        ELSE bank
    END,
    source_type = CASE
        WHEN source_type = '' AND source_label ILIKE '%credit card%' THEN 'Credit Card'
        WHEN source_type = '' AND source_label ILIKE '%debit card%' THEN 'Debit Card'
        WHEN source_type = '' AND source_label ILIKE '%upi%' THEN 'UPI'
        WHEN source_type = '' AND (source_label ILIKE '%netbanking%' OR source_label ILIKE '%net banking%') THEN 'NetBanking'
        WHEN source_type = '' AND (source_label ILIKE '%mobile%' OR source_label ILIKE '%imobile%') THEN 'Mobile App'
        ELSE source_type
    END
WHERE (bank = '' OR source_type = '')
  AND source_label <> '';
