package catalog

import (
	"strings"
	"testing"
)

func TestLoadValidatesBundledContent(t *testing.T) {
	content, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(content.SystemRules) == 0 || len(content.Seed.MCCEntries) == 0 || len(content.Seed.MerchantCategories) == 0 {
		t.Fatalf("Load() returned incomplete seed content: %#v", content.Seed)
	}
	if len(content.BanksJSON) == 0 || len(content.ReaderGuides["gmail"]) == 0 || len(content.ReaderGuides["thunderbird"]) == 0 {
		t.Fatal("Load() returned incomplete HTTP and reader content")
	}
	if content.PromptCatalog == nil || content.PromptCatalog.Len() == 0 || len(content.OpenAIModelOptions) == 0 {
		t.Fatal("Load() returned incomplete LLM content")
	}
}

func TestLoadICICICreditCardCoversBothExactSenders(t *testing.T) {
	content, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := map[string]bool{
		"credit_cards@icicibank.com": false,
		"credit_cards@icici.bank.in": false,
	}
	for _, rule := range content.SystemRules {
		if !strings.Contains(rule.Name, "ICICI Credit Card") {
			continue
		}
		for _, sender := range rule.SenderEmails {
			if _, ok := want[sender]; ok {
				want[sender] = true
			}
		}
	}
	for sender, found := range want {
		if !found {
			t.Errorf("expected ICICI credit card rule for sender %q", sender)
		}
	}
}
