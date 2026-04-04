-- Documentation only — this DDL is executed by store.initRules() called from store.New().

DO $$ BEGIN
    CREATE TYPE rule_source AS ENUM ('system', 'user');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS rules (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL,
    sender_email     TEXT NOT NULL DEFAULT '',
    subject_contains TEXT NOT NULL DEFAULT '',
    amount_regex     TEXT NOT NULL,
    merchant_regex   TEXT NOT NULL,
    currency_regex   TEXT NOT NULL DEFAULT '',
    enabled          BOOLEAN NOT NULL DEFAULT true,
    source           rule_source NOT NULL DEFAULT 'user',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (name, source)
);
