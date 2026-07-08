package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScanningState string

const (
	ScanningStateQueued              ScanningState = "queued"
	ScanningStateStarting            ScanningState = "starting"
	ScanningStateRunning             ScanningState = "running"
	ScanningStateBackingOff          ScanningState = "backing_off"
	ScanningStateNeedsAuth           ScanningState = "needs_auth"
	ScanningStateReaderNotConfigured ScanningState = "reader_not_configured"
	ScanningStatePaused              ScanningState = "paused"
	ScanningStateStopped             ScanningState = "stopped"
)

type ScanningReasonCode string

const (
	ScanningReasonNone                ScanningReasonCode = ""
	ScanningReasonMissingCredentials  ScanningReasonCode = "needs_auth_missing_credentials"
	ScanningReasonMissingToken        ScanningReasonCode = "needs_auth_missing_token"
	ScanningReasonInvalidGrant        ScanningReasonCode = "needs_auth_invalid_grant"
	ScanningReasonReaderNotConfigured ScanningReasonCode = "reader_not_configured"
	ScanningReasonTemporaryFailure    ScanningReasonCode = "temporary_failure"
)

type SchedulerConfig struct {
	MaxConcurrentScans int
	UpdatedAt          time.Time
}

type SchedulerConfigPatch struct {
	MaxConcurrentScans *int
}

type TenantScanningState struct {
	TenantID      string
	ActiveReader  string
	Enabled       bool
	State         ScanningState
	ReasonCode    ScanningReasonCode
	PublicMessage string
	LastStartedAt *time.Time
	LastStoppedAt *time.Time
	LastFailedAt  *time.Time
	NextRetryAt   *time.Time
	RetryCount    int
	UpdatedAt     time.Time
}

type ScanningStateUpdate struct {
	State         ScanningState
	ReasonCode    ScanningReasonCode
	PublicMessage string
	LastStartedAt *time.Time
	LastStoppedAt *time.Time
	LastFailedAt  *time.Time
	NextRetryAt   *time.Time
	RetryCount    *int
}

type pgScanningRepository struct {
	pool *pgxpool.Pool
}

func newPGScanningRepository(deps repositoryDependencies) *pgScanningRepository {
	return &pgScanningRepository{pool: deps.pool}
}

func (r *pgScanningRepository) GetSchedulerConfig(ctx context.Context) (SchedulerConfig, error) {
	var cfg SchedulerConfig
	err := r.pool.QueryRow(ctx, `
		SELECT max_concurrent_scans, updated_at
		FROM scheduler_config
		WHERE id = true
	`).Scan(&cfg.MaxConcurrentScans, &cfg.UpdatedAt)
	if err != nil {
		return SchedulerConfig{}, fmt.Errorf("getting scheduler config: %w", err)
	}
	return cfg, nil
}

func (r *pgScanningRepository) PatchSchedulerConfig(ctx context.Context, patch SchedulerConfigPatch) (SchedulerConfig, error) {
	if patch.MaxConcurrentScans == nil {
		return r.GetSchedulerConfig(ctx)
	}
	if *patch.MaxConcurrentScans < 1 || *patch.MaxConcurrentScans > 64 {
		return SchedulerConfig{}, errors.New("max concurrent scans must be between 1 and 64")
	}
	var cfg SchedulerConfig
	err := r.pool.QueryRow(ctx, `
		UPDATE scheduler_config
		SET max_concurrent_scans = $1, updated_at = now()
		WHERE id = true
		RETURNING max_concurrent_scans, updated_at
	`, *patch.MaxConcurrentScans).Scan(&cfg.MaxConcurrentScans, &cfg.UpdatedAt)
	if err != nil {
		return SchedulerConfig{}, fmt.Errorf("patching scheduler config: %w", err)
	}
	return cfg, nil
}

func (r *pgScanningRepository) EnsureScanningStateForTenant(ctx context.Context, tenant Tenant) error {
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
		return fmt.Errorf("ensuring scanning state for tenant: %w", err)
	}
	return nil
}

func (r *pgScanningRepository) GetScanningState(ctx context.Context, tenant Tenant) (TenantScanningState, error) {
	if err := r.EnsureScanningStateForTenant(ctx, tenant); err != nil {
		return TenantScanningState{}, err
	}
	return r.fetchScanningState(ctx, tenant)
}

func (r *pgScanningRepository) ListRunnableScanningStates(ctx context.Context) ([]TenantScanningState, error) {
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
		return nil, fmt.Errorf("listing runnable scanning states: %w", err)
	}
	defer rows.Close()

	states := make([]TenantScanningState, 0)
	for rows.Next() {
		state, err := scanTenantScanningState(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating runnable scanning states: %w", err)
	}
	return states, nil
}

func (r *pgScanningRepository) ListScanningStates(ctx context.Context) ([]TenantScanningState, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tenant_id::text, active_reader, enabled, state, reason_code, public_message,
		       last_started_at, last_stopped_at, last_failed_at, next_retry_at, retry_count, updated_at
		FROM tenant_scanning_state
		ORDER BY updated_at DESC, tenant_id
	`)
	if err != nil {
		return nil, fmt.Errorf("listing scanning states: %w", err)
	}
	defer rows.Close()

	states := make([]TenantScanningState, 0)
	for rows.Next() {
		state, err := scanTenantScanningState(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating scanning states: %w", err)
	}
	return states, nil
}

func (r *pgScanningRepository) SetActiveScanningReader(ctx context.Context, tenant Tenant, reader string) error {
	tenantID, err := requireTenantID(tenant)
	if err != nil {
		return err
	}
	reader = strings.TrimSpace(reader)
	state := ScanningStateQueued
	if reader == "" {
		state = ScanningStateStopped
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
		return fmt.Errorf("setting active scanning reader: %w", err)
	}
	return nil
}

func (r *pgScanningRepository) ClearActiveScanningReader(ctx context.Context, tenant Tenant) error {
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
		return fmt.Errorf("clearing active scanning reader: %w", err)
	}
	return nil
}

func (r *pgScanningRepository) SetScanningEnabled(ctx context.Context, tenant Tenant, enabled bool) error {
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
		return fmt.Errorf("setting scanning enabled: %w", err)
	}
	return nil
}

func (r *pgScanningRepository) UpdateScanningState(ctx context.Context, tenant Tenant, update ScanningStateUpdate) error {
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
		return fmt.Errorf("updating scanning state: %w", err)
	}
	return nil
}

func (r *pgScanningRepository) fetchScanningState(ctx context.Context, tenant Tenant) (TenantScanningState, error) {
	var state TenantScanningState
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
			return TenantScanningState{}, notFound("store.scanning.get_state")
		}
		return TenantScanningState{}, fmt.Errorf("getting scanning state: %w", err)
	}
	return state, nil
}

func scanTenantScanningState(row pgx.Row) (TenantScanningState, error) {
	var state TenantScanningState
	err := row.Scan(
		&state.TenantID, &state.ActiveReader, &state.Enabled, &state.State, &state.ReasonCode, &state.PublicMessage,
		&state.LastStartedAt, &state.LastStoppedAt, &state.LastFailedAt, &state.NextRetryAt, &state.RetryCount, &state.UpdatedAt,
	)
	if err != nil {
		return TenantScanningState{}, fmt.Errorf("scanning tenant scanning state: %w", err)
	}
	return state, nil
}

func requireTenantID(tenant Tenant) (string, error) {
	tenantID := strings.TrimSpace(tenant.ID)
	if tenantID == "" {
		return "", errors.New("tenant id is required")
	}
	return tenantID, nil
}
