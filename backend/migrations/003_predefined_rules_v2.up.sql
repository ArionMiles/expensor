-- Backfill existing installs to the v2 rule and source shape.
--
-- The startup seeder intentionally does not overwrite existing predefined rows,
-- so installs that already seeded the v1 rules need a migration for stale rule
-- removal, exact sender lists, and structured source fields. Old transaction
-- rows also need their legacy source strings mapped into source_type,
-- source_label, and bank.

CREATE TEMP TABLE predefined_rules_v2_migration (
    name TEXT PRIMARY KEY,
    sender_email TEXT NOT NULL,
    sender_emails TEXT[] NOT NULL,
    source_type TEXT NOT NULL,
    source_label TEXT NOT NULL,
    bank TEXT NOT NULL
) ON COMMIT DROP;

INSERT INTO predefined_rules_v2_migration (name, sender_email, sender_emails, source_type, source_label, bank)
VALUES
        (
            'ICICI Credit Card',
            'credit_cards@icicibank.com',
            ARRAY['credit_cards@icicibank.com', 'credit_cards@icici.bank.in']::TEXT[],
            'Credit Card',
            'ICICI Credit Card',
            'ICICI'
        ),
        (
            'HDFC Credit Card',
            'alerts@hdfcbank.bank.in',
            ARRAY['alerts@hdfcbank.bank.in', 'alerts@hdfcbank.net']::TEXT[],
            'Credit Card',
            'HDFC Credit Card',
            'HDFC'
        ),
        (
            'HDFC Credit Card Debit Alert',
            'alerts@hdfcbank.bank.in',
            ARRAY['alerts@hdfcbank.bank.in', 'alerts@hdfcbank.net']::TEXT[],
            'Credit Card',
            'HDFC Credit Card',
            'HDFC'
        ),
        (
            'HDFC Credit Card Payment Made',
            'alerts@hdfcbank.bank.in',
            ARRAY['alerts@hdfcbank.bank.in', 'alerts@hdfcbank.net']::TEXT[],
            'Credit Card',
            'HDFC Credit Card',
            'HDFC'
        ),
        (
            'HDFC UPI',
            'alerts@hdfcbank.bank.in',
            ARRAY['alerts@hdfcbank.bank.in', 'alerts@hdfcbank.net']::TEXT[],
            'UPI',
            'HDFC UPI',
            'HDFC'
        ),
        (
            'Axis Bank Credit Card',
            'alerts@axis.bank.in',
            ARRAY['alerts@axis.bank.in']::TEXT[],
            'Credit Card',
            'Axis Bank Credit Card',
            'Axis'
        ),
        (
            'ICICI iMobile Fund Transfer',
            'customercare@icicibank.com',
            ARRAY['customercare@icicibank.com']::TEXT[],
            'Mobile App',
            'ICICI iMobile',
            'ICICI'
        ),
        (
            'ICICI NEFT iMobile',
            'customernotification@icici.bank.in',
            ARRAY['customernotification@icici.bank.in']::TEXT[],
            'Mobile App',
            'ICICI iMobile',
            'ICICI'
        ),
        (
            'ICICI Debit Card',
            'alert@icicibank.com',
            ARRAY['alert@icicibank.com']::TEXT[],
            'Debit Card',
            'ICICI Debit Card',
            'ICICI'
        );

DELETE FROM rules AS r
WHERE r.predefined = TRUE
  AND NOT EXISTS (
      SELECT 1
      FROM predefined_rules_v2_migration AS v
      WHERE v.name = r.name
  );

UPDATE rules AS r
SET sender_email = v.sender_email,
    sender_emails = v.sender_emails,
    transaction_source = v.source_label,
    source_type = v.source_type,
    source_label = v.source_label,
    bank = v.bank
FROM predefined_rules_v2_migration AS v
WHERE r.predefined = TRUE
  AND r.name = v.name;

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
