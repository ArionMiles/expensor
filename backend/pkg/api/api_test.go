package api_test

import (
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

func TestRule_BuildGmailQuery(t *testing.T) {
	tests := []struct {
		name string
		rule api.Rule
		want string
	}{
		{
			name: "both sender and subject",
			rule: api.Rule{SenderEmail: "alerts@bank.com", SubjectContains: "Transaction Alert"},
			want: `from:alerts@bank.com subject:"Transaction Alert"`,
		},
		{
			name: "only sender",
			rule: api.Rule{SenderEmail: "alerts@bank.com"},
			want: "from:alerts@bank.com",
		},
		{
			name: "only subject",
			rule: api.Rule{SubjectContains: "Transaction Alert"},
			want: `subject:"Transaction Alert"`,
		},
		{
			name: "neither sender nor subject",
			rule: api.Rule{},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.rule.BuildGmailQuery()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRule_MatchesEmail(t *testing.T) {
	tests := []struct {
		name    string
		rule    api.Rule
		from    string
		subject string
		want    bool
	}{
		{
			name:    "match by sender exact",
			rule:    api.Rule{SenderEmail: "alerts@bank.com"},
			from:    "Bank Alerts <alerts@bank.com>",
			subject: "anything",
			want:    true,
		},
		{
			name:    "match by sender case insensitive",
			rule:    api.Rule{SenderEmail: "ALERTS@BANK.COM"},
			from:    "alerts@bank.com",
			subject: "",
			want:    true,
		},
		{
			name:    "match by subject",
			rule:    api.Rule{SubjectContains: "Transaction"},
			from:    "",
			subject: "Your Transaction Alert",
			want:    true,
		},
		{
			name:    "match by subject case insensitive",
			rule:    api.Rule{SubjectContains: "transaction"},
			from:    "",
			subject: "TRANSACTION ALERT",
			want:    true,
		},
		{
			name:    "match by both sender and subject",
			rule:    api.Rule{SenderEmail: "bank@example.com", SubjectContains: "Alert"},
			from:    "bank@example.com",
			subject: "Payment Alert",
			want:    true,
		},
		{
			name:    "no match wrong sender",
			rule:    api.Rule{SenderEmail: "real@bank.com"},
			from:    "phishing@evil.com",
			subject: "anything",
			want:    false,
		},
		{
			name:    "no match wrong subject",
			rule:    api.Rule{SubjectContains: "Transaction"},
			from:    "",
			subject: "Newsletter",
			want:    false,
		},
		{
			name:    "sender matches but subject does not",
			rule:    api.Rule{SenderEmail: "bank@example.com", SubjectContains: "Alert"},
			from:    "bank@example.com",
			subject: "Newsletter",
			want:    false,
		},
		{
			name:    "hdfc payment sender and subject match",
			rule:    api.Rule{SenderEmail: "alerts@hdfcbank.bank.in", SubjectContains: "A payment was made using your Credit Card"},
			from:    "HDFC Alerts <alerts@hdfcbank.bank.in>",
			subject: "A payment was made using your Credit Card",
			want:    true,
		},
		{
			name:    "hdfc payment sender .net and subject match",
			rule:    api.Rule{SenderEmail: "alerts@hdfcbank.net", SubjectContains: "A payment was made using your Credit Card"},
			from:    "HDFC Alerts <alerts@hdfcbank.net>",
			subject: "A payment was made using your Credit Card",
			want:    true,
		},
		{
			name:    "hdfc marketing sender does not match strict sender rule",
			rule:    api.Rule{SenderEmail: "alerts@hdfcbank.bank.in", SubjectContains: "A payment was made using your Credit Card"},
			from:    "HDFC Info <information@hdfcbank.ne>",
			subject: "Kanishk, Great News! Your FREE & Upgraded Credit Card Limit Awaits! 🎁",
			want:    false,
		},
		{
			name:    "hdfc domain .net and strict subject match",
			rule:    api.Rule{SenderEmail: "@hdfcbank.net", SubjectContains: "debited via Credit Card"},
			from:    "HDFC Alerts <alerts@hdfcbank.net>",
			subject: "Rs.868.00 debited via Credit Card **1234",
			want:    true,
		},
		{
			name:    "empty rule matches everything",
			rule:    api.Rule{},
			from:    "anyone@anywhere.com",
			subject: "anything at all",
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.rule.MatchesEmail(tc.from, tc.subject)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCategoryResolver(t *testing.T) {
	resolver := api.CategoryResolver(func(merchant string) (string, string) {
		switch merchant {
		case "AMAZON":
			return "Shopping", "Wants"
		case "SWIGGY":
			return "Food", "Wants"
		case "ELECTRICITY":
			return "Utilities", "Needs"
		default:
			return "", ""
		}
	})

	tests := []struct {
		merchant     string
		wantCategory string
		wantBucket   string
	}{
		{"AMAZON", "Shopping", "Wants"},
		{"SWIGGY", "Food", "Wants"},
		{"ELECTRICITY", "Utilities", "Needs"},
		{"UNKNOWN", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.merchant, func(t *testing.T) {
			got, bucket := resolver(tc.merchant)
			if got != tc.wantCategory {
				t.Errorf("category: got %q, want %q", got, tc.wantCategory)
			}
			if bucket != tc.wantBucket {
				t.Errorf("bucket: got %q, want %q", bucket, tc.wantBucket)
			}
		})
	}
}

func TestRuleDiagnosticSnapshot(t *testing.T) {
	rule := api.Rule{
		ID:              "rule-1",
		Name:            "Card",
		Amount:          regexp.MustCompile(`Amount: ([\d.]+)`),
		MerchantInfo:    regexp.MustCompile(`at (.+)`),
		Currency:        regexp.MustCompile(`Currency: ([A-Z]{3})`),
		Source:          "Credit Card",
		SenderEmail:     "alerts@example.com",
		SubjectContains: "spent",
	}

	snapshot := rule.DiagnosticSnapshot()

	if snapshot.RuleID != "rule-1" || snapshot.RuleName != "Card" {
		t.Fatalf("unexpected rule identity: %+v", snapshot)
	}
	if snapshot.AmountRegex != `Amount: ([\d.]+)` {
		t.Fatalf("amount regex = %q", snapshot.AmountRegex)
	}
	if snapshot.MerchantRegex != `at (.+)` {
		t.Fatalf("merchant regex = %q", snapshot.MerchantRegex)
	}
	if snapshot.CurrencyRegex != `Currency: ([A-Z]{3})` {
		t.Fatalf("currency regex = %q", snapshot.CurrencyRegex)
	}
}

func TestExtractionFailureReasons(t *testing.T) {
	reasons := api.ExtractionFailureReasons(nil)
	if reasons != nil {
		t.Fatalf("expected nil reasons for nil transaction, got %v", reasons)
	}

	reasons = api.ExtractionFailureReasons(&api.TransactionDetails{Amount: 0, MerchantInfo: ""})
	if diff := cmp.Diff([]string{api.FailureAmountZero, api.FailureMerchantEmpty}, reasons); diff != "" {
		t.Fatalf("reasons mismatch (-want +got):\n%s", diff)
	}

	reasons = api.ExtractionFailureReasons(&api.TransactionDetails{Amount: 42, MerchantInfo: " \t\n"})
	if diff := cmp.Diff([]string{api.FailureMerchantEmpty}, reasons); diff != "" {
		t.Fatalf("reasons mismatch (-want +got):\n%s", diff)
	}

	reasons = api.ExtractionFailureReasons(&api.TransactionDetails{Amount: 42, MerchantInfo: "Cafe"})
	if len(reasons) != 0 {
		t.Fatalf("expected no reasons, got %v", reasons)
	}
}

// Compile-time check that Rule has the regex fields we expect.
var _ = api.Rule{Amount: regexp.MustCompile(""), MerchantInfo: regexp.MustCompile("")}
