package assistant

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type ruleDraftRuntimeStore struct {
	runtime store.LLMProviderRuntime
	found   bool
	err     error
}

func (s *ruleDraftRuntimeStore) GetActiveLLMProviderRuntime(
	context.Context,
	store.Tenant,
) (store.LLMProviderRuntime, bool, error) {
	return s.runtime, s.found, s.err
}

type queuedRuleDraftClient struct {
	responses []string
	errs      []error
	requests  []llm.Request
}

func (c *queuedRuleDraftClient) Complete(_ context.Context, req llm.Request) (llm.Response, error) {
	c.requests = append(c.requests, req)
	if len(c.errs) > 0 {
		err := c.errs[0]
		c.errs = c.errs[1:]
		if err != nil {
			return llm.Response{}, err
		}
	}
	if len(c.responses) == 0 {
		return llm.Response{}, stderrors.New("unexpected rule draft request")
	}
	text := c.responses[0]
	c.responses = c.responses[1:]
	return llm.Response{Text: text}, nil
}

func (c *queuedRuleDraftClient) HealthCheck(context.Context) error {
	return nil
}

func newRuleDraftServiceForTest(t *testing.T, client *queuedRuleDraftClient, prompts *llm.PromptCatalog) *RuleDraftService {
	t.Helper()
	registry := llm.NewRegistry()
	if err := registry.RegisterProvider(llm.Provider{
		Metadata: llm.ProviderMetadata{
			Name:         "test",
			DisplayName:  "Test LLM",
			Auth:         llm.AuthSpec{Type: llm.AuthTypeAPIKey, Required: true},
			Capabilities: []llm.Capability{llm.CapabilityTextGeneration, llm.CapabilityJSONSchema},
		},
		NewClient: func(llm.ClientConfig) (llm.Client, error) {
			return client, nil
		},
	}); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	router := llm.NewRouter(llm.RouterConfig{
		Registry: registry,
		Runtime: &ruleDraftRuntimeStore{
			found: true,
			runtime: store.LLMProviderRuntime{
				Provider:       "test",
				Config:         json.RawMessage(`{}`),
				Credentials:    []byte(`{"api_key":"test"}`),
				HasCredentials: true,
				Active:         true,
			},
		},
		Prompts: prompts,
	})
	return NewRuleDraftService(router)
}

func ruleDraftPromptCatalog(t *testing.T) *llm.PromptCatalog {
	t.Helper()
	catalog, err := llm.LoadPromptCatalog(fstest.MapFS{
		"prompts/rule_draft.yaml": &fstest.MapFile{Data: []byte(`
id: rule_draft_test
version: 1
workflow: rule_drafting
purpose: draft_rule
required_capabilities:
  - json_schema
messages:
  - role: system
    content: Draft rule JSON.
  - role: user
    content: "{{rule_context_json}}"
`)},
	}, "prompts")
	if err != nil {
		t.Fatalf("LoadPromptCatalog() error = %v", err)
	}
	return catalog
}

func validRuleDraftInput() RuleDraftInput {
	return RuleDraftInput{
		Current: RuleDraft{
			Name:            " Current ",
			SenderEmails:    []string{" Alerts@Example.com "},
			SubjectContains: "Card alert",
			Source:          api.Source{Type: "Credit Card", Bank: "ICICI", Label: "ICICI Credit Card"},
		},
		Samples: []Sample{{
			Name:    " Sample 1 ",
			Sender:  "alerts@example.com",
			Subject: "Card alert",
			Body:    "INR 1522.00 spent at Amazon",
			Expected: Expected{
				Amount:   "1522.00",
				Merchant: "Amazon",
				Currency: "INR",
			},
		}},
	}
}

func draftJSON(t *testing.T, draft RuleDraft) string {
	t.Helper()
	body, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(body)
}

func matchingDraft() RuleDraft {
	return RuleDraft{
		Name:            "ICICI Credit Card",
		SenderEmails:    []string{"Alerts@Example.com", "alerts@example.com"},
		SubjectContains: "Card alert",
		AmountRegex:     `INR\s+([0-9.]+)`,
		MerchantRegex:   `at\s+([A-Za-z]+)`,
		CurrencyRegex:   `(INR)`,
		Source:          api.Source{Type: "Credit Card", Bank: "ICICI", Label: "ICICI Credit Card"},
		Notes:           "matched sample",
	}
}

func TestRuleDraftServiceReturnsValidatedDraft(t *testing.T) {
	client := &queuedRuleDraftClient{responses: []string{draftJSON(t, matchingDraft())}}
	service := newRuleDraftServiceForTest(t, client, ruleDraftPromptCatalog(t))

	result, err := service.DraftRule(context.Background(), store.Tenant{ID: "tenant-a"}, validRuleDraftInput())
	if err != nil {
		t.Fatalf("DraftRule() error = %v", err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(client.requests))
	}
	req := client.requests[0]
	if req.Workflow != ruleDraftWorkflow || req.Purpose != ruleDraftPurpose {
		t.Fatalf("workflow/purpose = %s/%s", req.Workflow, req.Purpose)
	}
	if req.ResponseFormat.Type != llm.ResponseFormatJSONSchema || !req.ResponseFormat.Strict {
		t.Fatalf("response format = %+v, want strict JSON schema", req.ResponseFormat)
	}
	if result.Draft.SenderEmails[0] != "alerts@example.com" {
		t.Fatalf("sender emails = %#v, want normalized unique sender", result.Draft.SenderEmails)
	}
	if len(result.ValidationIssues) != 0 {
		t.Fatalf("validation issues = %#v, want none", result.ValidationIssues)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(result.Matches))
	}
	match := result.Matches[0]
	if match.SampleIndex != 0 || match.SampleName != "Sample 1" || match.Amount != "1522.00" || match.Merchant != "Amazon" || match.Currency != "INR" {
		t.Fatalf("match = %+v, want validated sample extraction", match)
	}
}

