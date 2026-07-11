package rules

import (
	"context"
	"log/slog"
	"regexp"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// PersistedStore provides the tenant rule rows needed by LoadPersisted.
type PersistedStore interface {
	ListRules(ctx context.Context, tenant store.Tenant) ([]store.RuleRow, error)
}

// LoadPersisted compiles tenant-created rules, logging and skipping invalid rows.
func LoadPersisted(ctx context.Context, st PersistedStore, tenant store.Tenant, logger *slog.Logger) []api.Rule {
	rows, err := st.ListRules(ctx, tenant)
	if err != nil {
		logger.Warn("failed to load rules from DB, falling back to embedded rules", "error", err)
		return nil
	}
	out := make([]api.Rule, 0, len(rows))
	for _, row := range rows {
		if row.Predefined {
			continue
		}
		rule, compileErr := compilePersisted(row)
		if compileErr != nil {
			logger.Warn("skipping rule with invalid regex", "rule", row.Name, "error", compileErr)
			continue
		}
		out = append(out, rule)
	}
	return out
}

func compilePersisted(row store.RuleRow) (api.Rule, error) {
	amount, err := regexp.Compile(row.AmountRegex)
	if err != nil {
		return api.Rule{}, errors.E("rules.compile_persisted", errors.InvalidInput, "amount_regex", err)
	}
	merchant, err := regexp.Compile(row.MerchantRegex)
	if err != nil {
		return api.Rule{}, errors.E("rules.compile_persisted", errors.InvalidInput, "merchant_regex", err)
	}
	var currency *regexp.Regexp
	if row.CurrencyRegex != "" {
		currency, err = regexp.Compile(row.CurrencyRegex)
		if err != nil {
			return api.Rule{}, errors.E("rules.compile_persisted", errors.InvalidInput, "currency_regex", err)
		}
	}
	return api.Rule{
		ID: row.ID, Name: row.Name, SenderEmail: row.SenderEmail, SubjectContains: row.SubjectContains,
		Amount: amount, MerchantInfo: merchant, Currency: currency,
		SenderEmails: row.SenderEmails,
		Source:       api.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank},
	}, nil
}
