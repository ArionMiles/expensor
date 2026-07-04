CREATE TABLE IF NOT EXISTS scheduler_config (
    id boolean PRIMARY KEY DEFAULT true,
    max_concurrent_scans integer NOT NULL DEFAULT 4 CHECK (max_concurrent_scans >= 1 AND max_concurrent_scans <= 64),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT scheduler_config_singleton CHECK (id)
);

INSERT INTO scheduler_config (id, max_concurrent_scans)
VALUES (true, 4)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS tenant_scanning_state (
    tenant_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    active_reader text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT true,
    state text NOT NULL DEFAULT 'stopped',
    reason_code text NOT NULL DEFAULT '',
    public_message text NOT NULL DEFAULT '',
    last_started_at timestamptz,
    last_stopped_at timestamptz,
    last_failed_at timestamptz,
    next_retry_at timestamptz,
    retry_count integer NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (state IN ('queued', 'starting', 'running', 'backing_off', 'needs_auth', 'reader_not_configured', 'paused', 'stopped'))
);

INSERT INTO tenant_scanning_state (tenant_id, active_reader, enabled, state)
SELECT u.id, COALESCE(ac.value, ''), true,
       CASE WHEN COALESCE(ac.value, '') = '' THEN 'stopped' ELSE 'queued' END
FROM users u
LEFT JOIN app_config ac ON ac.tenant_id = u.id AND ac.key = 'active_reader'
ON CONFLICT (tenant_id) DO UPDATE
SET active_reader = EXCLUDED.active_reader,
    enabled = EXCLUDED.enabled,
    state = EXCLUDED.state,
    updated_at = now();
