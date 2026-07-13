package postgres

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

const diagnosticColumns = `
	id::text, status, reader, COALESCE(message_id, ''), source, sender, sender_email, subject, email_body,
	received_at, snippet, rule_id::text, rule_name, amount_regex, merchant_regex, currency_regex, failure_reasons,
	created_at, updated_at, resolved_at
`

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func diagnosticFailureReasons(reasons []string) []string {
	if reasons == nil {
		return []string{}
	}
	return reasons
}

func diagnosticSender(diagnostic api.ExtractionDiagnostic) string {
	if diagnostic.Sender != "" {
		return diagnostic.Sender
	}
	return diagnostic.SenderEmail
}

func scanDiagnosticRows(rows pgx.Rows) ([]store.ExtractionDiagnosticRow, error) {
	var result []store.ExtractionDiagnosticRow
	for rows.Next() {
		var row store.ExtractionDiagnosticRow
		var ruleID pgtype.Text
		var receivedAt pgtype.Timestamptz
		var resolvedAt pgtype.Timestamptz
		if err := rows.Scan(
			&row.ID,
			&row.Status,
			&row.Reader,
			&row.MessageID,
			&row.Source,
			&row.Sender,
			&row.SenderEmail,
			&row.Subject,
			&row.EmailBody,
			&receivedAt,
			&row.Snippet,
			&ruleID,
			&row.RuleName,
			&row.AmountRegex,
			&row.MerchantRegex,
			&row.CurrencyRegex,
			&row.FailureReasons,
			&row.CreatedAt,
			&row.UpdatedAt,
			&resolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning extraction diagnostic: %w", err)
		}
		if ruleID.Valid {
			row.RuleID = &ruleID.String
		}
		if receivedAt.Valid {
			received := receivedAt.Time
			row.ReceivedAt = &received
		}
		if resolvedAt.Valid {
			resolved := resolvedAt.Time
			row.ResolvedAt = &resolved
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []store.ExtractionDiagnosticRow{}
	}
	return result, nil
}

func scanTransactions(rows pgx.Rows) ([]store.Transaction, error) {
	var txns []store.Transaction
	for rows.Next() {
		var t store.Transaction
		var legacySource, sourceType, sourceLabel, bank string
		if err := rows.Scan(
			&t.ID, &t.MessageID, &t.Amount, &t.Currency,
			&t.OriginalAmount, &t.OriginalCurrency, &t.ExchangeRate,
			&t.Timestamp, &t.MerchantInfo, &t.Category, &t.Bucket,
			&legacySource, &sourceType, &sourceLabel, &bank,
			&t.Description, &t.Muted, &t.MutedByMerchant, &t.MuteReason, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning transaction row: %w", err)
		}
		if sourceLabel == "" {
			sourceLabel = legacySource
		}
		t.Source = api.Source{Type: sourceType, Label: sourceLabel, Bank: bank}
		t.Labels = []string{}
		txns = append(txns, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating transaction rows: %w", err)
	}
	return txns, nil
}

// --- Rules ---

const ruleColumns = `id, name, sender_email, sender_emails, subject_contains, amount_regex, merchant_regex,
	currency_regex, transaction_source, source_type, source_label, bank, predefined, created_at, updated_at`

func scanRuleRows(rows pgx.Rows) ([]store.RuleRow, error) {
	var result []store.RuleRow
	for rows.Next() {
		var r store.RuleRow
		if err := rows.Scan(
			&r.ID, &r.Name, &r.SenderEmail, &r.SenderEmails, &r.SubjectContains,
			&r.AmountRegex, &r.MerchantRegex, &r.CurrencyRegex,
			&r.TransactionSource, &r.SourceType, &r.SourceLabel, &r.Bank, &r.Predefined,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning rule row: %w", err)
		}
		result = append(result, r)
	}
	if result == nil {
		result = []store.RuleRow{}
	}
	return result, rows.Err()
}
