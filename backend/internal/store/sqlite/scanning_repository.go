package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const scanningStateColumns = `tenant_id, active_reader, enabled, state, reason_code, public_message,
	last_started_at, last_stopped_at, last_failed_at, next_retry_at, retry_count, updated_at`

func (s *Store) GetSchedulerConfig(ctx context.Context) (store.SchedulerConfig, error) {
	var cfg store.SchedulerConfig
	var updatedAt string
	err := s.repositories.query.QueryRowContext(
		ctx,
		`SELECT max_concurrent_scans, updated_at FROM scheduler_config WHERE id = 1`,
	).Scan(&cfg.MaxConcurrentScans, &updatedAt)
	if err != nil {
		return store.SchedulerConfig{}, mapSQLiteError("sqlite.scanning.get_scheduler_config", err)
	}
	cfg.UpdatedAt, err = timeFromText(updatedAt)
	if err != nil {
		return store.SchedulerConfig{}, errors.B.Op("sqlite.scanning.get_scheduler_config").Text("reading updated timestamp").Err(err).Build()
	}
	return cfg, nil
}

func (s *Store) PatchSchedulerConfig(ctx context.Context, patch store.SchedulerConfigPatch) (store.SchedulerConfig, error) {
	if patch.MaxConcurrentScans == nil {
		return s.GetSchedulerConfig(ctx)
	}
	if *patch.MaxConcurrentScans < 1 || *patch.MaxConcurrentScans > 64 {
		return store.SchedulerConfig{}, errors.B.KindInvalidInput().Text("max concurrent scans must be between 1 and 64").Build()
	}
	var cfg store.SchedulerConfig
	var updatedAt string
	now := timeToText(s.repositories.now())
	err := s.repositories.writeTx(ctx, func(query queryer) error {
		err := query.QueryRowContext(ctx, `
			UPDATE scheduler_config SET max_concurrent_scans = ?, updated_at = ? WHERE id = 1
			RETURNING max_concurrent_scans, updated_at
		`, *patch.MaxConcurrentScans, now).Scan(&cfg.MaxConcurrentScans, &updatedAt)
		return mapSQLiteError("sqlite.scanning.patch_scheduler_config", err)
	})
	if err != nil {
		return store.SchedulerConfig{}, err
	}
	cfg.UpdatedAt, err = timeFromText(updatedAt)
	if err != nil {
		return store.SchedulerConfig{}, errors.B.Op("sqlite.scanning.patch_scheduler_config").Text("reading updated timestamp").Err(err).Build()
	}
	return cfg, nil
}

func (s *Store) EnsureScanningStateForTenant(ctx context.Context, tenant store.Tenant) error {
	tenantID, err := requireSQLiteTenantID(tenant)
	if err != nil {
		return err
	}
	now := timeToText(s.repositories.now())
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			INSERT INTO tenant_scanning_state (tenant_id, active_reader, enabled, state, created_at, updated_at)
			SELECT ?, '', 1, 'stopped', ?, ? FROM users WHERE id = ?
			ON CONFLICT (tenant_id) DO NOTHING
		`, tenantID, now, now, tenantID)
		return mapSQLiteError("sqlite.scanning.ensure_scanning_state_for_tenant", err)
	})
}

func (s *Store) GetScanningState(ctx context.Context, tenant store.Tenant) (store.TenantScanningState, error) {
	if err := s.EnsureScanningStateForTenant(ctx, tenant); err != nil {
		return store.TenantScanningState{}, err
	}
	row := s.repositories.query.QueryRowContext(ctx, `SELECT `+scanningStateColumns+` FROM tenant_scanning_state WHERE tenant_id = ?`, tenant.ID)
	state, err := scanSQLiteScanningState(row)
	if errors.Is(err, sql.ErrNoRows) || errors.WhatKind(err) == errors.NotFound {
		return store.TenantScanningState{}, errors.B.Op("store.scanning.get_state").KindNotFound().Build()
	}
	return state, err
}

func (s *Store) ListRunnableScanningStates(ctx context.Context) ([]store.TenantScanningState, error) {
	now := timeToText(s.repositories.now())
	return s.queryScanningStates(ctx, "sqlite.scanning.list_runnable_scanning_states", `
		SELECT `+scanningStateColumns+` FROM tenant_scanning_state
		WHERE enabled = 1 AND active_reader <> ''
		  AND state NOT IN ('needs_auth', 'reader_not_configured', 'paused')
		  AND (next_retry_at IS NULL OR next_retry_at <= ?)
		ORDER BY updated_at, tenant_id
	`, now)
}

func (s *Store) ListScanningStates(ctx context.Context) ([]store.TenantScanningState, error) {
	return s.queryScanningStates(ctx, "sqlite.scanning.list_scanning_states", `
		SELECT `+scanningStateColumns+` FROM tenant_scanning_state ORDER BY updated_at DESC, tenant_id
	`)
}

func (s *Store) SetActiveScanningReader(ctx context.Context, tenant store.Tenant, reader string) error {
	tenantID, err := requireSQLiteTenantID(tenant)
	if err != nil {
		return err
	}
	reader = strings.TrimSpace(reader)
	state := store.ScanningStateQueued
	if reader == "" {
		state = store.ScanningStateStopped
	}
	now := timeToText(s.repositories.now())
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			INSERT INTO tenant_scanning_state
				(tenant_id, active_reader, enabled, state, reason_code, public_message, retry_count, next_retry_at, created_at, updated_at)
			VALUES (?, ?, 1, ?, '', '', 0, NULL, ?, ?)
			ON CONFLICT (tenant_id) DO UPDATE SET
				active_reader = excluded.active_reader, enabled = 1, state = excluded.state,
				reason_code = '', public_message = '', retry_count = 0, next_retry_at = NULL,
				updated_at = excluded.updated_at
		`, tenantID, reader, state, now, now)
		return mapSQLiteError("sqlite.scanning.set_active_scanning_reader", err)
	})
}

