package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/ArionMiles/expensor/backend/internal/assistant"
	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type ruleDraftService interface {
	DraftRule(ctx context.Context, tenant store.Tenant, input assistant.RuleDraftInput) (assistant.RuleDraftResult, error)
}

type ruleDraftSampleJSON struct {
	Name     string                `json:"name" validate:"no_control_chars"`
	Sender   string                `json:"sender" validate:"no_control_chars"`
	Subject  string                `json:"subject" validate:"no_control_chars"`
	Body     string                `json:"body"`
	Expected ruleDraftExpectedJSON `json:"expected"`
}

type ruleDraftExpectedJSON struct {
	Amount   string `json:"amount" validate:"no_control_chars"`
	Merchant string `json:"merchant" validate:"no_control_chars"`
	Currency string `json:"currency" validate:"no_control_chars"`
}

type ruleDraftSourceJSON struct {
	Type  string `json:"type" validate:"no_control_chars"`
	Label string `json:"label" validate:"no_control_chars"`
	Bank  string `json:"bank" validate:"no_control_chars"`
}

type ruleDraftRequestJSON struct {
	Name            string                `json:"name" validate:"no_control_chars"`
	SenderEmails    []string              `json:"sender_emails" validate:"dive,email"`
	SubjectContains string                `json:"subject_contains" validate:"no_control_chars"`
	AmountRegex     string                `json:"amount_regex" validate:"omitempty,regexp"`
	MerchantRegex   string                `json:"merchant_regex" validate:"omitempty,regexp"`
	CurrencyRegex   string                `json:"currency_regex" validate:"omitempty,regexp"`
	Source          ruleDraftSourceJSON   `json:"source"`
	Samples         []ruleDraftSampleJSON `json:"samples" validate:"required,min=1,dive"`
}

type ruleDraftResponseJSON struct {
	Draft            ruleDraftRuleJSON    `json:"draft"`
	Matches          []ruleDraftMatchJSON `json:"matches"`
	ValidationIssues []ruleDraftIssueJSON `json:"validation_issues,omitempty"`
}

type ruleDraftRuleJSON struct {
	Name            string     `json:"name"`
	SenderEmails    []string   `json:"sender_emails"`
	SubjectContains string     `json:"subject_contains"`
	AmountRegex     string     `json:"amount_regex"`
	MerchantRegex   string     `json:"merchant_regex"`
	CurrencyRegex   string     `json:"currency_regex"`
	Source          api.Source `json:"source"`
	Notes           string     `json:"notes"`
}

type ruleDraftMatchJSON struct {
	SampleIndex int    `json:"sample_index"`
	SampleName  string `json:"sample_name"`
	Amount      string `json:"amount"`
	Merchant    string `json:"merchant"`
	Currency    string `json:"currency"`
}

type ruleDraftIssueJSON struct {
	SampleIndex int    `json:"sample_index"`
	SampleName  string `json:"sample_name"`
	Field       string `json:"field"`
	Expected    string `json:"expected,omitempty"`
	Actual      string `json:"actual,omitempty"`
	Message     string `json:"message"`
}

// CreateRuleDraft handles POST /api/rule-drafts.
//
// @Summary Draft a rule using the active LLM provider
// @Tags Rules
// @Accept json
// @Produce json
// @Param request body RuleDraftRequest true "Rule draft request"
// @Success 200 {object} RuleDraftResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Router /rule-drafts [post]
func (h *Handlers) CreateRuleDraft(w http.ResponseWriter, r *http.Request) {
	if h.ruleDrafts == nil {
		writeError(w, http.StatusServiceUnavailable, "rule drafting is not configured")
		return
	}
	body, ok := decodeAndValidateJSON[ruleDraftRequestJSON](h, w, r)
	if !ok {
		return
	}
	if !validateRuleDraftRequest(w, body) {
		return
	}
	result, err := h.ruleDrafts.DraftRule(r.Context(), requestTenant(r), ruleDraftInputFromJSON(body))
	if err != nil {
		h.writeRuleDraftError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ruleDraftResultToJSON(result))
}

