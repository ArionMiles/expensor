package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const recordExtractionDiagnosticSQL = `
	INSERT INTO extraction_diagnostics (
		tenant_id, reader, message_id, source, sender, sender_email, subject, email_body, received_at, snippet,
		rule_id, rule_name, amount_regex, merchant_regex, currency_regex, failure_reasons
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	ON CONFLICT (tenant_id, reader, message_id, rule_name) WHERE tenant_id IS NOT NULL AND status = 'open' AND message_id IS NOT NULL
	DO UPDATE SET
	    source = EXCLUDED.source,
	    sender = EXCLUDED.sender,
	    sender_email = EXCLUDED.sender_email,
	    subject = EXCLUDED.subject,
	    email_body = EXCLUDED.email_body,
	    received_at = EXCLUDED.received_at,
	    snippet = EXCLUDED.snippet,
	    rule_id = EXCLUDED.rule_id,
	    amount_regex = EXCLUDED.amount_regex,
	    merchant_regex = EXCLUDED.merchant_regex,
	    currency_regex = EXCLUDED.currency_regex,
	    failure_reasons = EXCLUDED.failure_reasons,
	    updated_at = NOW()
`

const diagnosticColumns = `
	id::text, status, reader, COALESCE(message_id, ''), source, sender, sender_email, subject, email_body,
	received_at, snippet, rule_id::text, rule_name, amount_regex, merchant_regex, currency_regex, failure_reasons,
	created_at, updated_at, resolved_at
`

type diagnosticsRepository struct {
	pool *pgxpool.Pool
}

func newDiagnosticsRepository(deps repositoryDependencies) *diagnosticsRepository {
	return &diagnosticsRepository{
		pool: deps.pool,
	}
}

func (r *diagnosticsRepository) RecordExtractionDiagnostic(ctx context.Context, tenant store.Tenant, diagnostic api.ExtractionDiagnostic) error {
	if tenant.ID == "" {
		return errors.E("postgres.diagnostics.record_extraction", errors.InvalidInput, "tenant is required")
	}

	_, err := r.pool.Exec(ctx, recordExtractionDiagnosticSQL,
		tenant.ID,
		diagnostic.Reader,
		nullableString(diagnostic.MessageID),
		diagnostic.Source,
		diagnosticSender(diagnostic),
		diagnostic.SenderEmail,
		diagnostic.Subject,
		diagnostic.EmailBody,
		diagnostic.ReceivedAt,
		diagnostic.Snippet,
		nullableString(diagnostic.RuleID),
		diagnostic.RuleName,
		diagnostic.AmountRegex,
		diagnostic.MerchantRegex,
		diagnostic.CurrencyRegex,
		diagnosticFailureReasons(diagnostic.FailureReasons),
	)
	if err != nil {
		return fmt.Errorf("recording extraction diagnostic: %w", err)
	}
	return nil
}

func (r *diagnosticsRepository) ListExtractionDiagnostics(
	ctx context.Context,
	tenant store.Tenant,
	f store.DiagnosticFilter,
) ([]store.ExtractionDiagnosticRow, error) {
	if err := store.ValidateDiagnosticFilterStatus(f.Status); err != nil {
		return nil, err
	}

	query := `SELECT ` + diagnosticColumns + ` FROM extraction_diagnostics`
	args := []any{tenant.ID}
	query += ` WHERE tenant_id = $1`
	if f.Status != store.DiagnosticStatusAll {
		query += ` AND status = $2`
		args = append(args, f.Status)
	}
	query += ` ORDER BY created_at DESC`
	if f.Limit > 0 {
		args = append(args, f.Limit)
		query += fmt.Sprintf(` LIMIT $%d`, len(args))
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing extraction diagnostics: %w", err)
	}
	defer rows.Close()
	result, err := scanDiagnosticRows(rows)
	if err != nil {
		return nil, fmt.Errorf("listing extraction diagnostics: %w", err)
	}
	return result, nil
}

func (r *diagnosticsRepository) GetExtractionDiagnostic(ctx context.Context, tenant store.Tenant, id string) (*store.ExtractionDiagnosticRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+diagnosticColumns+` FROM extraction_diagnostics WHERE id = $1 AND tenant_id = $2`,
		id, tenant.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("fetching extraction diagnostic: %w", err)
	}
	defer rows.Close()
	result, err := scanDiagnosticRows(rows)
	if err != nil {
		return nil, fmt.Errorf("fetching extraction diagnostic: %w", err)
	}
	if len(result) == 0 {
		return nil, notFound("store.diagnostics.get")
	}
	return &result[0], nil
}

func (r *diagnosticsRepository) UpdateExtractionDiagnosticStatus(
	ctx context.Context,
	tenant store.Tenant,
	id string,
	status string,
) (*store.ExtractionDiagnosticRow, error) {
	if err := store.ValidateDiagnosticUpdateStatus(status); err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `
			UPDATE extraction_diagnostics
			SET status = $2,
			    resolved_at = CASE WHEN $2 = 'open' THEN NULL ELSE NOW() END,
			    updated_at = NOW()
			WHERE id = $1 AND tenant_id = $3
			RETURNING `+diagnosticColumns,
		id, status, tenant.ID,
	)
	if err != nil {
		if isDiagnosticOpenConflict(err) {
			return nil, conflict("store.diagnostics.update_status", fmt.Errorf("open diagnostic already exists for reader/message/rule: %s", messageDiagnosticConflict))
		}
		return nil, fmt.Errorf("updating extraction diagnostic status: %w", err)
	}
	defer rows.Close()
	result, err := scanDiagnosticRows(rows)
	if err != nil {
		if isDiagnosticOpenConflict(err) {
			return nil, conflict("store.diagnostics.update_status", fmt.Errorf("open diagnostic already exists for reader/message/rule: %s", messageDiagnosticConflict))
		}
		return nil, fmt.Errorf("updating extraction diagnostic status: %w", err)
	}
	if len(result) == 0 {
		return nil, notFound("store.diagnostics.update_status")
	}
	return &result[0], nil
}