func TestRuleDraftServiceRepairsInvalidDraftBeforeReturning(t *testing.T) {
	first := matchingDraft()
	first.AmountRegex = `INR\s+([0-9]+)`
	repaired := matchingDraft()
	client := &queuedRuleDraftClient{responses: []string{
		draftJSON(t, first),
		draftJSON(t, repaired),
	}}
	service := newRuleDraftServiceForTest(t, client, ruleDraftPromptCatalog(t))

	result, err := service.DraftRule(context.Background(), store.Tenant{ID: "tenant-a"}, validRuleDraftInput())
	if err != nil {
		t.Fatalf("DraftRule() error = %v", err)
	}
	if len(client.requests) != 2 {
		t.Fatalf("requests = %d, want initial draft and repair", len(client.requests))
	}
	repairContext := client.requests[1].Messages[1].Content
	if !strings.Contains(repairContext, "previous draft failed validation") || !strings.Contains(repairContext, "Amount matched") {
		t.Fatalf("repair context did not include validation failure details: %s", repairContext)
	}
	if len(result.ValidationIssues) != 0 {
		t.Fatalf("validation issues = %#v, want repaired draft to pass", result.ValidationIssues)
	}
	if result.Matches[0].Amount != "1522.00" {
		t.Fatalf("amount match = %q, want repaired regex result", result.Matches[0].Amount)
	}
}

func TestRuleDraftServiceReturnsAllRemainingValidationIssuesAfterRepair(t *testing.T) {
	bad := matchingDraft()
	bad.AmountRegex = `INR\s+([0-9]+)`
	bad.MerchantRegex = `spent\s+at\s+([A-Za-z]{3})`
	client := &queuedRuleDraftClient{responses: []string{
		draftJSON(t, bad),
		draftJSON(t, bad),
	}}
	service := newRuleDraftServiceForTest(t, client, ruleDraftPromptCatalog(t))

	result, err := service.DraftRule(context.Background(), store.Tenant{ID: "tenant-a"}, validRuleDraftInput())
	if err != nil {
		t.Fatalf("DraftRule() error = %v", err)
	}
	if len(result.ValidationIssues) != 2 {
		t.Fatalf("validation issues = %#v, want amount and merchant issues", result.ValidationIssues)
	}
	gotFields := []string{result.ValidationIssues[0].Field, result.ValidationIssues[1].Field}
	if strings.Join(gotFields, ",") != "amount,merchant" {
		t.Fatalf("issue fields = %#v, want amount then merchant", gotFields)
	}
	for _, issue := range result.ValidationIssues {
		if issue.SampleIndex != 0 || issue.SampleName != "Sample 1" {
			t.Fatalf("issue sample = %+v, want sample-specific issue", issue)
		}
		if issue.Expected == "" || issue.Actual == "" || issue.Message == "" {
			t.Fatalf("issue = %+v, want expected, actual and message", issue)
		}
	}
}

func TestRuleDraftServiceRejectsInvalidInputsBeforeProviderCall(t *testing.T) {
	client := &queuedRuleDraftClient{responses: []string{draftJSON(t, matchingDraft())}}
	service := newRuleDraftServiceForTest(t, client, ruleDraftPromptCatalog(t))

	_, err := service.DraftRule(context.Background(), store.Tenant{ID: "tenant-a"}, RuleDraftInput{
		Samples: []Sample{{Body: "email body", Expected: Expected{Amount: "10"}}},
	})
	if errors.WhatKind(err) != KindRuleDraftInvalidInput {
		t.Fatalf("DraftRule() error = %v, want KindRuleDraftInvalidInput", err)
	}
	if len(client.requests) != 0 {
		t.Fatalf("requests = %d, want no provider call", len(client.requests))
	}
}

func TestRuleDraftServiceReportsPromptAndOutputFailures(t *testing.T) {
	t.Run("missing prompt", func(t *testing.T) {
		service := newRuleDraftServiceForTest(t, &queuedRuleDraftClient{}, &llm.PromptCatalog{})

		_, err := service.DraftRule(context.Background(), store.Tenant{ID: "tenant-a"}, validRuleDraftInput())
		if errors.WhatKind(err) != KindRuleDraftPromptMissing {
			t.Fatalf("DraftRule() error = %v, want KindRuleDraftPromptMissing", err)
		}
	})

	t.Run("invalid provider output", func(t *testing.T) {
		service := newRuleDraftServiceForTest(t, &queuedRuleDraftClient{responses: []string{"not-json"}}, ruleDraftPromptCatalog(t))

		_, err := service.DraftRule(context.Background(), store.Tenant{ID: "tenant-a"}, validRuleDraftInput())
		if errors.WhatKind(err) != KindRuleDraftInvalidOutput {
			t.Fatalf("DraftRule() error = %v, want KindRuleDraftInvalidOutput", err)
		}
	})
}
