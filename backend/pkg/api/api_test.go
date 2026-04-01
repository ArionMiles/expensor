package api_test

import (
	"regexp"
	"testing"

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
			want: `is:unread from:alerts@bank.com subject:"Transaction Alert"`,
		},
		{
			name: "only sender",
			rule: api.Rule{SenderEmail: "alerts@bank.com"},
			want: "is:unread from:alerts@bank.com",
		},
		{
			name: "only subject",
			rule: api.Rule{SubjectContains: "Transaction Alert"},
			want: `is:unread subject:"Transaction Alert"`,
		},
		{
			name: "neither sender nor subject",
			rule: api.Rule{},
			want: "is:unread",
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

func TestLabels_LabelLookup(t *testing.T) {
	labels := api.Labels{
		"AMAZON":      {Category: "Shopping", Bucket: "Wants"},
		"SWIGGY":      {Category: "Food", Bucket: "Wants"},
		"ELECTRICITY": {Category: "Utilities", Bucket: "Needs"},
	}

	tests := []struct {
		merchant     string
		wantCategory string
		wantBucket   string
	}{
		{"AMAZON", "Shopping", "Wants"},
		{"SWIGGY", "Food", "Wants"},
		{"ELECTRICITY", "Utilities", "Needs"},
		{"UNKNOWN", "", ""},
		{"amazon", "", ""}, // case-sensitive: no match
	}

	for _, tc := range tests {
		t.Run(tc.merchant, func(t *testing.T) {
			got, bucket := labels.LabelLookup(tc.merchant)
			if got != tc.wantCategory {
				t.Errorf("category: got %q, want %q", got, tc.wantCategory)
			}
			if bucket != tc.wantBucket {
				t.Errorf("bucket: got %q, want %q", bucket, tc.wantBucket)
			}
		})
	}
}

func TestLabels_LabelLookup_EmptyMap(t *testing.T) {
	labels := api.Labels{}
	cat, bucket := labels.LabelLookup("ANYTHING")
	if cat != "" || bucket != "" {
		t.Errorf("expected empty strings for empty map, got category=%q bucket=%q", cat, bucket)
	}
}

// Compile-time check that Rule has the regex fields we expect.
var _ = api.Rule{Amount: regexp.MustCompile(""), MerchantInfo: regexp.MustCompile("")}
