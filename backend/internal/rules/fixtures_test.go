package rules_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/extractor"
	"github.com/ArionMiles/expensor/backend/internal/rules"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

var fixedFixtureTime = time.Date(2026, time.April, 12, 10, 30, 0, 0, time.UTC)

func TestRuleEmailFixtures(t *testing.T) {
	doc := loadRulesDocument(t, "../../cmd/server/content/rules.json")
	fixtures := loadFixtures(t, "../../../tests/data/rule-emails")
	if len(fixtures) == 0 {
		t.Fatal("expected at least one rule email fixture")
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.TestName, func(t *testing.T) {
			rule := findRule(t, doc.Rules, fixture.Rule)
			if !rule.MatchesEmail(fixture.Sender, fixture.Subject) {
				t.Fatalf("fixture did not match rule sender/subject")
			}

			tx := extractor.ExtractTransactionDetails(fixture.Body, rule.Amount, rule.MerchantInfo, rule.Currency, fixedFixtureTime)
			if tx.Amount != fixture.Expected.Amount {
				t.Fatalf("amount = %v, want %v", tx.Amount, fixture.Expected.Amount)
			}
			if tx.MerchantInfo != fixture.Expected.Merchant {
				t.Fatalf("merchant = %q, want %q", tx.MerchantInfo, fixture.Expected.Merchant)
			}
			if got := fixtureCurrency(tx); got != fixture.Expected.Currency {
				t.Fatalf("currency = %q, want %q", got, fixture.Expected.Currency)
			}
		})
	}
}

func TestRuleEmailFixturesCoverBundledRules(t *testing.T) {
	doc := loadRulesDocument(t, "../../cmd/server/content/rules.json")
	fixtures := loadFixtures(t, "../../../tests/data/rule-emails")
	coveredRules := make(map[string]bool, len(fixtures))
	for _, fixture := range fixtures {
		coveredRules[fixture.Rule] = true
	}

	for _, rule := range doc.Rules {
		if !coveredRules[rule.Name] {
			t.Fatalf("rule %q has no email fixture", rule.Name)
		}
	}
}

func TestLoadEmailFixturesParsesFrontMatterBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hdfc_credit-card_classic-spend.rule.fixture")
	data := []byte(`---
rule: HDFC Credit Card
sender: alerts@hdfcbank.net
subject: "Alert : Update on your HDFC Bank Credit Card"
expected:
  amount: 999.00
  merchant: SWIGGY
  currency: INR
---
Dear Customer,
Rs.999.00 spent at SWIGGY on your HDFC Credit Card on 12-Apr-2026.
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	fixtures, err := rules.LoadEmailFixtures(dir)
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	if len(fixtures) != 1 {
		t.Fatalf("len(fixtures) = %d, want 1", len(fixtures))
	}
	if got, want := fixtures[0].Body, "Dear Customer,\nRs.999.00 spent at SWIGGY on your HDFC Credit Card on 12-Apr-2026.\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func loadRulesDocument(t *testing.T, path string) *rules.Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read rules document: %v", err)
	}
	doc, err := rules.ParseDocument(data)
	if err != nil {
		t.Fatalf("parse rules document: %v", err)
	}
	return doc
}

func loadFixtures(t *testing.T, dir string) []rules.EmailFixture {
	t.Helper()
	fixtures, err := rules.LoadEmailFixtures(dir)
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	return fixtures
}

func findRule(t *testing.T, all []api.Rule, name string) api.Rule {
	t.Helper()
	for _, rule := range all {
		if rule.Name == name {
			return rule
		}
	}
	t.Fatalf("rule %q not found", name)
	return api.Rule{}
}

func fixtureCurrency(tx *api.TransactionDetails) string {
	if tx.Currency == "" {
		return "INR"
	}
	return tx.Currency
}
