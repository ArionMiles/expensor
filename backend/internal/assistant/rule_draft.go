package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

const (
	ruleDraftWorkflow       = "rule_drafting"
	ruleDraftPurpose        = "draft_rule"
	maxRuleDraftSamples     = 5
	maxRuleDraftSampleBytes = 12_000
)

var (
	ErrRuleDraftPromptMissing  = errors.New("rule draft prompt is not configured")
	ErrRuleDraftInvalidInput   = errors.New("rule draft input is invalid")
	ErrRuleDraftInvalidOutput  = errors.New("rule draft output is invalid")
	ErrRuleDraftValidationFail = errors.New("rule draft validation failed")
)

type RuleDraftService struct {
	router *llm.Router
}

type RuleDraftInput struct {
	Current RuleDraft `json:"current_rule"`
	Samples []Sample  `json:"samples"`
}

type RuleDraft struct {
	Name            string     `json:"name"`
	SenderEmails    []string   `json:"sender_emails"`
	SubjectContains string     `json:"subject_contains"`
	AmountRegex     string     `json:"amount_regex"`
	MerchantRegex   string     `json:"merchant_regex"`
	CurrencyRegex   string     `json:"currency_regex"`
	Source          api.Source `json:"source"`
	Notes           string     `json:"notes"`
}