func (h *Handlers) writeRuleDraftError(w http.ResponseWriter, err error) {
	switch errors.WhatKind(err) {
	case llm.KindNoProviderConfigured:
		writeError(w, http.StatusConflict, "Configure an LLM provider before drafting rules.")
	case llm.KindCapabilityUnsupported:
		writeError(w, http.StatusConflict, "The active LLM provider does not support structured rule drafting.")
	case assistant.KindRuleDraftInvalidInput:
		writeError(w, http.StatusUnprocessableEntity, safeRuleDraftErrorMessage(err, "rule draft input is invalid"))
	case assistant.KindRuleDraftInvalidOutput:
		writeError(w, http.StatusUnprocessableEntity, safeRuleDraftErrorMessage(err, "rule draft output is invalid"))
	case assistant.KindRuleDraftPromptMissing:
		writeError(w, http.StatusInternalServerError, "rule drafting prompt is not configured")
	default:
		writeLLMError(w, err)
	}
}

func safeRuleDraftErrorMessage(err error, fallback string) string {
	if msg := errors.UserMsg(err); msg != "" {
		return msg
	}
	return fallback
}

func validateRuleDraftRequest(w http.ResponseWriter, body ruleDraftRequestJSON) bool {
	hasBody := false
	hasExpected := false
	for _, sample := range body.Samples {
		if strings.TrimSpace(sample.Body) == "" {
			continue
		}
		hasBody = true
		if strings.TrimSpace(sample.Expected.Amount) != "" && strings.TrimSpace(sample.Expected.Merchant) != "" {
			hasExpected = true
			break
		}
	}
	if !hasBody {
		writeValidationErrors(w, []ValidationErrorDetail{{
			Field:    "samples",
			Location: "body",
			Message:  "must include at least one email body",
		}})
		return false
	}
	if !hasExpected {
		writeValidationErrors(w, []ValidationErrorDetail{{
			Field:    "samples.expected",
			Location: "body",
			Message:  "must include expected amount and merchant for at least one email body",
		}})
		return false
	}
	return true
}

func ruleDraftInputFromJSON(in ruleDraftRequestJSON) assistant.RuleDraftInput {
	samples := make([]assistant.Sample, 0, len(in.Samples))
	for _, sample := range in.Samples {
		samples = append(samples, assistant.Sample{
			Name:    sample.Name,
			Sender:  sample.Sender,
			Subject: sample.Subject,
			Body:    sample.Body,
			Expected: assistant.Expected{
				Amount:   sample.Expected.Amount,
				Merchant: sample.Expected.Merchant,
				Currency: sample.Expected.Currency,
			},
		})
	}
	return assistant.RuleDraftInput{
		Current: assistant.RuleDraft{
			Name:            in.Name,
			SenderEmails:    in.SenderEmails,
			SubjectContains: in.SubjectContains,
			AmountRegex:     in.AmountRegex,
			MerchantRegex:   in.MerchantRegex,
			CurrencyRegex:   in.CurrencyRegex,
			Source: api.Source{
				Type:  in.Source.Type,
				Label: in.Source.Label,
				Bank:  in.Source.Bank,
			},
		},
		Samples: samples,
	}
}

func ruleDraftResultToJSON(result assistant.RuleDraftResult) ruleDraftResponseJSON {
	matches := make([]ruleDraftMatchJSON, 0, len(result.Matches))
	for _, match := range result.Matches {
		matches = append(matches, ruleDraftMatchJSON{
			SampleIndex: match.SampleIndex,
			SampleName:  match.SampleName,
			Amount:      match.Amount,
			Merchant:    match.Merchant,
			Currency:    match.Currency,
		})
	}
	issues := make([]ruleDraftIssueJSON, 0, len(result.ValidationIssues))
	for _, issue := range result.ValidationIssues {
		issues = append(issues, ruleDraftIssueJSON{
			SampleIndex: issue.SampleIndex,
			SampleName:  issue.SampleName,
			Field:       issue.Field,
			Expected:    issue.Expected,
			Actual:      issue.Actual,
			Message:     issue.Message,
		})
	}
	return ruleDraftResponseJSON{
		Draft: ruleDraftRuleJSON{
			Name:            result.Draft.Name,
			SenderEmails:    result.Draft.SenderEmails,
			SubjectContains: result.Draft.SubjectContains,
			AmountRegex:     result.Draft.AmountRegex,
			MerchantRegex:   result.Draft.MerchantRegex,
			CurrencyRegex:   result.Draft.CurrencyRegex,
			Source:          result.Draft.Source,
			Notes:           result.Draft.Notes,
		},
		Matches:          matches,
		ValidationIssues: issues,
	}
}
