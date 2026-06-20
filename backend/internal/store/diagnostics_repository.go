package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

type pgDiagnosticsRepository struct {
	pool *pgxpool.Pool
}

func newPGDiagnosticsRepository(deps repositoryDependencies) *pgDiagnosticsRepository {
	return &pgDiagnosticsRepository{
		pool: deps.pool,
	}
}

func (r *pgDiagnosticsRepository) RecordExtractionDiagnostic(ctx context.Context, tenant Tenant, diagnostic api.ExtractionDiagnostic) error {
	q := diagnosticUpsertSQL(tenant)

	_, err := r.pool.Exec(ctx, q,
		tenantIDParam(tenant),
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

func diagnosticUpsertSQL(tenant Tenant) string {
	conflict := "ON CONFLICT (tenant_id, reader, message_id, rule_name) WHERE tenant_id IS NOT NULL AND status = 'open' AND message_id IS NOT NULL"
	if tenantIDParam(tenant) == nil {
		conflict = "ON CONFLICT (reader, message_id, rule_name) WHERE tenant_id IS NULL AND status = 'open' AND message_id IS NOT NULL"
	}
	return `
			INSERT INTO extraction_diagnostics (
				tenant_id, reader, message_id, source, sender, sender_email, subject, email_body, received_at, snippet,
				rule_id, rule_name, amount_regex, merchant_regex, currency_regex, failure_reasons
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			` + conflict + `
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
}

func (r *pgDiagnosticsRepository) ListExtractionDiagnostics(ctx context.Context, tenant Tenant, f DiagnosticFilter) ([]ExtractionDiagnosticRow, error) {
	if err := ValidateDiagnosticFilterStatus(f.Status); err != nil {
		return nil, err
	}

	query := `SELECT ` + diagnosticColumns + ` FROM extraction_diagnostics`
	args := []any{tenantIDParam(tenant)}
	query += ` WHERE tenant_id IS NOT DISTINCT FROM $1`
	if f.Status != DiagnosticStatusAll {
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

func (r *pgDiagnosticsRepository) GetExtractionDiagnostic(ctx context.Context, tenant Tenant, id string) (*ExtractionDiagnosticRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+diagnosticColumns+` FROM extraction_diagnostics WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
		id, tenantIDParam(tenant),
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
		return nil, ErrNotFound
	}
	return &result[0], nil
}

func (r *pgDiagnosticsRepository) UpdateExtractionDiagnosticStatus(
	ctx context.Context,
	tenant Tenant,
	id string,
	status string,
) (*ExtractionDiagnosticRow, error) {
	if err := validateDiagnosticRowStatus(status); err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `
			UPDATE extraction_diagnostics
			SET status = $2,
			    resolved_at = CASE WHEN $2 = 'open' THEN NULL ELSE NOW() END,
			    updated_at = NOW()
			WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $3
			RETURNING `+diagnosticColumns,
		id, status, tenantIDParam(tenant),
	)
	if err != nil {
		if isDiagnosticOpenConflict(err) {
			return nil, fmt.Errorf("open diagnostic already exists for reader/message/rule: %w", ErrDiagnosticConflict)
		}
		return nil, fmt.Errorf("updating extraction diagnostic status: %w", err)
	}
	defer rows.Close()
	result, err := scanDiagnosticRows(rows)
	if err != nil {
		if isDiagnosticOpenConflict(err) {
			return nil, fmt.Errorf("open diagnostic already exists for reader/message/rule: %w", ErrDiagnosticConflict)
		}
		return nil, fmt.Errorf("updating extraction diagnostic status: %w", err)
	}
	if len(result) == 0 {
		return nil, ErrNotFound
	}
	return &result[0], nil
}
