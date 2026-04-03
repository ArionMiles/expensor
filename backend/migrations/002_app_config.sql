CREATE TABLE IF NOT EXISTS app_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO app_config (key, value) VALUES ('base_currency', 'INR')
ON CONFLICT (key) DO NOTHING;