func (s *Store) ClearActiveScanningReader(ctx context.Context, tenant store.Tenant) error {
	tenantID, err := requireSQLiteTenantID(tenant)
	if err != nil {
		return err
	}
	now := timeToText(s.repositories.now())
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			INSERT INTO tenant_scanning_state
				(tenant_id, active_reader, enabled, state, reason_code, public_message, retry_count, next_retry_at, last_stopped_at, created_at, updated_at)
			VALUES (?, '', 0, 'stopped', '', '', 0, NULL, ?, ?, ?)
			ON CONFLICT (tenant_id) DO UPDATE SET
				active_reader = '', enabled = 0, state = 'stopped', reason_code = '', public_message = '',
				retry_count = 0, next_retry_at = NULL, last_stopped_at = excluded.last_stopped_at,
				updated_at = excluded.updated_at
		`, tenantID, now, now, now)
		return mapSQLiteError("sqlite.scanning.clear_active_scanning_reader", err)
	})
}

func (s *Store) SetScanningEnabled(ctx context.Context, tenant store.Tenant, enabled bool) error {
	tenantID, err := requireSQLiteTenantID(tenant)
	if err != nil {
		return err
	}
	now := timeToText(s.repositories.now())
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			UPDATE tenant_scanning_state SET
				enabled = ?,
				state = CASE WHEN ? = 0 THEN 'paused' WHEN active_reader = '' THEN 'stopped' ELSE 'queued' END,
				reason_code = '', public_message = '', next_retry_at = NULL, updated_at = ?
			WHERE tenant_id = ?
		`, enabled, enabled, now, tenantID)
		return mapSQLiteError("sqlite.scanning.set_scanning_enabled", err)
	})
}

func (s *Store) UpdateScanningState(ctx context.Context, tenant store.Tenant, update store.ScanningStateUpdate) error {
	tenantID, err := requireSQLiteTenantID(tenant)
	if err != nil {
		return err
	}
	if update.RetryCount != nil && *update.RetryCount < 0 {
		return errors.B.Op("store.scanning.update_scanning_state").KindInvalidInput().Text("retry count cannot be negative").Build()
	}
	var retryCount any
	if update.RetryCount != nil {
		retryCount = *update.RetryCount
	}
	now := timeToText(s.repositories.now())
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			UPDATE tenant_scanning_state SET
				state = ?, reason_code = ?, public_message = ?,
				last_started_at = COALESCE(?, last_started_at),
				last_stopped_at = COALESCE(?, last_stopped_at),
				last_failed_at = COALESCE(?, last_failed_at),
				next_retry_at = ?, retry_count = COALESCE(?, retry_count), updated_at = ?
			WHERE tenant_id = ?
		`, update.State, update.ReasonCode, update.PublicMessage,
			nullableScanningTime(update.LastStartedAt), nullableScanningTime(update.LastStoppedAt), nullableScanningTime(update.LastFailedAt),
			nullableScanningTime(update.NextRetryAt), retryCount, now, tenantID)
		return mapSQLiteError("sqlite.scanning.update_scanning_state", err)
	})
}

func (s *Store) queryScanningStates(ctx context.Context, op, statement string, args ...any) ([]store.TenantScanningState, error) {
	rows, err := s.repositories.query.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, mapSQLiteError(op, err)
	}
	defer rows.Close()

	states := make([]store.TenantScanningState, 0)
	for rows.Next() {
		state, err := scanSQLiteScanningState(rows)
		if err != nil {
			return nil, errors.B.Op(op).Text("scanning state row").Err(err).Build()
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, mapSQLiteError(op, err)
	}
	return states, nil
}

type sqliteRowScanner interface {
	Scan(dest ...any) error
}

func scanSQLiteScanningState(row sqliteRowScanner) (store.TenantScanningState, error) {
	var state store.TenantScanningState
	var lastStarted, lastStopped, lastFailed, nextRetry sql.NullString
	var updatedAt string
	err := row.Scan(
		&state.TenantID, &state.ActiveReader, &state.Enabled, &state.State, &state.ReasonCode, &state.PublicMessage,
		&lastStarted, &lastStopped, &lastFailed, &nextRetry, &state.RetryCount, &updatedAt,
	)
	if err != nil {
		return store.TenantScanningState{}, mapSQLiteError("sqlite.scanning.scan_state", err)
	}
	if state.LastStartedAt, err = scanningTimeFromNull(lastStarted); err != nil {
		return store.TenantScanningState{}, err
	}
	if state.LastStoppedAt, err = scanningTimeFromNull(lastStopped); err != nil {
		return store.TenantScanningState{}, err
	}
	if state.LastFailedAt, err = scanningTimeFromNull(lastFailed); err != nil {
		return store.TenantScanningState{}, err
	}
	if state.NextRetryAt, err = scanningTimeFromNull(nextRetry); err != nil {
		return store.TenantScanningState{}, err
	}
	state.UpdatedAt, err = timeFromText(updatedAt)
	if err != nil {
		return store.TenantScanningState{}, err
	}
	return state, nil
}

func requireSQLiteTenantID(tenant store.Tenant) (string, error) {
	tenantID := strings.TrimSpace(tenant.ID)
	if tenantID == "" {
		return "", errors.B.KindInvalidInput().Text("tenant id is required").Build()
	}
	return tenantID, nil
}

func nullableScanningTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return timeToText(*value)
}

func scanningTimeFromNull(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := timeFromText(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
