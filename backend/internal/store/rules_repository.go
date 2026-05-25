package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RulesRepository interface {
	InitRules(ctx context.Context) error
	ListRules(ctx context.Context) ([]RuleRow, error)
	GetRule(ctx context.Context, id string) (*RuleRow, error)
	CreateRule(ctx context.Context, r RuleRow) (*RuleRow, error)
	UpdateRule(ctx context.Context, id string, r RuleRow) (*RuleRow, error)
	DeleteRule(ctx context.Context, id string) error
	SeedPredefinedRules(ctx context.Context, rules []RuleRow) error
	ImportUserRules(ctx context.Context, rules []RuleRow) error
}

type pgRulesRepository struct {
	pool *pgxpool.Pool
}

func NewRulesRepository(deps repositoryDependencies) RulesRepository {
	return &pgRulesRepository{
		pool: deps.pool,
	}
}

func primarySender(rule RuleRow) string {
	if rule.SenderEmail != "" {
		return rule.SenderEmail
	}
	if len(rule.SenderEmails) > 0 {
		return rule.SenderEmails[0]
	}
	return ""
}

func normalizedRuleSenders(rule RuleRow) []string {
	if len(rule.SenderEmails) > 0 {
		return rule.SenderEmails
	}
	if rule.SenderEmail != "" {
		return []string{rule.SenderEmail}
	}
	return []string{}
}

func ruleSourceLabel(rule RuleRow) string {
	if rule.SourceLabel != "" {
		return rule.SourceLabel
	}
	return rule.TransactionSource
}

func (r *pgRulesRepository) InitRules(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
			-- Fresh-install path: create the table with the correct final schema.
			CREATE TABLE IF NOT EXISTS rules (
				id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name               TEXT NOT NULL,
				sender_email       TEXT NOT NULL DEFAULT '',
				subject_contains   TEXT NOT NULL DEFAULT '',
				amount_regex       TEXT NOT NULL,
				merchant_regex     TEXT NOT NULL,
				currency_regex     TEXT NOT NULL DEFAULT '',
				transaction_source TEXT NOT NULL DEFAULT '',
				predefined         BOOLEAN NOT NULL DEFAULT false,
				created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);

			-- Upgrade path: add columns that may be missing on older installs.
			ALTER TABLE rules ADD COLUMN IF NOT EXISTS transaction_source TEXT NOT NULL DEFAULT '';
			ALTER TABLE rules ADD COLUMN IF NOT EXISTS sender_emails TEXT[] NOT NULL DEFAULT '{}';
			ALTER TABLE rules ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '';
			ALTER TABLE rules ADD COLUMN IF NOT EXISTS source_label TEXT NOT NULL DEFAULT '';
			ALTER TABLE rules ADD COLUMN IF NOT EXISTS bank TEXT NOT NULL DEFAULT '';
			ALTER TABLE rules ADD COLUMN IF NOT EXISTS predefined BOOLEAN NOT NULL DEFAULT false;

			UPDATE rules
			SET sender_emails = ARRAY[sender_email]
			WHERE cardinality(sender_emails) = 0 AND sender_email <> '';

			UPDATE rules
			SET source_label = transaction_source
			WHERE source_label = '' AND transaction_source <> '';

			-- Ensure UNIQUE (name) constraint exists (idempotent).
			DO $$ BEGIN
				IF NOT EXISTS (
					SELECT 1 FROM information_schema.table_constraints
					WHERE table_name = 'rules' AND constraint_name = 'rules_name_key'
				) THEN
					ALTER TABLE rules ADD CONSTRAINT rules_name_key UNIQUE (name);
				END IF;
			END $$;
		`)
	if err != nil {
		return fmt.Errorf("initializing rules: executing rules initialization: %w", err)
	}
	return nil
}

func (r *pgRulesRepository) ListRules(ctx context.Context) ([]RuleRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+ruleColumns+`
			 FROM rules
			 ORDER BY predefined, name`)
	if err != nil {
		return nil, fmt.Errorf("listing rules: %w", err)
	}
	defer rows.Close()
	result, err := scanRuleRows(rows)
	if err != nil {
		return nil, fmt.Errorf("listing rules: %w", err)
	}
	return result, nil
}

func (r *pgRulesRepository) GetRule(ctx context.Context, id string) (*RuleRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+ruleColumns+` FROM rules WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("fetching rule: %w", err)
	}
	defer rows.Close()
	result, err := scanRuleRows(rows)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, ErrNotFound
	}
	return &result[0], nil
}

