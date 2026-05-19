package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

type DiagnosticsRepository interface {
	RecordExtractionDiagnostic(ctx context.Context, diagnostic api.ExtractionDiagnostic) error
	ListExtractionDiagnostics(ctx context.Context, f DiagnosticFilter) ([]ExtractionDiagnosticRow, error)
	GetExtractionDiagnostic(ctx context.Context, id string) (*ExtractionDiagnosticRow, error)
	UpdateExtractionDiagnosticStatus(ctx context.Context, id, status string) (*ExtractionDiagnosticRow, error)
}

type pgDiagnosticsRepository struct {
	pool    *pgxpool.Pool
	metrics *QueryInstrumentation
}

func NewDiagnosticsRepository(deps repositoryDependencies) DiagnosticsRepository {
	metrics := deps.metrics
	if metrics == nil {
		metrics = NewQueryInstrumentation(deps.logger)
	}
	return &pgDiagnosticsRepository{
		pool:    deps.pool,
		metrics: metrics,
	}
}

func (r *pgDiagnosticsRepository) RecordExtractionDiagnostic(ctx context.Context, diagnostic api.ExtractionDiagnostic) error {
	return r.metrics.Observe(ctx, "diagnostics.record_extraction", func(ctx context.Context) error {
		const q = `
			INSERT INTO extraction_diagnostics (
				reader, message_id, source, sender, sender_email, subject, email_body, received_at, snippet,
				rule_id, rule_name, amount_regex, merchant_regex, currency_regex, failure_reasons
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			ON CONFLICT (reader, message_id, rule_name) WHERE status = 'open' AND message_id IS NOT NULL
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

		_, err := r.pool.Exec(ctx, q,
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
	})
}

func (r *pgDiagnosticsRepository) ListExtractionDiagnostics(ctx context.Context, f DiagnosticFilter) ([]ExtractionDiagnosticRow, error) {
	var result []ExtractionDiagnosticRow
	err := r.metrics.Observe(ctx, "diagnostics.list_extraction", func(ctx context.Context) error {
		if err := ValidateDiagnosticFilterStatus(f.Status); err != nil {
			return err
		}

		query := `SELECT ` + diagnosticColumns + ` FROM extraction_diagnostics`
		args := []any{}
		if f.Status != DiagnosticStatusAll {
			query += ` WHERE status = $1`
			args = append(args, f.Status)
		}
		query += ` ORDER BY created_at DESC`
		if f.Limit > 0 {
			args = append(args, f.Limit)
			query += fmt.Sprintf(` LIMIT $%d`, len(args))
		}

		rows, err := r.pool.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("listing extraction diagnostics: %w", err)
		}
		defer rows.Close()
		result, err = scanDiagnosticRows(rows)
		if err != nil {
			return fmt.Errorf("listing extraction diagnostics: %w", err)
		}
		return nil
	})
	return result, err
}

func (r *pgDiagnosticsRepository) GetExtractionDiagnostic(ctx context.Context, id string) (*ExtractionDiagnosticRow, error) {
	var diagnostic *ExtractionDiagnosticRow
	err := r.metrics.Observe(ctx, "diagnostics.get_extraction", func(ctx context.Context) error {
		rows, err := r.pool.Query(ctx, `SELECT `+diagnosticColumns+` FROM extraction_diagnostics WHERE id = $1`, id)
		if err != nil {
			return fmt.Errorf("fetching extraction diagnostic: %w", err)
		}
		defer rows.Close()
		result, err := scanDiagnosticRows(rows)
		if err != nil {
			return fmt.Errorf("fetching extraction diagnostic: %w", err)
		}
		if len(result) == 0 {
			return ErrNotFound
		}
		diagnostic = &result[0]
		return nil
	})
	return diagnostic, err
}

func (r *pgDiagnosticsRepository) UpdateExtractionDiagnosticStatus(
	ctx context.Context,
	id string,
	status string,
) (*ExtractionDiagnosticRow, error) {
	var diagnostic *ExtractionDiagnosticRow
	err := r.metrics.Observe(ctx, "diagnostics.update_extraction_status", func(ctx context.Context) error {
		if err := validateDiagnosticRowStatus(status); err != nil {
			return err
		}

		rows, err := r.pool.Query(ctx, `
			UPDATE extraction_diagnostics
			SET status = $2,
			    resolved_at = CASE WHEN $2 = 'open' THEN NULL ELSE NOW() END,
			    updated_at = NOW()
			WHERE id = $1
			RETURNING `+diagnosticColumns,
			id, status,
		)
		if err != nil {
			if isDiagnosticOpenConflict(err) {
				return fmt.Errorf("open diagnostic already exists for reader/message/rule: %w", ErrDiagnosticConflict)
			}
			return fmt.Errorf("updating extraction diagnostic status: %w", err)
		}
		defer rows.Close()
		result, err := scanDiagnosticRows(rows)
		if err != nil {
			if isDiagnosticOpenConflict(err) {
				return fmt.Errorf("open diagnostic already exists for reader/message/rule: %w", ErrDiagnosticConflict)
			}
			return fmt.Errorf("updating extraction diagnostic status: %w", err)
		}
		if len(result) == 0 {
			return ErrNotFound
		}
		diagnostic = &result[0]
		return nil
	})
	return diagnostic, err
}
