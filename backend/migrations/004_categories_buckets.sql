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
    ('Food & Dining', true),
    ('Transport',     true),
    ('Shopping',      true),
    ('Utilities',     true),
    ('Healthcare',    true),
    ('Entertainment', true),
    ('Travel',        true),
    ('Finance',       true),
    ('Uncategorized', true)
ON CONFLICT (name) DO NOTHING;

INSERT INTO buckets (name, is_default) VALUES
    ('Needs',   true),
    ('Wants',   true),
    ('Savings', true),
    ('Income',  true)
ON CONFLICT (name) DO NOTHING;
