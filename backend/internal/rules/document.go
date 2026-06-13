package rules

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

const currentDocumentVersion = 2

// Document is the versioned rules.json shape.
type Document struct {
	Version int
	Presets Presets
	Rules   []api.Rule
}

// Presets lists source taxonomy values shown by rule authoring UIs.
type Presets struct {
	SourceTypes []PresetValue `json:"source_types"`
	Banks       []PresetValue `json:"banks"`
}

// PresetValue identifies whether a taxonomy value is shipped or user-created.
type PresetValue struct {
	Value  string `json:"value"`
	Origin string `json:"origin"`
}

type rawDocument struct {
	Version int       `json:"version"`
	Presets Presets   `json:"presets"`
	Rules   []rawRule `json:"rules"`
}

type rawRule struct {
	Name              string          `json:"name"`
	SenderEmail       string          `json:"senderEmail"`
	SenderEmailSnake  string          `json:"sender_email"`
	SenderEmails      []string        `json:"sender_emails"`
	SubjectContains   string          `json:"subjectContains"`
	SubjectSnake      string          `json:"subject_contains"`
	AmountRegex       string          `json:"amountRegex"`
	AmountSnake       string          `json:"amount_regex"`
	MerchantRegex     string          `json:"merchantInfoRegex"`
	MerchantSnake     string          `json:"merchant_regex"`
	CurrencyRegex     string          `json:"currencyRegex"`
	CurrencySnake     string          `json:"currency_regex"`
	Source            json.RawMessage `json:"source"`
	SourceText        string          `json:"transaction_source"`
	LegacySource      string          `json:"-"`
	TransactionSource string          `json:"transactionSource"`
}

// ParseDocument parses versioned v2 rules and the legacy array format.
func ParseDocument(body []byte) (*Document, error) {
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "[") {
		var raw []rawRule
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("parsing legacy rules: %w", err)
		}
		rules, err := compileRules(raw)
		if err != nil {
			return nil, err
		}
		return &Document{Version: currentDocumentVersion, Rules: rules}, nil
	}

	var raw rawDocument
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing rule document: %w", err)
	}
	if raw.Version != currentDocumentVersion {
		return nil, fmt.Errorf("unsupported rule document version %d", raw.Version)
	}
	rules, err := compileRules(raw.Rules)
	if err != nil {
		return nil, err
	}
	return &Document{
		Version: raw.Version,
		Presets: raw.Presets,
		Rules:   rules,
	}, nil
}

func compileRules(rawRules []rawRule) ([]api.Rule, error) {
	out := make([]api.Rule, 0, len(rawRules))
	for _, raw := range rawRules {
		rule, err := compileRule(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, nil
}

func compileRule(raw rawRule) (api.Rule, error) {
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return api.Rule{}, fmt.Errorf("rule name is required")
	}
	senders := normalizeSenderList(raw.SenderEmails)
	for _, sender := range []string{raw.SenderEmail, raw.SenderEmailSnake} {
		if strings.TrimSpace(sender) != "" {
			senders = normalizeSenderList(append(senders, sender))
		}
	}
	if len(senders) == 0 {
		return api.Rule{}, fmt.Errorf("rule %q requires at least one sender email", name)
	}

	amountRegex := firstNonEmpty(raw.AmountRegex, raw.AmountSnake)
	merchantRegex := firstNonEmpty(raw.MerchantRegex, raw.MerchantSnake)
	amount, err := compileNamedRegex(name, "amount_regex", amountRegex)
	if err != nil {
		return api.Rule{}, err
	}
	merchant, err := compileNamedRegex(name, "merchant_regex", merchantRegex)
	if err != nil {
		return api.Rule{}, err
	}
	currencyPattern := firstNonEmpty(raw.CurrencyRegex, raw.CurrencySnake)
	currency, err := compileOptionalRegex(name, "currency_regex", currencyPattern)
	if err != nil {
		return api.Rule{}, err
	}

	source, err := parseSource(raw.Source)
	if err != nil {
		return api.Rule{}, fmt.Errorf("rule %q invalid source: %w", name, err)
	}
	if source == (api.Source{}) {
		source = splitLegacySource(firstNonEmpty(raw.LegacySource, raw.SourceText, raw.TransactionSource))
	}

	return api.Rule{
		Name:            name,
		SenderEmail:     senders[0],
		SenderEmails:    senders,
		SubjectContains: firstNonEmpty(raw.SubjectContains, raw.SubjectSnake),
		Amount:          amount,
		MerchantInfo:    merchant,
		Currency:        currency,
		Source:          source,
	}, nil
}

func parseSource(raw json.RawMessage) (api.Source, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return api.Source{}, nil
	}
	var source api.Source
	if err := json.Unmarshal(raw, &source); err == nil {
		return source, nil
	}
	var legacy string
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return api.Source{}, err
	}
	return splitLegacySource(legacy), nil
}

func compileNamedRegex(ruleName, field, pattern string) (*regexp.Regexp, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, fmt.Errorf("rule %q requires %s", ruleName, field)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("rule %q invalid %s: %w", ruleName, field, err)
	}
	return re, nil
}

func compileOptionalRegex(ruleName, field, pattern string) (*regexp.Regexp, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("rule %q invalid %s: %w", ruleName, field, err)
	}
	return re, nil
}

func normalizeSenderList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func splitLegacySource(source string) api.Source {
	source = strings.TrimSpace(source)
	if source == "" {
		return api.Source{}
	}
	parts := strings.Split(source, " - ")
	if len(parts) == 2 {
		return api.Source{Type: strings.TrimSpace(parts[0]), Label: source, Bank: strings.TrimSpace(parts[1])}
	}
	return api.Source{Label: source}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
