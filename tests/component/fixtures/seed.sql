INSERT INTO users (id, email, password_hash, display_name, role, avatar_key) VALUES (
  '00000000-0000-0000-0000-00000000c0de',
  'component-admin@example.com',
  '$2a$10$LfS5UEzIPBGqzhsDVmohr.FfBuPQLWlRH9QfNkMZgig0mnFc.BeCC',
  'Component Admin',
  'admin',
  'default'
) ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  password_hash = EXCLUDED.password_hash,
  display_name = EXCLUDED.display_name,
  role = EXCLUDED.role,
  avatar_key = EXCLUDED.avatar_key,
  disabled_at = NULL,
  updated_at = NOW();

INSERT INTO access_tokens (user_id, name, token_hash) VALUES (
  '00000000-0000-0000-0000-00000000c0de',
  'component contract token',
  'sha256:136a976b1f2c9c3d0fe47759b8f1b112a7dcb1c77197d213c6bd1715c54c111b'
) ON CONFLICT (user_id, name) DO UPDATE SET
  token_hash = EXCLUDED.token_hash,
  revoked_at = NULL;

INSERT INTO app_config (tenant_id, key, value) VALUES
  ('00000000-0000-0000-0000-00000000c0de', 'base_currency', 'INR'),
  ('00000000-0000-0000-0000-00000000c0de', 'scan_interval', '120'),
  ('00000000-0000-0000-0000-00000000c0de', 'lookback_days', '365'),
  ('00000000-0000-0000-0000-00000000c0de', 'app.timezone', 'Asia/Kolkata'),
  ('00000000-0000-0000-0000-00000000c0de', 'app.time_format', 'HH:mm'),
  ('00000000-0000-0000-0000-00000000c0de', 'active_reader', 'thunderbird'),
  ('00000000-0000-0000-0000-00000000c0de', 'reader.gmail.last_scan_at', '2026-05-23T06:00:00Z')
ON CONFLICT (tenant_id, key) WHERE tenant_id IS NOT NULL DO UPDATE SET value = EXCLUDED.value;

INSERT INTO reader_runtime (tenant_id, reader, config) VALUES
  ('00000000-0000-0000-0000-00000000c0de', 'thunderbird', '{"config":{"profilePath":"/workspace/tests/component/fixtures/thunderbird-profile","mailboxes":"Inbox"}}'::jsonb)
ON CONFLICT (tenant_id, reader) WHERE tenant_id IS NOT NULL DO UPDATE SET config = EXCLUDED.config, updated_at = NOW();

INSERT INTO labels (tenant_id, name, color) VALUES
  ('00000000-0000-0000-0000-00000000c0de', '10min Delivery', '#8b5cf6'),
  ('00000000-0000-0000-0000-00000000c0de', 'Recurring', '#6366f1'),
  ('00000000-0000-0000-0000-00000000c0de', 'Weekend', '#f59e0b'),
  ('00000000-0000-0000-0000-00000000c0de', 'Family', '#ec4899'),
  ('00000000-0000-0000-0000-00000000c0de', 'Office', '#14b8a6'),
  ('00000000-0000-0000-0000-00000000c0de', 'Shared', '#22c55e'),
  ('00000000-0000-0000-0000-00000000c0de', 'High Value', '#ef4444'),
  ('00000000-0000-0000-0000-00000000c0de', 'Reimbursable', '#10b981'),
  ('00000000-0000-0000-0000-00000000c0de', 'Late Night', '#64748b'),
  ('00000000-0000-0000-0000-00000000c0de', 'Online', '#3b82f6')
ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL DO UPDATE SET color = EXCLUDED.color;

INSERT INTO categories (tenant_id, name, description, is_default) VALUES
  ('00000000-0000-0000-0000-00000000c0de', 'Food & Dining', 'Seeded dining category', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Groceries', 'Seeded groceries category', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Utilities', 'Seeded utilities category', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Entertainment', 'Seeded entertainment category', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Travel', 'Seeded travel category', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Healthcare', 'Seeded healthcare category', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Transport', 'Seeded transport category', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Shopping', 'Seeded shopping category', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Subscriptions', 'Seeded subscriptions category', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Personal Care', 'Seeded personal care category', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Investments', 'Seeded investments category', true)
ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL DO UPDATE SET
  description = EXCLUDED.description,
  is_default = EXCLUDED.is_default;

