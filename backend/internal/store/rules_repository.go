package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgRulesRepository struct {
	pool *pgxpool.Pool
}

func newPGRulesRepository(deps repositoryDependencies) *pgRulesRepository {
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

func (r *pgRulesRepository) ListRules(ctx context.Context, tenant Tenant) ([]RuleRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+ruleColumns+`
			 FROM rules
			 WHERE predefined = true OR tenant_id IS NOT DISTINCT FROM $1
			 ORDER BY predefined, name`,
		tenantIDParam(tenant))
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

func (r *pgRulesRepository) GetRule(ctx context.Context, tenant Tenant, id string) (*RuleRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+ruleColumns+` FROM rules WHERE id = $1 AND (predefined = true OR tenant_id IS NOT DISTINCT FROM $2)`,
		id, tenantIDParam(tenant))
	if err != nil {
		return nil, fmt.Errorf("fetching rule: %w", err)
	}
	defer rows.Close()
	result, err := scanRuleRows(rows)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, notFound("store.rules.get")
	}
	return &result[0], nil
}

func (r *pgRulesRepository) CreateRule(ctx context.Context, tenant Tenant, rule RuleRow) (*RuleRow, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO rules (
				tenant_id, name, sender_email, sender_emails, subject_contains, amount_regex, merchant_regex,
				currency_regex, transaction_source, source_type, source_label, bank
			)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			 RETURNING `+ruleColumns,
		tenantIDParam(tenant), rule.Name, primarySender(rule), normalizedRuleSenders(rule), rule.SubjectContains,
		rule.AmountRegex, rule.MerchantRegex, rule.CurrencyRegex,
		ruleSourceLabel(rule), rule.SourceType, ruleSourceLabel(rule), rule.Bank,
	)
	if err != nil {
		if isRuleNameConflict(err) {
			return nil, conflict("store.rules.create", messageRuleNameConflict)
		}
		return nil, fmt.Errorf("creating rule: %w", err)
	}
	defer rows.Close()
	result, err := scanRuleRows(rows)
	if err != nil {
		if isRuleNameConflict(err) {
			return nil, conflict("store.rules.create", messageRuleNameConflict)
		}
		return nil, fmt.Errorf("creating rule: %w", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("creating rule: no row returned")
	}
	return &result[0], nil
}

func (r *pgRulesRepository) UpdateRule(ctx context.Context, tenant Tenant, id string, rule RuleRow) (*RuleRow, error) {
	rows, err := r.pool.Query(ctx,
		`UPDATE rules
			 SET name=$2, sender_email=$3, sender_emails=$4, subject_contains=$5,
			     amount_regex=$6, merchant_regex=$7, currency_regex=$8,
			     transaction_source=$9, source_type=$10, source_label=$11, bank=$12, updated_at=NOW()
			 WHERE id=$1 AND predefined = false AND tenant_id IS NOT DISTINCT FROM $13
			 RETURNING `+ruleColumns,
		id, rule.Name, primarySender(rule), normalizedRuleSenders(rule), rule.SubjectContains,
		rule.AmountRegex, rule.MerchantRegex, rule.CurrencyRegex,
		ruleSourceLabel(rule), rule.SourceType, ruleSourceLabel(rule), rule.Bank, tenantIDParam(tenant),
	)
	if err != nil {
		if isRuleNameConflict(err) {
			return nil, conflict("store.rules.update", messageRuleNameConflict)
		}
		return nil, fmt.Errorf("updating rule: %w", err)
	}
	defer rows.Close()
	result, err := scanRuleRows(rows)
	if err != nil {
		if isRuleNameConflict(err) {
			return nil, conflict("store.rules.update", messageRuleNameConflict)
		}
		return nil, fmt.Errorf("updating rule: %w", err)
	}
	if len(result) == 0 {
		return nil, notFound("store.rules.update")
	}
	return &result[0], nil
}

func (r *pgRulesRepository) DeleteRule(ctx context.Context, tenant Tenant, id string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM rules WHERE id=$1 AND predefined = false AND tenant_id IS NOT DISTINCT FROM $2`,
		id, tenantIDParam(tenant))
	if err != nil {
		return fmt.Errorf("deleting rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.rules.delete")
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
				ON CONFLICT (name) WHERE tenant_id IS NULL AND predefined = true DO NOTHING`,
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

func (r *pgRulesRepository) ImportUserRules(ctx context.Context, tenant Tenant, rules []RuleRow) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning import transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	conflictClause := importUserRulesConflictClause(tenant)
	for _, rule := range rules {
		_, err := tx.Exec(ctx, `
				INSERT INTO rules
				  (tenant_id, name, sender_email, sender_emails, subject_contains, amount_regex, merchant_regex,
				   currency_regex, transaction_source, source_type, source_label, bank)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				`+conflictClause+` DO UPDATE SET
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
			tenantIDParam(tenant), rule.Name, primarySender(rule), normalizedRuleSenders(rule), rule.SubjectContains,
			rule.AmountRegex, rule.MerchantRegex, rule.CurrencyRegex,
			ruleSourceLabel(rule), rule.SourceType, ruleSourceLabel(rule), rule.Bank,
		)
		if err != nil {
			return fmt.Errorf("importing rule %q: %w", rule.Name, err)
		}
	}
	return tx.Commit(ctx)
}

func importUserRulesConflictClause(tenant Tenant) string {
	if tenantIDParam(tenant) == nil {
		return "ON CONFLICT (name) WHERE tenant_id IS NULL AND predefined = false"
	}
	return "ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL AND predefined = false"
}
