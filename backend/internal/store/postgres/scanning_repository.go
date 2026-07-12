package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type scanningRepository struct {
	pool *pgxpool.Pool
}

func newScanningRepository(deps repositoryDependencies) *scanningRepository {
	return &scanningRepository{pool: deps.pool}
}

func (r *scanningRepository) GetSchedulerConfig(ctx context.Context) (store.SchedulerConfig, error) {
	var cfg store.SchedulerConfig
	err := r.pool.QueryRow(ctx, `
		SELECT max_concurrent_scans, updated_at
		FROM scheduler_config
		WHERE id = true
	`).Scan(&cfg.MaxConcurrentScans, &cfg.UpdatedAt)
	if err != nil {
		return store.SchedulerConfig{}, errors.E("postgres.scanning.get_scheduler_config", "getting scheduler config", err)
	}
	return cfg, nil
}

func (r *scanningRepository) PatchSchedulerConfig(ctx context.Context, patch store.SchedulerConfigPatch) (store.SchedulerConfig, error) {
	if patch.MaxConcurrentScans == nil {
		return r.GetSchedulerConfig(ctx)
	}
	if *patch.MaxConcurrentScans < 1 || *patch.MaxConcurrentScans > 64 {
		return store.SchedulerConfig{}, errors.E(errors.InvalidInput, "max concurrent scans must be between 1 and 64")
	}
	var cfg store.SchedulerConfig
	err := r.pool.QueryRow(ctx, `
		UPDATE scheduler_config
		SET max_concurrent_scans = $1, updated_at = now()
		WHERE id = true
		RETURNING max_concurrent_scans, updated_at
	`, *patch.MaxConcurrentScans).Scan(&cfg.MaxConcurrentScans, &cfg.UpdatedAt)
	if err != nil {
		return store.SchedulerConfig{}, errors.E("postgres.scanning.patch_scheduler_config", "patching scheduler config", err)
	}
	return cfg, nil
}

func (r *scanningRepository) EnsureScanningStateForTenant(ctx context.Context, tenant store.Tenant) error {
	tenantID, err := requireTenantID(tenant)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO tenant_scanning_state (tenant_id, active_reader, enabled, state)
		SELECT $1::uuid, '', true, 'stopped'
		FROM users u
		WHERE u.id = $1
		ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID)
	if err != nil {
		return errors.E("postgres.scanning.ensure_scanning_state_for_tenant", "ensuring scanning state for tenant", err)
	}
	return nil
}

func (r *scanningRepository) GetScanningState(ctx context.Context, tenant store.Tenant) (store.TenantScanningState, error) {
	if err := r.EnsureScanningStateForTenant(ctx, tenant); err != nil {
		return store.TenantScanningState{}, err
	}
	return r.fetchScanningState(ctx, tenant)
}

func (r *scanningRepository) ListRunnableScanningStates(ctx context.Context) ([]store.TenantScanningState, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tenant_id::text, active_reader, enabled, state, reason_code, public_message,
		       last_started_at, last_stopped_at, last_failed_at, next_retry_at, retry_count, updated_at
		FROM tenant_scanning_state
		WHERE enabled = true
		  AND active_reader <> ''
		  AND state NOT IN ('needs_auth', 'reader_not_configured', 'paused')
		  AND (next_retry_at IS NULL OR next_retry_at <= now())
		ORDER BY updated_at, tenant_id
	`)
	if err != nil {
		return nil, errors.E("postgres.scanning.list_runnable_scanning_states", "listing runnable scanning states", err)
	}
	defer rows.Close()

	states := make([]store.TenantScanningState, 0)
	for rows.Next() {
		state, err := scanTenantScanningState(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.E("postgres.scanning.list_runnable_scanning_states", "iterating runnable scanning states", err)
	}
	return states, nil
}

func (r *scanningRepository) ListScanningStates(ctx context.Context) ([]store.TenantScanningState, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tenant_id::text, active_reader, enabled, state, reason_code, public_message,
		       last_started_at, last_stopped_at, last_failed_at, next_retry_at, retry_count, updated_at
		FROM tenant_scanning_state
		ORDER BY updated_at DESC, tenant_id
	`)
	if err != nil {
		return nil, errors.E("postgres.scanning.list_scanning_states", "listing scanning states", err)
	}
	defer rows.Close()

	states := make([]store.TenantScanningState, 0)
	for rows.Next() {
		state, err := scanTenantScanningState(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.E("postgres.scanning.list_scanning_states", "iterating scanning states", err)
	}
	return states, nil
}