INSERT INTO buckets (tenant_id, name, description, is_default) VALUES
  ('00000000-0000-0000-0000-00000000c0de', 'Needs', 'Seeded needs bucket', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Wants', 'Seeded wants bucket', true),
  ('00000000-0000-0000-0000-00000000c0de', 'Investments', 'Seeded investments bucket', true)
ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL DO UPDATE SET
  description = EXCLUDED.description,
  is_default = EXCLUDED.is_default;

DELETE FROM transaction_label_sources
WHERE transaction_id IN (SELECT id FROM transactions WHERE message_id LIKE 'seed-msg-%');

DELETE FROM transaction_labels
WHERE transaction_id IN (SELECT id FROM transactions WHERE message_id LIKE 'seed-msg-%');

DELETE FROM transactions
WHERE tenant_id = '00000000-0000-0000-0000-00000000c0de'
  AND message_id LIKE 'seed-msg-%';

WITH merchant_seed AS (
  SELECT *
  FROM (
    VALUES
      (1, 'Swiggy', 277.00, 'Food & Dining', 'Wants', 'Credit Card', 'ICICI Credit Card', 'ICICI', 'Food delivery'),
      (2, 'Swiggy Limited', 593.00, 'Food & Dining', 'Wants', 'Credit Card', 'ICICI Credit Card', 'ICICI', 'Food delivery'),
      (3, 'PYU*Swiggy Food', 576.00, 'Food & Dining', 'Wants', 'Credit Card', 'HDFC Credit Card', 'HDFC', 'Food delivery'),
      (4, 'ZOMATO', 575.84, 'Food & Dining', 'Wants', 'Credit Card', 'ICICI Credit Card', 'ICICI', 'Dinner order'),
      (5, 'RAZ*Swiggy', 410.00, 'Food & Dining', 'Wants', 'Credit Card', 'HDFC Credit Card', 'HDFC', 'Food delivery'),
      (6, 'BLINKIT', 485.00, 'Groceries', 'Needs', 'Credit Card', 'HDFC Credit Card', 'HDFC', 'Grocery order'),
      (7, 'SWIGGY INSTAMART', 955.00, 'Groceries', 'Needs', 'UPI', 'HDFC UPI', 'HDFC', 'Grocery order'),
      (8, 'RSP*INSTAMART', 585.00, 'Groceries', 'Wants', 'Credit Card', 'HDFC Credit Card', 'HDFC', 'Quick commerce'),
      (9, 'RSP*BLINK COMMERCE PVT', 909.00, 'Groceries', 'Needs', 'Credit Card', 'HDFC Credit Card', 'HDFC', 'Grocery order'),
      (10, '1MG HEALTHCARE SOLU', 620.30, 'Healthcare', 'Needs', 'Credit Card', 'ICICI Credit Card', 'ICICI', 'Pharmacy'),
      (11, 'BUNDL TECHNOLOGIES', 2135.00, 'Utilities', 'Needs', 'Credit Card', 'HDFC Credit Card', 'HDFC', 'Home services'),
      (12, 'SURESH BALU SHENDGE', 1900.00, 'Utilities', 'Needs', 'UPI', 'HDFC UPI', 'HDFC', 'Maintenance'),
      (13, 'MSEDCL BILLDESK', 1540.00, 'Utilities', 'Needs', 'NetBanking', 'ICICI NetBanking', 'ICICI', 'Electricity bill'),
      (14, 'OPENAI *CHATGPT SUBSCR', 1999.00, 'Subscriptions', 'Wants', 'Credit Card', 'ICICI Credit Card', 'ICICI', 'Monthly subscription'),
      (15, 'NETFLIX.COM', 649.00, 'Entertainment', 'Wants', 'Credit Card', 'HDFC Credit Card', 'HDFC', 'Streaming'),
      (16, 'PVR INOX Limited', 850.00, 'Entertainment', 'Wants', 'UPI', 'HDFC UPI', 'HDFC', 'Movie tickets'),
      (17, 'PLAYSTATION', 849.00, 'Entertainment', 'Wants', 'Credit Card', 'ICICI Credit Card', 'ICICI', 'Game purchase'),
      (18, 'BookMyShow', 1260.00, 'Entertainment', 'Wants', 'UPI', 'Axis UPI', 'Axis', 'Event tickets'),
      (19, 'DREAM ENTERPRISES', 1500.00, 'Personal Care', 'Needs', 'UPI', 'HDFC UPI', 'HDFC', 'Haircut and spa'),
      (20, 'Apollo Pharmacy', 780.00, 'Healthcare', 'Needs', 'Credit Card', 'HDFC Credit Card', 'HDFC', 'Medicines'),
      (21, 'Uber India', 324.00, 'Transport', 'Needs', 'UPI', 'HDFC UPI', 'HDFC', 'Cab ride'),
      (22, 'Rapido', 146.00, 'Transport', 'Needs', 'UPI', 'HDFC UPI', 'HDFC', 'Bike taxi'),
      (23, 'IRCTC', 2450.00, 'Travel', 'Wants', 'Debit Card', 'ICICI Debit Card', 'ICICI', 'Train booking'),
      (24, 'INDIGO', 6990.00, 'Travel', 'Wants', 'Credit Card', 'ICICI Credit Card', 'ICICI', 'Flight booking'),
      (25, 'Airbnb', 4800.00, 'Travel', 'Wants', 'Credit Card', 'Axis Credit Card', 'Axis', 'Stay booking'),
      (26, 'AMAZON PAY', 2350.00, 'Shopping', 'Wants', 'Credit Card', 'ICICI Credit Card', 'ICICI', 'Online order'),
      (27, 'Myntra', 1890.00, 'Shopping', 'Wants', 'Credit Card', 'Axis Credit Card', 'Axis', 'Clothing order'),
      (28, 'GROWW MUTUAL FUND', 5000.00, 'Investments', 'Investments', 'Mobile App', 'ICICI iMobile', 'ICICI', 'Monthly SIP'),
      (29, 'ZERODHA BROKING', 3500.00, 'Investments', 'Investments', 'NetBanking', 'HDFC NetBanking', 'HDFC', 'Broker transfer'),
      (30, 'NPS CONTRIBUTION', 2500.00, 'Investments', 'Investments', 'Debit Card', 'SBI Debit Card', 'SBI', 'Retirement contribution'),
      (31, 'RENTOMOJO', 1200.00, 'Subscriptions', 'Needs', 'Credit Card', 'HDFC Credit Card', 'HDFC', 'Rental subscription'),
      (32, 'TATA CLiQ', 2100.00, 'Shopping', 'Wants', 'Credit Card', 'HDFC Credit Card', 'HDFC', 'Online order')
  ) AS seed(idx, merchant_info, base_amount, category, bucket, source_type, source_label, bank, description)
),
current_month_rows AS (
  SELECT seq
  FROM generate_series(1, 75) AS seq
),
history_rows AS (
  SELECT seq
  FROM generate_series(76, 240) AS seq
),
seed_rows AS (
  SELECT
    seq,
    make_timestamptz(
      2026,
      5,
      31 - ((seq - 1) % 30),
      7 + (seq % 15),
      (seq * 7) % 60,
      0,
      'Asia/Kolkata'
    ) AS spent_at
  FROM current_month_rows
  UNION ALL
  SELECT
    seq,
    make_timestamptz(
      EXTRACT(YEAR FROM (DATE '2025-06-01' + (((seq - 76) / 15) * INTERVAL '1 month')))::int,
      EXTRACT(MONTH FROM (DATE '2025-06-01' + (((seq - 76) / 15) * INTERVAL '1 month')))::int,
      ((seq - 76) % 27) + 1,
      7 + (seq % 15),
      (seq * 11) % 60,
      0,
      'Asia/Kolkata'
    ) AS spent_at
  FROM history_rows
),
transactions_to_insert AS (
  SELECT
    lpad(to_hex(seed_rows.seq), 32, '0')::uuid AS id,
    'seed-msg-' || seed_rows.seq AS message_id,
    (merchant_seed.base_amount + ((seed_rows.seq * 37) % 260))::numeric(19, 4) AS amount,
    'INR' AS currency,
    seed_rows.spent_at AS timestamp,
    merchant_seed.merchant_info,
    merchant_seed.category,
    merchant_seed.bucket,
    merchant_seed.source_type || ' - ' || merchant_seed.bank AS source,
    merchant_seed.source_type,
    merchant_seed.source_label,
    merchant_seed.bank,
    merchant_seed.description,
    false AS muted,
    false AS muted_by_merchant,
    NULL::text AS mute_reason
  FROM seed_rows
  JOIN merchant_seed ON merchant_seed.idx = ((seed_rows.seq - 1) % 32) + 1
)
INSERT INTO transactions (
  tenant_id,
  id,
  message_id,
  amount,
  currency,
  timestamp,
  merchant_info,
  category,
  bucket,
  source,
  source_type,
  source_label,
  bank,
  description,
  muted,
  muted_by_merchant,
  mute_reason
)
SELECT
  '00000000-0000-0000-0000-00000000c0de',
  id,
  message_id,
  amount,
  currency,
  timestamp,
  merchant_info,
  category,
  bucket,
  source,
  source_type,
  source_label,
  bank,
  description,
  muted,
  muted_by_merchant,
  mute_reason
FROM transactions_to_insert
ON CONFLICT (id) DO NOTHING;

WITH labeled AS (
  SELECT id, label, source_type, merchant_pattern
  FROM (
    SELECT id, '10min Delivery' AS label, 'merchant' AS source_type, merchant_info AS merchant_pattern
    FROM transactions
    WHERE message_id LIKE 'seed-msg-%'
      AND tenant_id = '00000000-0000-0000-0000-00000000c0de'
      AND (merchant_info ILIKE '%BLINK%' OR merchant_info ILIKE '%INSTAMART%')
    UNION
    SELECT id, 'Recurring', 'merchant', merchant_info
    FROM transactions
    WHERE message_id LIKE 'seed-msg-%'
      AND tenant_id = '00000000-0000-0000-0000-00000000c0de'
      AND merchant_info IN ('OPENAI *CHATGPT SUBSCR', 'NETFLIX.COM', 'RENTOMOJO', 'NPS CONTRIBUTION')
    UNION
    SELECT id, 'Weekend', 'manual', ''
    FROM transactions
    WHERE message_id LIKE 'seed-msg-%'
      AND tenant_id = '00000000-0000-0000-0000-00000000c0de'
      AND EXTRACT(ISODOW FROM timestamp AT TIME ZONE 'Asia/Kolkata') IN (6, 7)
    UNION
    SELECT id, 'Family', 'manual', ''
    FROM transactions
    WHERE message_id LIKE 'seed-msg-%'
      AND tenant_id = '00000000-0000-0000-0000-00000000c0de'
      AND regexp_replace(message_id, 'seed-msg-', '')::int % 11 = 0
    UNION
    SELECT id, 'Office', 'manual', ''
    FROM transactions
    WHERE message_id LIKE 'seed-msg-%'
      AND tenant_id = '00000000-0000-0000-0000-00000000c0de'
      AND regexp_replace(message_id, 'seed-msg-', '')::int % 13 = 0
    UNION
    SELECT id, 'Shared', 'manual', ''
    FROM transactions
    WHERE message_id LIKE 'seed-msg-%'
      AND tenant_id = '00000000-0000-0000-0000-00000000c0de'
      AND regexp_replace(message_id, 'seed-msg-', '')::int % 7 = 0
    UNION
    SELECT id, 'High Value', 'manual', ''
    FROM transactions
    WHERE message_id LIKE 'seed-msg-%'
      AND tenant_id = '00000000-0000-0000-0000-00000000c0de'
      AND amount >= 3500
    UNION
    SELECT id, 'Reimbursable', 'manual', ''
    FROM transactions
    WHERE message_id LIKE 'seed-msg-%'
      AND tenant_id = '00000000-0000-0000-0000-00000000c0de'
      AND amount >= 1800
      AND regexp_replace(message_id, 'seed-msg-', '')::int % 4 = 0
    UNION
    SELECT id, 'Late Night', 'manual', ''
    FROM transactions
    WHERE message_id LIKE 'seed-msg-%'
      AND tenant_id = '00000000-0000-0000-0000-00000000c0de'
      AND EXTRACT(HOUR FROM timestamp AT TIME ZONE 'Asia/Kolkata') >= 21
    UNION
    SELECT id, 'Online', 'manual', ''
    FROM transactions
    WHERE message_id LIKE 'seed-msg-%'
      AND tenant_id = '00000000-0000-0000-0000-00000000c0de'
      AND source_type = 'Credit Card'
      AND regexp_replace(message_id, 'seed-msg-', '')::int % 5 = 0
  ) AS labels_for_transactions
)
INSERT INTO transaction_labels (transaction_id, label)
SELECT id, label
FROM labeled
ON CONFLICT (transaction_id, label) DO NOTHING;

INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
SELECT
  transaction_id,
  label,
  CASE
    WHEN label IN ('10min Delivery', 'Recurring') THEN 'merchant'
    ELSE 'manual'
  END,
  CASE
    WHEN label IN ('10min Delivery', 'Recurring') THEN (
      SELECT merchant_info FROM transactions WHERE transactions.id = transaction_labels.transaction_id
    )
    ELSE ''
  END
FROM transaction_labels
WHERE transaction_id IN (
  SELECT id
  FROM transactions
  WHERE tenant_id = '00000000-0000-0000-0000-00000000c0de'
    AND message_id LIKE 'seed-msg-%'
)
ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING;

INSERT INTO labels (tenant_id, name, color) VALUES
  ('00000000-0000-0000-0000-00000000c0de', 'ContractLabel', '#f59e0b')
ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL DO UPDATE SET color = EXCLUDED.color;

INSERT INTO categories (tenant_id, name, description, is_default) VALUES
  ('00000000-0000-0000-0000-00000000c0de', 'ContractCategory', 'Stable contract-test category', false)
ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL DO UPDATE SET
  description = EXCLUDED.description,
  is_default = EXCLUDED.is_default;

INSERT INTO buckets (tenant_id, name, description, is_default) VALUES
  ('00000000-0000-0000-0000-00000000c0de', 'ContractBucket', 'Stable contract-test bucket', false)
ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL DO UPDATE SET
  description = EXCLUDED.description,
  is_default = EXCLUDED.is_default;

INSERT INTO transaction_labels (transaction_id, label) VALUES
  ('00000000-0000-0000-0000-000000000001', 'Online')
ON CONFLICT (transaction_id, label) DO NOTHING;

INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern) VALUES
  ('00000000-0000-0000-0000-000000000001', 'Online', 'manual', '')
ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING;

