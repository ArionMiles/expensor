package postgres

import (
	"context"
	"log/slog"
	"regexp"
	"sort"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type seederRepository struct {
	rules     *rulesRepository
	community *communityRepository
	logger    *slog.Logger
}

func newSeederRepository(rules *rulesRepository, community *communityRepository, logger *slog.Logger) *seederRepository {
	return &seederRepository{rules: rules, community: community, logger: logger}
}

func (r *seederRepository) Seed(ctx context.Context, content store.SeedContent) (api.CategoryResolver, error) {
	systemRuleRows := buildSystemRuleRows(content.Rules)
	if err := r.rules.SeedPredefinedRules(ctx, systemRuleRows); err != nil {
		return nil, errors.E("postgres.seeder.seed", errors.Internal, "seeding predefined rules", err)
	}
	r.logger.Info("predefined rules seeded", "count", len(systemRuleRows))

	if err := r.community.SeedMCCCodes(ctx, content.MCCEntries); err != nil {
		return nil, errors.E("postgres.seeder.seed", errors.Internal, "seeding MCC codes", err)
	}
	if _, err := r.community.SeedMerchantCategories(ctx, content.MerchantCategories); err != nil {
		return nil, errors.E("postgres.seeder.seed", errors.Internal, "seeding merchant categories", err)
	}
	if err := r.community.SeedMCCCategories(ctx, uniqueCategoryNames(content.MCCEntries)); err != nil {
		return nil, errors.E("postgres.seeder.seed", errors.Internal, "seeding MCC category names", err)
	}

	resolver, err := r.community.LoadCategorySnapshot(ctx)
	if err != nil {
		return nil, errors.E("postgres.seeder.seed", errors.Internal, "loading category snapshot", err)
	}
	r.logger.Info("category resolver loaded")
	return resolver, nil
}

func buildSystemRuleRows(raw []api.Rule) []store.RuleRow {
	rows := make([]store.RuleRow, 0, len(raw))
	for _, rule := range raw {
		sender := rule.SenderEmail
		if sender == "" && len(rule.SenderEmails) > 0 {
			sender = rule.SenderEmails[0]
		}
		rows = append(rows, store.RuleRow{
			Name:              rule.Name,
			SenderEmail:       sender,
			SubjectContains:   rule.SubjectContains,
			AmountRegex:       regexString(rule.Amount),
			MerchantRegex:     regexString(rule.MerchantInfo),
			CurrencyRegex:     regexString(rule.Currency),
			TransactionSource: rule.Source.Display(),
			SenderEmails:      rule.SenderEmails,
			SourceType:        rule.Source.Type,
			SourceLabel:       rule.Source.Label,
			Bank:              rule.Source.Bank,
		})
	}
	return rows
}

func regexString(re *regexp.Regexp) string {
	if re == nil {
		return ""
	}
	return re.String()
}

func uniqueCategoryNames(entries []store.MCCEntry) []string {
	seen := make(map[string]struct{})
	for _, entry := range entries {
		seen[entry.Category] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