func (r *pgRulesRepository) CreateRule(ctx context.Context, rule RuleRow) (*RuleRow, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO rules (
				name, sender_email, sender_emails, subject_contains, amount_regex, merchant_regex,
				currency_regex, transaction_source, source_type, source_label, bank
			)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			 RETURNING `+ruleColumns,
		rule.Name, primarySender(rule), normalizedRuleSenders(rule), rule.SubjectContains,
		rule.AmountRegex, rule.MerchantRegex, rule.CurrencyRegex,
		ruleSourceLabel(rule), rule.SourceType, ruleSourceLabel(rule), rule.Bank,
	)
	if err != nil {
		if isRuleNameConflict(err) {
			return nil, ErrRuleNameConflict
		}
		return nil, fmt.Errorf("creating rule: %w", err)
	}
	defer rows.Close()
	result, err := scanRuleRows(rows)
	if err != nil {
		if isRuleNameConflict(err) {
			return nil, ErrRuleNameConflict
		}
		return nil, fmt.Errorf("creating rule: %w", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("creating rule: no row returned")
	}
	return &result[0], nil
}

func (r *pgRulesRepository) UpdateRule(ctx context.Context, id string, rule RuleRow) (*RuleRow, error) {
	rows, err := r.pool.Query(ctx,
		`UPDATE rules
			 SET name=$2, sender_email=$3, sender_emails=$4, subject_contains=$5,
			     amount_regex=$6, merchant_regex=$7, currency_regex=$8,
			     transaction_source=$9, source_type=$10, source_label=$11, bank=$12, updated_at=NOW()
			 WHERE id=$1
			 RETURNING `+ruleColumns,
		id, rule.Name, primarySender(rule), normalizedRuleSenders(rule), rule.SubjectContains,
		rule.AmountRegex, rule.MerchantRegex, rule.CurrencyRegex,
		ruleSourceLabel(rule), rule.SourceType, ruleSourceLabel(rule), rule.Bank,
	)
	if err != nil {
		if isRuleNameConflict(err) {
			return nil, ErrRuleNameConflict
		}
		return nil, fmt.Errorf("updating rule: %w", err)
	}
	defer rows.Close()
	result, err := scanRuleRows(rows)
	if err != nil {
		if isRuleNameConflict(err) {
			return nil, ErrRuleNameConflict
		}
		return nil, fmt.Errorf("updating rule: %w", err)
	}
	if len(result) == 0 {
		return nil, ErrNotFound
	}
	return &result[0], nil
}

func (r *pgRulesRepository) DeleteRule(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM rules WHERE id=$1 AND predefined = false`, id)
	if err != nil {
		return fmt.Errorf("deleting rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRulesRepository) SeedPredefinedRules(ctx context.Context, rules []RuleRow) error {
	for _, rule := range rules {
		_, err := r.pool.Exec(ctx, `
				INSERT INTO rules
				  (name, sender_email, sender_emails, subject_contains, amount_regex, merchant_regex,
				   currency_regex, transaction_source, source_type, source_label, bank, predefined)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, true)
				ON CONFLICT (name) DO NOTHING`,
			rule.Name, primarySender(rule), normalizedRuleSenders(rule), rule.SubjectContains,
			rule.AmountRegex, rule.MerchantRegex, rule.CurrencyRegex,
			ruleSourceLabel(rule), rule.SourceType, ruleSourceLabel(rule), rule.Bank,
		)
		if err != nil {
			return fmt.Errorf("seeding predefined rule %q: %w", rule.Name, err)
		}
	}
	return nil
}

func (r *pgRulesRepository) ImportUserRules(ctx context.Context, rules []RuleRow) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning import transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, rule := range rules {
		_, err := tx.Exec(ctx, `
				INSERT INTO rules
				  (name, sender_email, sender_emails, subject_contains, amount_regex, merchant_regex,
				   currency_regex, transaction_source, source_type, source_label, bank)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
				ON CONFLICT (name) DO UPDATE SET
					sender_email       = EXCLUDED.sender_email,
					sender_emails      = EXCLUDED.sender_emails,
					subject_contains   = EXCLUDED.subject_contains,
					amount_regex       = EXCLUDED.amount_regex,
					merchant_regex     = EXCLUDED.merchant_regex,
					currency_regex     = EXCLUDED.currency_regex,
					transaction_source = EXCLUDED.transaction_source,
					source_type        = EXCLUDED.source_type,
					source_label       = EXCLUDED.source_label,
					bank               = EXCLUDED.bank,
					updated_at         = NOW()`,
			rule.Name, primarySender(rule), normalizedRuleSenders(rule), rule.SubjectContains,
			rule.AmountRegex, rule.MerchantRegex, rule.CurrencyRegex,
			ruleSourceLabel(rule), rule.SourceType, ruleSourceLabel(rule), rule.Bank,
		)
		if err != nil {
			return fmt.Errorf("importing rule %q: %w", rule.Name, err)
		}
	}
	return tx.Commit(ctx)
}
