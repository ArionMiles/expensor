package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"

	"github.com/ArionMiles/expensor/backend/internal/rules"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

var (
	//go:embed content/rules.json
	rulesInput string
	//go:embed content/mcc.json
	mccInput []byte
	//go:embed content/categories.json
	categoriesInput []byte
	//go:embed content/banks.json
	banksInput []byte
	//go:embed content/readers/gmail/guide.json content/readers/thunderbird/guide.json
	readersFS embed.FS
)

// embeddedContent bundles parsed startup assets.
type embeddedContent struct {
	rawRules   []api.Rule
	rules      []api.Rule
	mccEntries []store.MCCEntry
	catEntries []store.MerchantCategoryEntry
}

// RuleJSON represents a rule in JSON format for parsing.
type RuleJSON struct {
	Name            string `json:"name"`
	SenderEmail     string `json:"senderEmail"`
	SubjectContains string `json:"subjectContains"`
	AmountRegex     string `json:"amountRegex"`
	MerchantRegex   string `json:"merchantInfoRegex"`
	CurrencyRegex   string `json:"currencyRegex,omitempty"`
	Source          string `json:"source"`
}

// buildSystemRuleRows converts embedded rules into rows ready for seeding.
func buildSystemRuleRows(raw []api.Rule) []store.RuleRow {
	rows := make([]store.RuleRow, 0, len(raw))
	for _, r := range raw {
		sender := r.SenderEmail
		if sender == "" && len(r.SenderEmails) > 0 {
			sender = r.SenderEmails[0]
		}
		rows = append(rows, store.RuleRow{
			Name: r.Name, SenderEmail: sender, SubjectContains: r.SubjectContains,
			AmountRegex: regexString(r.Amount), MerchantRegex: regexString(r.MerchantInfo),
			CurrencyRegex: regexString(r.Currency), TransactionSource: r.Source.Display(),
			SenderEmails: r.SenderEmails, SourceType: r.Source.Type, SourceLabel: r.Source.Label, Bank: r.Source.Bank,
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

// loadUserRules compiles tenant user-created rules, skipping invalid persisted regexes.
func loadUserRules(ctx context.Context, st daemonStore, tenant store.Tenant, logger *slog.Logger) []api.Rule {
	rows, err := st.ListRules(ctx, tenant)
	if err != nil {
		logger.Warn("failed to load rules from DB, falling back to embedded rules", "error", err)
		return nil
	}
	var out []api.Rule
	for _, row := range rows {
		if row.Predefined {
			continue
		}
		r, compileErr := compileRule(row)
		if compileErr != nil {
			logger.Warn("skipping rule with invalid regex", "rule", row.Name, "error", compileErr)
			continue
		}
		out = append(out, r)
	}
	return out
}

// compileRule converts persisted regex strings into an executable rule.
func compileRule(row store.RuleRow) (api.Rule, error) {
	amount, err := regexp.Compile(row.AmountRegex)
	if err != nil {
		return api.Rule{}, fmt.Errorf("amount_regex: %w", err)
	}
	merchant, err := regexp.Compile(row.MerchantRegex)
	if err != nil {
		return api.Rule{}, fmt.Errorf("merchant_regex: %w", err)
	}
	var currency *regexp.Regexp
	if row.CurrencyRegex != "" {
		currency, err = regexp.Compile(row.CurrencyRegex)
		if err != nil {
			return api.Rule{}, fmt.Errorf("currency_regex: %w", err)
		}
	}
	return api.Rule{
		ID: row.ID, Name: row.Name, SenderEmail: row.SenderEmail, SubjectContains: row.SubjectContains,
		Amount: amount, MerchantInfo: merchant, Currency: currency,
		SenderEmails: row.SenderEmails,
		Source:       api.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank},
	}, nil
}

// parseEmbedded parses all startup JSON assets.
func parseEmbedded(rulesJSON string, mccJSON, categoriesJSON []byte) (embeddedContent, error) {
	compiled, err := parseRules(rulesJSON)
	if err != nil {
		return embeddedContent{}, err
	}
	var mccEntries []store.MCCEntry
	if err := json.Unmarshal(mccJSON, &mccEntries); err != nil {
		return embeddedContent{}, fmt.Errorf("parsing mcc JSON: %w", err)
	}
	var catEntries []store.MerchantCategoryEntry
	if err := json.Unmarshal(categoriesJSON, &catEntries); err != nil {
		return embeddedContent{}, fmt.Errorf("parsing categories JSON: %w", err)
	}
	return embeddedContent{rawRules: compiled, rules: compiled, mccEntries: mccEntries, catEntries: catEntries}, nil
}

// uniqueCategoryNames returns sorted category names from MCC entries.
func uniqueCategoryNames(entries []store.MCCEntry) []string {
	seen := make(map[string]struct{})
	for _, e := range entries {
		seen[e.Category] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// parseRules parses the versioned embedded rule document.
func parseRules(rulesJSON string) ([]api.Rule, error) {
	doc, err := rules.ParseDocument([]byte(rulesJSON))
	if err != nil {
		return nil, fmt.Errorf("parsing rules JSON: %w", err)
	}
	return doc.Rules, nil
}
