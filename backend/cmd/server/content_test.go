package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

func TestParseRules_ICICICreditCardCoversBothExactSenders(t *testing.T) {
	rules, err := parseRules(rulesInput)
	if err != nil {
		t.Fatalf("parseRules() error = %v", err)
	}

	wantSenders := map[string]bool{
		"credit_cards@icicibank.com": false,
		"credit_cards@icici.bank.in": false,
	}

	for _, rule := range rules {
		if !strings.Contains(rule.Name, "ICICI Credit Card") {
			continue
		}
		for _, sender := range rule.SenderEmails {
			if _, ok := wantSenders[sender]; ok {
				wantSenders[sender] = true
			}
		}
	}

	for sender, found := range wantSenders {
		if !found {
			t.Errorf("expected ICICI credit card rule for sender %q", sender)
		}
	}
}

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
