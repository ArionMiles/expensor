package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/internal/rules"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	apperrors "github.com/ArionMiles/expensor/backend/pkg/errors"
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
	//go:embed content/llm/prompts content/llm/providers
	llmContentFS embed.FS
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
		return api.Rule{}, apperrors.E("content.compile_rule", apperrors.InvalidInput, "amount_regex", err)
	}
	merchant, err := regexp.Compile(row.MerchantRegex)
	if err != nil {
		return api.Rule{}, apperrors.E("content.compile_rule", apperrors.InvalidInput, "merchant_regex", err)
	}
	var currency *regexp.Regexp
	if row.CurrencyRegex != "" {
		currency, err = regexp.Compile(row.CurrencyRegex)
		if err != nil {
			return api.Rule{}, apperrors.E("content.compile_rule", apperrors.InvalidInput, "currency_regex", err)
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
		return embeddedContent{}, apperrors.E("content.parse_embedded", apperrors.Internal, "parsing mcc JSON", err)
	}
	var catEntries []store.MerchantCategoryEntry
	if err := json.Unmarshal(categoriesJSON, &catEntries); err != nil {
		return embeddedContent{}, apperrors.E("content.parse_embedded", apperrors.Internal, "parsing categories JSON", err)
	}
	return embeddedContent{rawRules: compiled, rules: compiled, mccEntries: mccEntries, catEntries: catEntries}, nil
}

func loadLLMModelOptions(fsys embed.FS, name string) ([]llm.ModelOption, error) {
	body, err := fsys.ReadFile(fmt.Sprintf("content/llm/providers/%s_models.json", name))
	if err != nil {
		return nil, apperrors.E("content.llm_model_options", apperrors.Internal, fmt.Sprintf("reading %s llm model options", name), err)
	}
	var options []llm.ModelOption
	if err := json.Unmarshal(body, &options); err != nil {
		return nil, apperrors.E("content.llm_model_options", apperrors.Internal, fmt.Sprintf("parsing %s llm model options", name), err)
	}
	return options, nil
}

// parseRules parses the versioned embedded rule document.
func parseRules(rulesJSON string) ([]api.Rule, error) {
	doc, err := rules.ParseDocument([]byte(rulesJSON))
	if err != nil {
		return nil, apperrors.E("content.parse_rules", apperrors.Internal, "parsing rules JSON", err)
	}
	return doc.Rules, nil
}
