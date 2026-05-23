-- Remove stale v1 bundled rules and backfill the remaining predefined rules to
-- the v2 sender/source shape.
--
-- The startup seeder intentionally does not overwrite existing predefined rows,
-- so installs that already seeded the v1 rules need a migration for stale rule
-- removal, exact sender lists, and structured source fields.

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
