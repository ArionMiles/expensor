INSERT INTO app_config (key, value) VALUES
  ('base_currency', 'INR'),
  ('scan_interval', '120'),
  ('lookback_days', '365'),
  ('app.timezone', 'Asia/Kolkata'),
  ('app.time_format', 'HH:mm'),
  ('reader.gmail.last_scan_at', '2026-04-10T00:00:00Z')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO labels (name, color) VALUES
  ('Reimbursable', '#10b981'),
  ('Subscription', '#6366f1')
ON CONFLICT (name) DO UPDATE SET color = EXCLUDED.color;

INSERT INTO categories (name, description, is_default) VALUES
  ('Food & Dining', 'Seeded dining category', true),
  ('Utilities', 'Seeded utilities category', true),
  ('Travel', 'Seeded travel category', true)
ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO buckets (name, description, is_default) VALUES
  ('Needs', 'Seeded needs bucket', true),
  ('Wants', 'Seeded wants bucket', true),
  ('Investments', 'Seeded investments bucket', true)
ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO transactions (
  id,
  message_id,
  amount,
  currency,
  timestamp,
  merchant_info,
  category,
  bucket,
  source,
  description,
  muted,
  muted_by_merchant,
  mute_reason
) VALUES
  (
    '11111111-1111-1111-1111-111111111111',
    'seed-msg-1',
    1007.0000,
    'INR',
    '2026-04-16T04:58:45Z',
    'Food Merchant A',
    'Food & Dining',
    'Wants',
    'Credit Card - HDFC',
    'Seeded meal purchase',
    false,
    false,
    NULL
  ),
  (
    '22222222-2222-2222-2222-222222222222',
    'seed-msg-2',
    1295.6400,
    'INR',
    '2026-04-14T16:07:49Z',
    'Utility Merchant B',
    'Utilities',
    'Needs',
    'Credit Card - ICICI',
    'Seeded utility bill',
    false,
    false,
    NULL
  ),
  (
    '33333333-3333-3333-3333-333333333333',
    'seed-msg-3',
    15054.0000,
    'INR',
    '2026-04-12T19:04:01Z',
    'Travel Merchant C',
    'Travel',
    'Wants',
    'Credit Card - ICICI',
    'Seeded travel booking',
    false,
    false,
    NULL
  ),
  (
    '44444444-4444-4444-4444-444444444444',
    'seed-msg-4',
    518.0000,
    'INR',
    '2026-04-13T07:17:29Z',
    'Food Merchant D',
    'Food & Dining',
    'Wants',
    'Credit Card - HDFC',
    'Seeded recurring food order',
    true,
    false,
    'seeded muted case'
  )
ON CONFLICT (id) DO NOTHING;

INSERT INTO transaction_labels (transaction_id, label) VALUES
  ('11111111-1111-1111-1111-111111111111', 'Reimbursable'),
  ('44444444-4444-4444-4444-444444444444', 'Subscription')
ON CONFLICT (transaction_id, label) DO NOTHING;

INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern) VALUES
  ('11111111-1111-1111-1111-111111111111', 'Reimbursable', 'manual', ''),
  ('44444444-4444-4444-4444-444444444444', 'Subscription', 'manual', '')
ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING;
