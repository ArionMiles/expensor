CREATE TABLE IF NOT EXISTS labels (
    name        TEXT PRIMARY KEY,
    color       TEXT NOT NULL DEFAULT '#6366f1',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO labels (name, color) VALUES
    ('food',          '#f59e0b'),
    ('transport',     '#3b82f6'),
    ('shopping',      '#8b5cf6'),
    ('utilities',     '#06b6d4'),
    ('healthcare',    '#10b981'),
    ('entertainment', '#ec4899'),
    ('travel',        '#f97316'),
    ('recurring',     '#6366f1')
ON CONFLICT (name) DO NOTHING;
