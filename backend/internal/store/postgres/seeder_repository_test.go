package postgres

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

func TestBuildSystemRuleRowsPreservesV2SourceAndSenders(t *testing.T) {
	rows := buildSystemRuleRows([]api.Rule{
		{
			Name:            "HDFC Credit Card",
			SenderEmails:    []string{"alerts@hdfcbank.bank.in", "alerts@hdfcbank.net"},
			SubjectContains: "Alert",
			Amount:          regexp.MustCompile(`Rs\. ([\d.]+)`),
			MerchantInfo:    regexp.MustCompile(`at (.*?) on`),
			Source:          api.Source{Type: "Credit Card", Label: "HDFC Credit Card", Bank: "HDFC"},
		},
	})

	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.SenderEmail != "alerts@hdfcbank.bank.in" {
		t.Fatalf("SenderEmail = %q", row.SenderEmail)
	}
	if strings.Join(row.SenderEmails, ",") != "alerts@hdfcbank.bank.in,alerts@hdfcbank.net" {
		t.Fatalf("SenderEmails = %#v", row.SenderEmails)
	}
	if row.SourceType != "Credit Card" || row.SourceLabel != "HDFC Credit Card" || row.Bank != "HDFC" {
		t.Fatalf("source = (%q, %q, %q)", row.SourceType, row.SourceLabel, row.Bank)
	}
	if row.TransactionSource != "HDFC Credit Card" {
		t.Fatalf("TransactionSource = %q", row.TransactionSource)
	}
}
