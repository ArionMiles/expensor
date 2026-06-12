package rules_test

import (
	"reflect"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/rules"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

func r(name string) api.Rule { return api.Rule{Name: name} }

func TestMergeRules_UserOverridesSystem(t *testing.T) {
	system := []api.Rule{r("A"), r("B")}
	user := []api.Rule{{Name: "B", SenderEmail: "override@example.com"}} // user edits B
	got := rules.MergeRules(system, user)
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
	if got[1].Name != "B" || got[1].SenderEmail != "override@example.com" {
		t.Errorf("user override of B not applied, got %+v", got[1])
	}
}

func TestParseDocumentV2(t *testing.T) {
	body := []byte(`{
		"version": 2,
		"presets": {
			"source_types": [
				{"value": "Credit Card", "origin": "predefined"}
			],
			"banks": [
				{"value": "HDFC", "origin": "predefined"}
			]
		},
		"rules": [{
			"name": "HDFC Credit Card",
			"sender_emails": ["Alerts@HDFCBank.net", "alerts@hdfcbank.bank.in"],
			"subject_contains": "HDFC Bank Credit Card",
			"amount_regex": "Rs\\.\\s*([\\d,]+(?:\\.\\d+)?)",
			"merchant_regex": "\\bat\\b (.*?) on",
			"currency_regex": "",
			"source": {"type": "Credit Card", "label": "HDFC Credit Card", "bank": "HDFC"}
		}]
	}`)

	doc, err := rules.ParseDocument(body)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if doc.Version != 2 {
		t.Fatalf("version = %d, want 2", doc.Version)
	}
	if got := doc.Presets.SourceTypes; !reflect.DeepEqual(got, []rules.PresetValue{{Value: "Credit Card", Origin: "predefined"}}) {
		t.Fatalf("source type presets = %#v", got)
	}
	rule := doc.Rules[0]
	if got := rule.SenderEmails; !reflect.DeepEqual(got, []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"}) {
		t.Fatalf("sender emails = %#v", got)
	}
	if rule.Source.Type != "Credit Card" || rule.Source.Bank != "HDFC" || rule.Source.Label != "HDFC Credit Card" {
		t.Fatalf("source = %#v", rule.Source)
	}
	if rule.Amount == nil || rule.MerchantInfo == nil {
		t.Fatalf("compiled regexes missing: %#v", rule)
	}
}

func TestParseDocumentLegacyArray(t *testing.T) {
	body := []byte(`[{
		"name": "Legacy HDFC",
		"senderEmail": "alerts@hdfcbank.net",
		"subjectContains": "HDFC",
		"amountRegex": "Rs\\.([\\d.]+)",
		"merchantInfoRegex": "at (.*?) on",
		"source": "Credit Card - HDFC"
	}]`)

	doc, err := rules.ParseDocument(body)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	rule := doc.Rules[0]
	if got := rule.SenderEmails; !reflect.DeepEqual(got, []string{"alerts@hdfcbank.net"}) {
		t.Fatalf("sender emails = %#v", got)
	}
	if rule.Source.Type != "Credit Card" || rule.Source.Bank != "HDFC" || rule.Source.Label != "Credit Card - HDFC" {
		t.Fatalf("source = %#v", rule.Source)
	}
}

func TestRuleMatchesEmailExactSenderAddress(t *testing.T) {
	rule := api.Rule{SenderEmails: []string{"alerts@hdfcbank.net"}, SubjectContains: "statement"}
	if !rule.MatchesEmail("HDFC <alerts@hdfcbank.net>", "Monthly statement") {
		t.Fatal("expected exact parsed address match")
	}
	if rule.MatchesEmail("alerts@hdfcbank.net.evil.example", "Monthly statement") {
		t.Fatal("expected substring sender mismatch")
	}
}

func TestMergeRules_UserOnlyAppended(t *testing.T) {
	system := []api.Rule{r("A")}
	user := []api.Rule{r("C")} // C not in system
	got := rules.MergeRules(system, user)
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
	if got[1].Name != "C" {
		t.Errorf("expected user-only rule C appended last, got %q", got[1].Name)
	}
}

func TestMergeRules_EmptyUser(t *testing.T) {
	system := []api.Rule{r("A"), r("B")}
	got := rules.MergeRules(system, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
}

func TestMergeRules_EmptySystem(t *testing.T) {
	user := []api.Rule{r("X")}
	got := rules.MergeRules(nil, user)
	if len(got) != 1 || got[0].Name != "X" {
		t.Error("expected user-only rule X returned")
	}
}

func TestMergeRules_BothEmpty(t *testing.T) {
	got := rules.MergeRules(nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d rules", len(got))
	}
}
