package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const diagnosticColumns = `id, status, reader, COALESCE(message_id, ''), source, sender, sender_email,
	subject, email_body, received_at, snippet, rule_id, rule_name, amount_regex, merchant_regex,
	currency_regex, failure_reasons, created_at, updated_at, resolved_at`

func (s *Store) RecordExtractionDiagnostic(ctx context.Context, tenant store.Tenant, diagnostic api.ExtractionDiagnostic) error {
	if tenant.ID == "" {
		return errors.B.Op("sqlite.diagnostics.record").KindInvalidInput().Text("tenant is required").Build()
	}
	reasons, err := jsonArrayToText(diagnostic.FailureReasons)
	if err != nil {
		return err
	}
	return s.repositories.writeTx(ctx, func(q queryer) error {
		now := timeToText(s.repositories.now())
		_, err := q.ExecContext(ctx, `INSERT INTO extraction_diagnostics (
			id, tenant_id, reader, message_id, source, sender, sender_email, subject, email_body,
			received_at, snippet, rule_id, rule_name, amount_regex, merchant_regex, currency_regex,
			failure_reasons, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, reader, message_id, rule_name)
		WHERE tenant_id IS NOT NULL AND status='open' AND message_id IS NOT NULL DO UPDATE SET
			source=excluded.source, sender=excluded.sender, sender_email=excluded.sender_email,
			subject=excluded.subject, email_body=excluded.email_body, received_at=excluded.received_at,
			snippet=excluded.snippet, rule_id=excluded.rule_id, amount_regex=excluded.amount_regex,
			merchant_regex=excluded.merchant_regex, currency_regex=excluded.currency_regex,
			failure_reasons=excluded.failure_reasons, updated_at=excluded.updated_at`,
			uuid.NewString(), tenant.ID, diagnostic.Reader, nullableDiagnosticString(diagnostic.MessageID),
			diagnostic.Source, diagnosticSender(diagnostic), diagnostic.SenderEmail, diagnostic.Subject,
			diagnostic.EmailBody, nullableDiagnosticTime(diagnostic.ReceivedAt), diagnostic.Snippet,
			nullableDiagnosticString(diagnostic.RuleID), diagnostic.RuleName, diagnostic.AmountRegex,
			diagnostic.MerchantRegex, diagnostic.CurrencyRegex, reasons, now, now)
		return mapSQLiteError("sqlite.diagnostics.record", err)
	})
}

func diagnosticSender(diagnostic api.ExtractionDiagnostic) string {
	if diagnostic.Sender != "" {
		return diagnostic.Sender
	}
	return diagnostic.SenderEmail
}

func nullableDiagnosticString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableDiagnosticTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return timeToText(*value)
}