INSERT INTO rules (
  tenant_id,
  id,
  name,
  sender_email,
  sender_emails,
  subject_contains,
  amount_regex,
  merchant_regex,
  currency_regex,
  transaction_source,
  source_type,
  source_label,
  bank,
  predefined
) VALUES (
  '00000000-0000-0000-0000-00000000c0de',
  '00000000-0000-0000-0000-00000000c001',
  'Contract Existing Rule',
  'contract-existing@example.com',
  ARRAY['contract-existing@example.com'],
  'Contract transaction',
  'INR\s+([0-9,.]+)',
  'at\s+(.+)$',
  '(INR)',
  'Contract',
  'Email',
  'Contract',
  'Contract Bank',
  false
) ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  sender_email = EXCLUDED.sender_email,
  sender_emails = EXCLUDED.sender_emails,
  subject_contains = EXCLUDED.subject_contains,
  amount_regex = EXCLUDED.amount_regex,
  merchant_regex = EXCLUDED.merchant_regex,
  currency_regex = EXCLUDED.currency_regex,
  transaction_source = EXCLUDED.transaction_source,
  source_type = EXCLUDED.source_type,
  source_label = EXCLUDED.source_label,
  bank = EXCLUDED.bank,
  predefined = EXCLUDED.predefined,
  updated_at = NOW();

