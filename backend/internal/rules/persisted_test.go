package rules

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

type persistedStoreStub struct {
	rows []store.RuleRow
	err  error
}

func (s persistedStoreStub) ListRules(context.Context, store.Tenant) ([]store.RuleRow, error) {
	return s.rows, s.err
}

func TestLoadPersistedSkipsPredefinedAndInvalidRules(t *testing.T) {
	valid := store.RuleRow{
		ID: "rule-1", Name: "Valid", SenderEmail: "alerts@example.test", SenderEmails: []string{"alerts@example.test"},
		SubjectContains: "spent", AmountRegex: `([0-9.]+)`, MerchantRegex: `at ([A-Z]+)`, CurrencyRegex: `(INR)`,
		SourceType: "card", SourceLabel: "Card", Bank: "Example Bank",
	}
	rows := []store.RuleRow{
		{Predefined: true, Name: "Bundled", AmountRegex: `([0-9.]+)`, MerchantRegex: `at ([A-Z]+)`},
		{Name: "Bad amount", AmountRegex: `(`, MerchantRegex: `ok`},
		{Name: "Bad merchant", AmountRegex: `ok`, MerchantRegex: `(`},
		{Name: "Bad currency", AmountRegex: `ok`, MerchantRegex: `ok`, CurrencyRegex: `(`},
		valid,
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	got := LoadPersisted(context.Background(), persistedStoreStub{rows: rows}, store.Tenant{ID: "tenant-a"}, logger)
	if len(got) != 1 {
		t.Fatalf("LoadPersisted() returned %d rules, want 1", len(got))
	}
	if got[0].ID != valid.ID || got[0].SenderEmail != valid.SenderEmail || got[0].Source.Type != valid.SourceType ||
		got[0].Source.Label != valid.SourceLabel || got[0].Source.Bank != valid.Bank {
		t.Fatalf("compiled rule = %#v", got[0])
	}
	if count := strings.Count(logs.String(), "skipping rule with invalid regex"); count != 3 {
		t.Fatalf("invalid-rule log count = %d, want 3; logs: %s", count, logs.String())
	}
}