type Sample struct {
	SampleIndex int      `json:"sample_index"`
	Name        string   `json:"name"`
	Sender      string   `json:"sender"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
	Expected    Expected `json:"expected"`
}

type Expected struct {
	Amount   string `json:"amount"`
	Merchant string `json:"merchant"`
	Currency string `json:"currency"`
}

type RuleDraftResult struct {
	Draft            RuleDraft              `json:"draft"`
	Matches          []SampleMatch          `json:"matches"`
	ValidationIssues []RuleDraftSampleIssue `json:"validation_issues,omitempty"`
}

type SampleMatch struct {
	SampleIndex int    `json:"sample_index"`
	SampleName  string `json:"sample_name"`
	Amount      string `json:"amount"`
	Merchant    string `json:"merchant"`
	Currency    string `json:"currency"`
}

type RuleDraftSampleIssue struct {
	SampleIndex int    `json:"sample_index"`
	SampleName  string `json:"sample_name"`
	Field       string `json:"field"`
	Expected    string `json:"expected,omitempty"`
	Actual      string `json:"actual,omitempty"`
	Message     string `json:"message"`
}

func NewRuleDraftService(router *llm.Router) *RuleDraftService {
	return &RuleDraftService{router: router}
}

func (s *RuleDraftService) DraftRule(ctx context.Context, tenant store.Tenant, input RuleDraftInput) (RuleDraftResult, error) {
	if s == nil || s.router == nil {
		return RuleDraftResult{}, llm.ErrNoProviderConfigured
	}
	normalized, err := normalizeRuleDraftInput(input)
	if err != nil {
		return RuleDraftResult{}, err
	}
	prompt, ok := s.router.PromptCatalog().Get(ruleDraftWorkflow, ruleDraftPurpose)
	if !ok {
		return RuleDraftResult{}, ErrRuleDraftPromptMissing
	}

	draft, raw, err := s.requestDraft(ctx, tenant, prompt, normalized, "")
	if err != nil {
		return RuleDraftResult{}, err
	}
	matches, issues := validateRuleDraft(normalized.Samples, draft)
	if len(issues) == 0 {
		return RuleDraftResult{Draft: draft, Matches: matches}, nil
	}

	repairNote := fmt.Sprintf("The previous draft failed validation: %s. Previous draft JSON: %s", formatRuleDraftIssues(issues), raw)
	draft, _, repairErr := s.requestDraft(ctx, tenant, prompt, normalized, repairNote)
	if repairErr != nil {
		return RuleDraftResult{}, repairErr
	}
	matches, issues = validateRuleDraft(normalized.Samples, draft)
	if len(issues) > 0 {
		return RuleDraftResult{Draft: draft, Matches: matches, ValidationIssues: issues}, nil
	}
	return RuleDraftResult{Draft: draft, Matches: matches}, nil
}

func (s *RuleDraftService) requestDraft(
	ctx context.Context,
	tenant store.Tenant,
	prompt llm.PromptDefinition,
	input RuleDraftInput,
	repairNote string,
) (RuleDraft, string, error) {
	contextJSON, err := ruleDraftContextJSON(input, repairNote)
	if err != nil {
		return RuleDraft{}, "", err
	}
	messages := renderPromptMessages(prompt.Messages, map[string]string{
		"rule_context_json": contextJSON,
	})
	response, err := s.router.Complete(ctx, tenant, llm.Request{
		Workflow:             prompt.Workflow,
		Purpose:              prompt.Purpose,
		Messages:             messages,
		RequiredCapabilities: append([]llm.Capability(nil), prompt.RequiredCapabilities...),
		MaxOutputTokens:      1600,
		ResponseFormat: llm.ResponseFormat{
			Type:   llm.ResponseFormatJSONSchema,
			Name:   "expensor_rule_draft",
			Strict: true,
			Schema: ruleDraftSchema(),
		},
	})
	if err != nil {
		return RuleDraft{}, "", err
	}
	var draft RuleDraft
	if err := json.Unmarshal([]byte(response.Text), &draft); err != nil {
		return RuleDraft{}, response.Text, fmt.Errorf("%w: %w", ErrRuleDraftInvalidOutput, err)
	}
	draft.normalize()
	return draft, response.Text, nil
}

func normalizeRuleDraftInput(input RuleDraftInput) (RuleDraftInput, error) {
	input.Current.normalize()
	samples := make([]Sample, 0, len(input.Samples))
	for _, sample := range input.Samples {
		sample.Name = strings.TrimSpace(sample.Name)
		sample.Sender = strings.TrimSpace(sample.Sender)
		sample.Subject = strings.TrimSpace(sample.Subject)
		sample.Body = strings.TrimSpace(sample.Body)
		sample.Expected.Amount = strings.TrimSpace(sample.Expected.Amount)
		sample.Expected.Merchant = strings.TrimSpace(sample.Expected.Merchant)
		sample.Expected.Currency = strings.TrimSpace(sample.Expected.Currency)
		if sample.Body == "" {
			continue
		}
		if len(sample.Body) > maxRuleDraftSampleBytes {
			sample.Body = sample.Body[:maxRuleDraftSampleBytes]
		}
		sample.SampleIndex = len(samples)
		samples = append(samples, sample)
		if len(samples) == maxRuleDraftSamples {
			break
		}
	}
	if len(samples) == 0 {
		return RuleDraftInput{}, fmt.Errorf("%w: add at least one email sample", ErrRuleDraftInvalidInput)
	}
	hasExpected := false
	for _, sample := range samples {
		if sample.Expected.Amount != "" && sample.Expected.Merchant != "" {
			hasExpected = true
			break
		}
	}
	if !hasExpected {
		return RuleDraftInput{}, fmt.Errorf("%w: add expected amount and merchant to at least one sample", ErrRuleDraftInvalidInput)
	}
	input.Samples = samples
	return input, nil
}

func (d *RuleDraft) normalize() {
	d.Name = strings.TrimSpace(d.Name)
	d.SenderEmails = normalizedStrings(d.SenderEmails)
	d.SubjectContains = strings.TrimSpace(d.SubjectContains)
	d.AmountRegex = strings.TrimSpace(d.AmountRegex)
	d.MerchantRegex = strings.TrimSpace(d.MerchantRegex)
	d.CurrencyRegex = strings.TrimSpace(d.CurrencyRegex)
	d.Source.Type = strings.TrimSpace(d.Source.Type)
	d.Source.Label = strings.TrimSpace(d.Source.Label)
	d.Source.Bank = strings.TrimSpace(d.Source.Bank)
	if d.Source.Label == "" {
		d.Source.Label = d.Source.Display()
	}
	d.Notes = strings.TrimSpace(d.Notes)
}

func normalizedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
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

func ruleDraftContextJSON(input RuleDraftInput, repairNote string) (string, error) {
	payload := struct {
		Current    RuleDraft `json:"current_rule"`
		Samples    []Sample  `json:"samples"`
		RepairNote string    `json:"repair_note,omitempty"`
	}{
		Current:    input.Current,
		Samples:    input.Samples,
		RepairNote: repairNote,
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding rule draft prompt context: %w", err)
	}
	return string(body), nil
}

func renderPromptMessages(messages []llm.Message, variables map[string]string) []llm.Message {
	out := make([]llm.Message, 0, len(messages))
	for _, message := range messages {
		content := message.Content
		for key, value := range variables {
			content = strings.ReplaceAll(content, "{{"+key+"}}", value)
		}
		message.Content = content
		out = append(out, message)
	}
	return out
}

func validateRuleDraft(samples []Sample, draft RuleDraft) ([]SampleMatch, []RuleDraftSampleIssue) {
	if draft.AmountRegex == "" || draft.MerchantRegex == "" {
		return nil, []RuleDraftSampleIssue{{
			SampleIndex: -1,
			Field:       "rule",
			Message:     "Draft did not include amount and merchant regexes.",
		}}
	}
	if len(draft.SenderEmails) == 0 {
		return nil, []RuleDraftSampleIssue{{
			SampleIndex: -1,
			Field:       "sender_emails",
			Message:     "Draft did not include sender emails.",
		}}
	}
	amount, err := regexp.Compile(draft.AmountRegex)
	if err != nil {
		return nil, []RuleDraftSampleIssue{{
			SampleIndex: -1,
			Field:       "amount",
			Message:     fmt.Sprintf("Amount regex does not compile: %v.", err),
		}}
	}
	merchant, err := regexp.Compile(draft.MerchantRegex)
	if err != nil {
		return nil, []RuleDraftSampleIssue{{
			SampleIndex: -1,
			Field:       "merchant",
			Message:     fmt.Sprintf("Merchant regex does not compile: %v.", err),
		}}
	}
	var currency *regexp.Regexp
	if draft.CurrencyRegex != "" {
		currency, err = regexp.Compile(draft.CurrencyRegex)
		if err != nil {
			return nil, []RuleDraftSampleIssue{{
				SampleIndex: -1,
				Field:       "currency",
				Message:     fmt.Sprintf("Currency regex does not compile: %v.", err),
			}}
		}
	}

	matches := make([]SampleMatch, 0, len(samples))
	issues := make([]RuleDraftSampleIssue, 0)
	for _, sample := range samples {
		match, sampleIssues := validateRuleDraftSample(sample, amount, merchant, currency)
		matches = append(matches, match)
		issues = append(issues, sampleIssues...)
	}
	return matches, issues
}

func validateRuleDraftSample(
	sample Sample,
	amount *regexp.Regexp,
	merchant *regexp.Regexp,
	currency *regexp.Regexp,
) (SampleMatch, []RuleDraftSampleIssue) {
	match := SampleMatch{
		SampleIndex: sample.SampleIndex,
		SampleName:  sample.Name,
		Amount:      firstSubmatch(amount, sample.Body),
		Merchant:    firstSubmatch(merchant, sample.Body),
	}
	if currency != nil {
		match.Currency = firstSubmatch(currency, sample.Body)
	}
	name := sampleName(sample)
	issues := make([]RuleDraftSampleIssue, 0, 3)
	if expected := sample.Expected.Amount; expected != "" && match.Amount != expected {
		issues = append(issues, RuleDraftSampleIssue{
			SampleIndex: sample.SampleIndex,
			SampleName:  name,
			Field:       "amount",
			Expected:    expected,
			Actual:      match.Amount,
			Message:     formatDraftMismatch("Amount", match.Amount, expected),
		})
	}
	if expected := sample.Expected.Merchant; expected != "" && match.Merchant != expected {
		issues = append(issues, RuleDraftSampleIssue{
			SampleIndex: sample.SampleIndex,
			SampleName:  name,
			Field:       "merchant",
			Expected:    expected,
			Actual:      match.Merchant,
			Message:     formatDraftMismatch("Merchant", match.Merchant, expected),
		})
	}
	issues = append(issues, validateCurrencyMatch(sample, match.Currency, currency != nil, name)...)
	return match, issues
}

func validateCurrencyMatch(sample Sample, got string, hasCurrencyRegex bool, sampleName string) []RuleDraftSampleIssue {
	expected := sample.Expected.Currency
	if expected == "" {
		return nil
	}
	if !hasCurrencyRegex {
		return []RuleDraftSampleIssue{{
			SampleIndex: sample.SampleIndex,
			SampleName:  sampleName,
			Field:       "currency",
			Expected:    expected,
			Message:     "Currency regex is required for this sample.",
		}}
	}
	if got != expected {
		return []RuleDraftSampleIssue{{
			SampleIndex: sample.SampleIndex,
			SampleName:  sampleName,
			Field:       "currency",
			Expected:    expected,
			Actual:      got,
			Message:     formatDraftMismatch("Currency", got, expected),
		}}
	}
	return nil
}

func formatDraftMismatch(field, got, expected string) string {
	if got == "" {
		return fmt.Sprintf("%s was missing; expected %q.", field, expected)
	}
	return fmt.Sprintf("%s matched %q, expected %q.", field, got, expected)
}

func formatRuleDraftIssues(issues []RuleDraftSampleIssue) string {
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		name := issue.SampleName
		if name == "" {
			name = "rule"
		}
		parts = append(parts, fmt.Sprintf("%s %s: %s", name, issue.Field, issue.Message))
	}
	return strings.Join(parts, " ")
}

func firstSubmatch(re *regexp.Regexp, body string) string {
	match := re.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func sampleName(sample Sample) string {
	if sample.Name != "" {
		return sample.Name
	}
	if sample.Subject != "" {
		return sample.Subject
	}
	return "sample"
}

func ruleDraftSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"required":["name","sender_emails","subject_contains","amount_regex","merchant_regex","currency_regex","source","notes"],
		"properties":{
			"name":{"type":"string","description":"Concise rule name."},
			"sender_emails":{"type":"array","items":{"type":"string"},"description":"Exact sender email addresses this rule should match."},
			"subject_contains":{"type":"string","description":"A concise subject substring shared by the samples, or empty string if none is safe."},
			"amount_regex":{"type":"string","description":"Go RE2 regex. Capture the numeric amount in group 1."},
			"merchant_regex":{"type":"string","description":"Go RE2 regex. Capture merchant text in group 1."},
			"currency_regex":{"type":"string","description":"Go RE2 regex capturing currency in group 1, or empty string."},
			"source":{
				"type":"object",
				"additionalProperties":false,
				"required":["type","label","bank"],
				"properties":{
					"type":{"type":"string"},
					"label":{"type":"string"},
					"bank":{"type":"string"}
				}
			},
			"notes":{"type":"string","description":"Short implementation note or empty string."}
		}
	}`)
}
