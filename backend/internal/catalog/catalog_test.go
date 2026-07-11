package catalog

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
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

func TestLoadRejectsStructurallyInvalidContent(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "empty rules", path: "content/rules.json", body: `{"version":2,"presets":{},"rules":[]}`},
		{name: "incomplete mcc", path: "content/mcc.json", body: `[{"code":"5411"}]`},
		{name: "incomplete categories", path: "content/categories.json", body: `[{"fragment":"shop"}]`},
		{name: "invalid banks shape", path: "content/banks.json", body: `{}`},
		{name: "empty guide", path: gmailGuidePath, body: `{"sections":[]}`},
		{name: "empty models", path: openAIModelsPath, body: `[]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			contents := bundledMapFS(t)
			contents[tc.path] = &fstest.MapFile{Data: []byte(tc.body)}
			if _, err := loadFromFS(contents); err == nil {
				t.Fatal("loadFromFS() error = nil")
			}
		})
	}
}

func bundledMapFS(t *testing.T) fstest.MapFS {
	t.Helper()
	contents := fstest.MapFS{}
	err := fs.WalkDir(contentFS, "content", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		body, err := fs.ReadFile(contentFS, path)
		if err != nil {
			return err
		}
		contents[path] = &fstest.MapFile{Data: body}
		return nil
	})
	if err != nil {
		t.Fatalf("copy bundled content: %v", err)
	}
	return contents
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
