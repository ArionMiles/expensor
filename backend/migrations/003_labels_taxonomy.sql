CREATE TABLE IF NOT EXISTS labels (
    name        TEXT PRIMARY KEY,
    color       TEXT NOT NULL DEFAULT '#6366f1',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO labels (name, color) VALUES
    ('Food',          '#f59e0b'),
    ('Transport',     '#3b82f6'),
    ('Shopping',      '#8b5cf6'),
    ('Utilities',     '#06b6d4'),
    ('Healthcare',    '#10b981'),
    ('Entertainment', '#ec4899'),
    ('Travel',        '#f97316'),
    ('Recurring',     '#6366f1')
ON CONFLICT (name) DO NOTHING;
