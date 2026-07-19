package catalog

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ArionMiles/expensor/backend/internal/llm"
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
	if content.PromptCatalog == nil || content.PromptCatalog.Len() == 0 || len(content.LLMProviders) != 2 {
		t.Fatal("Load() returned incomplete LLM content")
	}
	assertBundledLLMProvider(t, content.LLMProviders["openai"], bundledLLMProviderExpectation{
		displayName: "OpenAI", apiKeyLinkText: "OpenAI dashboard", defaultModel: "gpt-5.4-mini", wantBaseURL: true,
	})
	assertBundledLLMProvider(t, content.LLMProviders["gemini"], bundledLLMProviderExpectation{
		displayName: "Gemini", apiKeyLinkText: "Google AI dashboard", defaultModel: "gemini-3.5-flash",
	})
}

type bundledLLMProviderExpectation struct {
	displayName    string
	apiKeyLinkText string
	defaultModel   string
	wantBaseURL    bool
}

func assertBundledLLMProvider(
	t *testing.T,
	provider llm.ProviderMetadata,
	want bundledLLMProviderExpectation,
) {
	t.Helper()
	if provider.Name == "" || provider.DisplayName != want.displayName || provider.APIKeyLinkText != want.apiKeyLinkText {
		t.Fatalf("provider metadata = %#v", provider)
	}
	if !strings.HasPrefix(provider.APIKeyURL, "https://") || !strings.HasPrefix(provider.DataUse.PolicyURL, "https://") {
		t.Fatalf("provider URLs = %q, %q, want HTTPS", provider.APIKeyURL, provider.DataUse.PolicyURL)
	}
	if provider.Auth.Type != llm.AuthTypeAPIKey || !provider.Auth.Required || len(provider.Capabilities) != 0 {
		t.Fatalf("provider auth/capabilities = %#v/%#v, want required API key and catalog-owned capabilities omitted", provider.Auth, provider.Capabilities)
	}
	if model, ok := llm.ConfigStringDefault(provider.ConfigSchema, "model"); !ok || model != want.defaultModel {
		t.Fatalf("model default = %q, %v, want %q, true", model, ok, want.defaultModel)
	}
	_, hasBaseURL := llm.ConfigStringDefault(provider.ConfigSchema, "base_url")
	if hasBaseURL != want.wantBaseURL {
		t.Fatalf("base URL default present = %v, want %v", hasBaseURL, want.wantBaseURL)
	}
	if len(provider.ModelOptions) < 2 || !provider.ModelOptions[0].Recommended || provider.ModelOptions[0].ID != want.defaultModel {
		t.Fatalf("model options = %#v, want recommended default first", provider.ModelOptions)
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
		{name: "incomplete provider", path: openAIProviderPath, body: `{"name":"openai"}`},
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

func TestLoadRejectsInvalidLLMProviderCatalogs(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		mutate func(map[string]any)
	}{
		{name: "capabilities are implementation owned", path: geminiProviderPath, mutate: func(doc map[string]any) {
			doc["capabilities"] = []any{}
		}},
		{name: "unknown field", path: geminiProviderPath, mutate: func(doc map[string]any) {
			doc["display_nam"] = "Gemini"
		}},
		{name: "provider name mismatch", path: geminiProviderPath, mutate: func(doc map[string]any) {
			doc["name"] = "other"
		}},
		{name: "invalid API key URL", path: geminiProviderPath, mutate: func(doc map[string]any) {
			doc["api_key_url"] = "http://example.com/key"
		}},
		{name: "missing API key link text", path: geminiProviderPath, mutate: func(doc map[string]any) {
			doc["api_key_link_text"] = " "
		}},
		{name: "unsupported data use mode", path: geminiProviderPath, mutate: func(doc map[string]any) {
			dataUse := doc["data_use"].(map[string]any)
			dataUse["mode"] = "unknown"
		}},
		{name: "optional API key auth", path: geminiProviderPath, mutate: func(doc map[string]any) {
			auth := doc["auth"].(map[string]any)
			auth["required"] = false
		}},
		{name: "missing model default", path: geminiProviderPath, mutate: func(doc map[string]any) {
			configSchema := doc["config_schema"].(map[string]any)
			properties := configSchema["properties"].(map[string]any)
			delete(properties, "model")
		}},
		{name: "OpenAI missing base URL default", path: openAIProviderPath, mutate: func(doc map[string]any) {
			configSchema := doc["config_schema"].(map[string]any)
			properties := configSchema["properties"].(map[string]any)
			delete(properties, "base_url")
		}},
		{name: "empty models", path: geminiProviderPath, mutate: func(doc map[string]any) {
			doc["model_options"] = []any{}
		}},
		{name: "recommended model differs from default", path: geminiProviderPath, mutate: func(doc map[string]any) {
			models := doc["model_options"].([]any)
			models[0].(map[string]any)["recommended"] = false
			models[1].(map[string]any)["recommended"] = true
		}},
		{name: "multiple recommended models", path: geminiProviderPath, mutate: func(doc map[string]any) {
			models := doc["model_options"].([]any)
			models[1].(map[string]any)["recommended"] = true
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			contents := bundledMapFS(t)
			mutateCatalogJSON(t, contents, tc.path, tc.mutate)
			if _, err := loadFromFS(contents); err == nil {
				t.Fatal("loadFromFS() error = nil")
			}
		})
	}
}

func mutateCatalogJSON(t *testing.T, contents fstest.MapFS, path string, mutate func(map[string]any)) {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(contents[path].Data, &document); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	mutate(document)
	body, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	contents[path] = &fstest.MapFile{Data: body}
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
