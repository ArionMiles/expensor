package llm

import (
	"testing"
	"testing/fstest"
)

func TestLoadPromptCatalogAllowsEmptyCatalog(t *testing.T) {
	catalog, err := LoadPromptCatalog(fstest.MapFS{
		"content/llm/prompts/README.md": {Data: []byte("Prompt files are added by workflow PRs.\n")},
	}, "content/llm/prompts")
	if err != nil {
		t.Fatalf("LoadPromptCatalog() error = %v", err)
	}
	if catalog.Len() != 0 {
		t.Fatalf("catalog length = %d, want 0", catalog.Len())
	}
}

func TestLoadPromptCatalogLoadsYAMLPrompts(t *testing.T) {
	catalog, err := LoadPromptCatalog(fstest.MapFS{
		"content/llm/prompts/test.v1.yaml": {Data: []byte(`
id: test.v1
version: 1
workflow: test
purpose: explain
description: Explain a test fixture.
required_capabilities:
  - json_schema
variables:
  - name: input
    description: Input text
    required: true
messages:
  - role: system
    content: You explain tests.
  - role: user
    content: "{{ input }}"
`)},
	}, "content/llm/prompts")
	if err != nil {
		t.Fatalf("LoadPromptCatalog() error = %v", err)
	}

	prompt, ok := catalog.Get("test", "explain")
	if !ok {
		t.Fatal("prompt not found")
	}
	if prompt.ID != "test.v1" || prompt.Version != 1 {
		t.Fatalf("prompt identity = %q v%d, want test.v1 v1", prompt.ID, prompt.Version)
	}
	if len(prompt.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(prompt.Messages))
	}
	if len(prompt.RequiredCapabilities) != 1 || prompt.RequiredCapabilities[0] != CapabilityJSONSchema {
		t.Fatalf("required capabilities = %#v, want json_schema", prompt.RequiredCapabilities)
	}
}

func TestLoadPromptCatalogRejectsDuplicateWorkflowPurpose(t *testing.T) {
	_, err := LoadPromptCatalog(fstest.MapFS{
		"content/llm/prompts/a.v1.yaml": {Data: []byte(`
id: a.v1
version: 1
workflow: test
purpose: explain
messages:
  - role: user
    content: first
`)},
		"content/llm/prompts/b.v1.yaml": {Data: []byte(`
id: b.v1
version: 1
workflow: test
purpose: explain
messages:
  - role: user
    content: second
`)},
	}, "content/llm/prompts")
	if err == nil {
		t.Fatal("expected duplicate prompt error, got nil")
	}
}
