package main

import (
	"strings"
	"testing"
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
