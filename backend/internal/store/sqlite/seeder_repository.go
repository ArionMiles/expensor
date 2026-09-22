package sqlite

import (
	"context"
	"regexp"
	"sort"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func (s *Store) Seed(ctx context.Context, content store.SeedContent) (api.CategoryResolver, error) {
	if err := s.SeedPredefinedRules(ctx, buildSystemRuleRows(content.Rules)); err != nil {
		return nil, errors.B.Op("sqlite.seeder.seed").KindInternal().Text("seeding predefined rules").Err(err).Build()
	}
	if err := s.SeedMCCCodes(ctx, content.MCCEntries); err != nil {
		return nil, errors.B.Op("sqlite.seeder.seed").KindInternal().Text("seeding MCC codes").Err(err).Build()
	}
	if _, err := s.SeedMerchantCategories(ctx, content.MerchantCategories); err != nil {
		return nil, errors.B.Op("sqlite.seeder.seed").KindInternal().Text("seeding merchant categories").Err(err).Build()
	}
	if err := s.SeedMCCCategories(ctx, uniqueCategoryNames(content.MCCEntries)); err != nil {
		return nil, errors.B.Op("sqlite.seeder.seed").KindInternal().Text("seeding MCC category names").Err(err).Build()
	}
	resolver, err := s.LoadCategorySnapshot(ctx)
	if err != nil {
		return nil, errors.B.Op("sqlite.seeder.seed").KindInternal().Text("loading category snapshot").Err(err).Build()
	}
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
			Name: rule.Name, SenderEmail: sender, SenderEmails: rule.SenderEmails,
			SubjectContains: rule.SubjectContains, AmountRegex: regexString(rule.Amount),
			MerchantRegex: regexString(rule.MerchantInfo), CurrencyRegex: regexString(rule.Currency),
			TransactionSource: rule.Source.Display(), SourceType: rule.Source.Type,
			SourceLabel: rule.Source.Label, Bank: rule.Source.Bank,
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