INSERT INTO extraction_diagnostics (
  tenant_id,
  id,
  status,
  reader,
  message_id,
  source,
  sender,
  sender_email,
  subject,
  email_body,
  received_at,
  snippet,
  rule_id,
  rule_name,
  amount_regex,
  merchant_regex,
  currency_regex,
  failure_reasons
) VALUES (
  '00000000-0000-0000-0000-00000000c0de',
  '00000000-0000-0000-0000-00000000c002',
  'open',
  'thunderbird',
  'contract-diagnostic-message',
  'Contract Bank',
  'Contract Bank',
  'contract@example.com',
  'Contract transaction',
  'Your card was charged INR 249.50 at Swiggy',
  '2026-05-31T06:00:00Z',
  'Your card was charged INR 249.50 at Swiggy',
  '00000000-0000-0000-0000-00000000c001',
  'Contract Existing Rule',
  'INR\s+([0-9,.]+)',
  'at\s+(.+)$',
  '(INR)',
  ARRAY['amount_not_found']
) ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  status = EXCLUDED.status,
  reader = EXCLUDED.reader,
  message_id = EXCLUDED.message_id,
  source = EXCLUDED.source,
  sender = EXCLUDED.sender,
  sender_email = EXCLUDED.sender_email,
  subject = EXCLUDED.subject,
  email_body = EXCLUDED.email_body,
  received_at = EXCLUDED.received_at,
  snippet = EXCLUDED.snippet,
  rule_id = EXCLUDED.rule_id,
  rule_name = EXCLUDED.rule_name,
  amount_regex = EXCLUDED.amount_regex,
  merchant_regex = EXCLUDED.merchant_regex,
  currency_regex = EXCLUDED.currency_regex,
  failure_reasons = EXCLUDED.failure_reasons,
  updated_at = NOW(),
  resolved_at = NULL;

INSERT INTO muted_merchants (tenant_id, id, pattern, reason) VALUES
  ('00000000-0000-0000-0000-00000000c0de', '00000000-0000-0000-0000-00000000c003', 'Contract Muted Merchant', 'contract check')
ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  pattern = EXCLUDED.pattern,
  reason = EXCLUDED.reason;