func (s *Store) ListExtractionDiagnostics(ctx context.Context, tenant store.Tenant, filter store.DiagnosticFilter) ([]store.ExtractionDiagnosticRow, error) {
	if err := store.ValidateDiagnosticFilterStatus(filter.Status); err != nil {
		return nil, err
	}
	query := `SELECT ` + diagnosticColumns + ` FROM extraction_diagnostics WHERE tenant_id=?`
	args := []any{tenant.ID}
	if filter.Status != store.DiagnosticStatusAll {
		query += ` AND status=?`
		args = append(args, filter.Status)
	}
	query += ` ORDER BY created_at DESC, id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.repositories.query.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, mapSQLiteError("sqlite.diagnostics.list", err)
	}
	defer rows.Close()
	return scanDiagnosticRows(rows)
}

func (s *Store) GetExtractionDiagnostic(ctx context.Context, tenant store.Tenant, id string) (*store.ExtractionDiagnosticRow, error) {
	row := s.repositories.query.QueryRowContext(ctx, `SELECT `+diagnosticColumns+`
		FROM extraction_diagnostics WHERE id=? AND tenant_id=?`, id, tenant.ID)
	result, err := scanDiagnostic(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.B.Op("store.diagnostics.get").KindNotFound().UserMsg("extraction diagnostic not found").Build()
		}
		return nil, errors.B.Op("sqlite.diagnostics.get").Text("fetching extraction diagnostic").Err(err).Build()
	}
	return &result, nil
}

func (s *Store) UpdateExtractionDiagnosticStatus(ctx context.Context, tenant store.Tenant, id, status string) (*store.ExtractionDiagnosticRow, error) {
	if err := store.ValidateDiagnosticUpdateStatus(status); err != nil {
		return nil, err
	}
	var result store.ExtractionDiagnosticRow
	err := s.repositories.writeTx(ctx, func(q queryer) error {
		now := timeToText(s.repositories.now())
		resolvedAt := any(now)
		if status == store.DiagnosticStatusOpen {
			resolvedAt = nil
		}
		row := q.QueryRowContext(ctx, `UPDATE extraction_diagnostics
			SET status=?, resolved_at=?, updated_at=? WHERE id=? AND tenant_id=? RETURNING `+diagnosticColumns,
			status, resolvedAt, now, id, tenant.ID)
		var scanErr error
		result, scanErr = scanDiagnostic(row)
		return mapSQLiteError("sqlite.diagnostics.update_status", scanErr)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.WhatKind(err) == errors.NotFound {
			return nil, errors.B.Op("store.diagnostics.update_status").KindNotFound().UserMsg("extraction diagnostic not found").Build()
		}
		if errors.WhatKind(err) == errors.Conflict {
			return nil, errors.B.Op("store.diagnostics.update_status").KindConflict().UserMsg("open extraction diagnostic already exists").Err(err).Build()
		}
		return nil, errors.B.Op("sqlite.diagnostics.update_status").Text("updating extraction diagnostic status").Err(err).Build()
	}
	return &result, nil
}

type diagnosticScanner interface {
	Scan(...any) error
}

func scanDiagnostic(scanner diagnosticScanner) (store.ExtractionDiagnosticRow, error) {
	var result store.ExtractionDiagnosticRow
	var messageID string
	var receivedAt, ruleID, resolvedAt sql.NullString
	var reasons, createdAt, updatedAt string
	if err := scanner.Scan(&result.ID, &result.Status, &result.Reader, &messageID, &result.Source,
		&result.Sender, &result.SenderEmail, &result.Subject, &result.EmailBody, &receivedAt,
		&result.Snippet, &ruleID, &result.RuleName, &result.AmountRegex, &result.MerchantRegex,
		&result.CurrencyRegex, &reasons, &createdAt, &updatedAt, &resolvedAt); err != nil {
		return result, err
	}
	result.MessageID = messageID
	if ruleID.Valid {
		result.RuleID = &ruleID.String
	}
	if err := jsonArrayFromText(reasons, &result.FailureReasons); err != nil {
		return result, err
	}
	created, err := timeFromText(createdAt)
	if err != nil {
		return result, err
	}
	updated, err := timeFromText(updatedAt)
	if err != nil {
		return result, err
	}
	result.CreatedAt, result.UpdatedAt = created, updated
	if receivedAt.Valid {
		value, err := timeFromText(receivedAt.String)
		if err != nil {
			return result, err
		}
		result.ReceivedAt = &value
	}
	if resolvedAt.Valid {
		value, err := timeFromText(resolvedAt.String)
		if err != nil {
			return result, err
		}
		result.ResolvedAt = &value
	}
	return result, nil
}

func scanDiagnosticRows(rows *sql.Rows) ([]store.ExtractionDiagnosticRow, error) {
	result := []store.ExtractionDiagnosticRow{}
	for rows.Next() {
		row, err := scanDiagnostic(rows)
		if err != nil {
			return nil, errors.B.Op("sqlite.diagnostics.scan").Text("scanning extraction diagnostic").Err(err).Build()
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, mapSQLiteError("sqlite.diagnostics.scan", err)
	}
	return result, nil
}
