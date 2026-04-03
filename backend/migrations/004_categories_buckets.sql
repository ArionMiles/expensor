CREATE TABLE IF NOT EXISTS categories (
    name        TEXT PRIMARY KEY,
    description TEXT,
    is_default  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS buckets (
    name        TEXT PRIMARY KEY,
    description TEXT,
    is_default  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO categories (name, is_default) VALUES
    ('food & dining', true),
    ('transport',     true),
    ('shopping',      true),
    ('utilities',     true),
    ('healthcare',    true),
    ('entertainment', true),
    ('travel',        true),
    ('finance',       true),
    ('uncategorized', true)
ON CONFLICT (name) DO NOTHING;

INSERT INTO buckets (name, is_default) VALUES
    ('needs',   true),
    ('wants',   true),
    ('savings', true),
    ('income',  true)
ON CONFLICT (name) DO NOTHING;