func (r *scanningRepository) SetActiveScanningReader(ctx context.Context, tenant store.Tenant, reader string) error {
	tenantID, err := requireTenantID(tenant)
	if err != nil {
		return err
	}
	reader = strings.TrimSpace(reader)
	state := store.ScanningStateQueued
	if reader == "" {
		state = store.ScanningStateStopped
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO tenant_scanning_state (tenant_id, active_reader, enabled, state, reason_code, public_message, retry_count, next_retry_at)
		VALUES ($1, $2, true, $3, '', '', 0, NULL)
		ON CONFLICT (tenant_id) DO UPDATE
		SET active_reader = EXCLUDED.active_reader,
		    enabled = true,
		    state = EXCLUDED.state,
		    reason_code = '',
		    public_message = '',
		    retry_count = 0,
		    next_retry_at = NULL,
		    updated_at = now()
	`, tenantID, reader, state)
	if err != nil {
		return errors.E("postgres.scanning.set_active_scanning_reader", "setting active scanning reader", err)
	}
	return nil
}

func (r *scanningRepository) ClearActiveScanningReader(ctx context.Context, tenant store.Tenant) error {
	tenantID, err := requireTenantID(tenant)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO tenant_scanning_state (tenant_id, active_reader, enabled, state, reason_code, public_message, retry_count, next_retry_at, last_stopped_at)
		VALUES ($1, '', false, 'stopped', '', '', 0, NULL, now())
		ON CONFLICT (tenant_id) DO UPDATE
		SET active_reader = '',
		    enabled = false,
		    state = 'stopped',
		    reason_code = '',
		    public_message = '',
		    retry_count = 0,
		    next_retry_at = NULL,
		    last_stopped_at = now(),
		    updated_at = now()
	`, tenantID)
	if err != nil {
		return errors.E("postgres.scanning.clear_active_scanning_reader", "clearing active scanning reader", err)
	}
	return nil
}

func (r *scanningRepository) SetScanningEnabled(ctx context.Context, tenant store.Tenant, enabled bool) error {
	tenantID, err := requireTenantID(tenant)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE tenant_scanning_state
		SET enabled = $2,
		    state = CASE
		        WHEN $2 = false THEN 'paused'
		        WHEN active_reader = '' THEN 'stopped'
		        ELSE 'queued'
		    END,
		    reason_code = '',
		    public_message = '',
		    next_retry_at = NULL,
		    updated_at = now()
		WHERE tenant_id = $1
	`, tenantID, enabled)
	if err != nil {
		return errors.E("postgres.scanning.set_scanning_enabled", "setting scanning enabled", err)
	}
	return nil
}

func (r *scanningRepository) UpdateScanningState(ctx context.Context, tenant store.Tenant, update store.ScanningStateUpdate) error {
	tenantID, err := requireTenantID(tenant)
	if err != nil {
		return err
	}
	retryCount := any(nil)
	if update.RetryCount != nil {
		retryCount = *update.RetryCount
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE tenant_scanning_state
		SET state = $2,
		    reason_code = $3,
		    public_message = $4,
		    last_started_at = COALESCE($5, last_started_at),
		    last_stopped_at = COALESCE($6, last_stopped_at),
		    last_failed_at = COALESCE($7, last_failed_at),
		    next_retry_at = $8,
		    retry_count = COALESCE($9, retry_count),
		    updated_at = now()
		WHERE tenant_id = $1
	`,
		tenantID, update.State, update.ReasonCode, update.PublicMessage, update.LastStartedAt,
		update.LastStoppedAt, update.LastFailedAt, update.NextRetryAt, retryCount,
	)
	if err != nil {
		return errors.E("postgres.scanning.update_scanning_state", "updating scanning state", err)
	}
	return nil
}

func (r *scanningRepository) fetchScanningState(ctx context.Context, tenant store.Tenant) (store.TenantScanningState, error) {
	var state store.TenantScanningState
	err := r.pool.QueryRow(ctx, `
		SELECT tenant_id::text, active_reader, enabled, state, reason_code, public_message,
		       last_started_at, last_stopped_at, last_failed_at, next_retry_at, retry_count, updated_at
		FROM tenant_scanning_state
		WHERE tenant_id = $1
	`, tenantIDParam(tenant)).Scan(
		&state.TenantID, &state.ActiveReader, &state.Enabled, &state.State, &state.ReasonCode, &state.PublicMessage,
		&state.LastStartedAt, &state.LastStoppedAt, &state.LastFailedAt, &state.NextRetryAt, &state.RetryCount, &state.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.TenantScanningState{}, notFound("store.scanning.get_state")
		}
		return store.TenantScanningState{}, errors.E("postgres.scanning.fetch_scanning_state", "getting scanning state", err)
	}
	return state, nil
}

func scanTenantScanningState(row pgx.Row) (store.TenantScanningState, error) {
	var state store.TenantScanningState
	err := row.Scan(
		&state.TenantID, &state.ActiveReader, &state.Enabled, &state.State, &state.ReasonCode, &state.PublicMessage,
		&state.LastStartedAt, &state.LastStoppedAt, &state.LastFailedAt, &state.NextRetryAt, &state.RetryCount, &state.UpdatedAt,
	)
	if err != nil {
		return store.TenantScanningState{}, errors.E("postgres.scanning.scan_tenant_scanning_state", "scanning tenant scanning state", err)
	}
	return state, nil
}

func requireTenantID(tenant store.Tenant) (string, error) {
	tenantID := strings.TrimSpace(tenant.ID)
	if tenantID == "" {
		return "", errors.E(errors.InvalidInput, "tenant id is required")
	}
	return tenantID, nil
}
