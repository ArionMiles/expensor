CREATE TABLE IF NOT EXISTS llm_provider_runtime (
    tenant_id uuid REFERENCES users(id) ON DELETE CASCADE,
    provider text NOT NULL,
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    credentials_ciphertext bytea,
    active boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS llm_provider_runtime_legacy_provider_key
    ON llm_provider_runtime (provider)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS llm_provider_runtime_tenant_provider_key
    ON llm_provider_runtime (tenant_id, provider)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS llm_provider_runtime_legacy_active_key
    ON llm_provider_runtime (active)
    WHERE tenant_id IS NULL AND active = true;

CREATE UNIQUE INDEX IF NOT EXISTS llm_provider_runtime_tenant_active_key
    ON llm_provider_runtime (tenant_id)
    WHERE tenant_id IS NOT NULL AND active = true;
