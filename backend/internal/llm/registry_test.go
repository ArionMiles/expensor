package llm

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type stubClient struct{}

func (c stubClient) Complete(context.Context, Request) (Response, error) {
	return Response{Text: "ok"}, nil
}

func (c stubClient) HealthCheck(context.Context) error {
	return nil
}

func testProvider(name string, capabilities ...Capability) Provider {
	return Provider{
		Metadata: ProviderMetadata{
			Name:         name,
			DisplayName:  "Test Provider",
			Auth:         AuthSpec{Type: AuthTypeAPIKey, Required: true},
			Capabilities: capabilities,
		},
		NewClient: func(ClientConfig) (Client, error) {
			return stubClient{}, nil
		},
	}
}

func TestConfigStringDefault(t *testing.T) {
	schema := json.RawMessage(`{
		"type":"object",
		"properties":{
			"model":{"type":"string","default":" model-a "},
			"count":{"type":"integer","default":1},
			"empty":{"type":"string","default":" "}
		}
	}`)

	if value, ok := ConfigStringDefault(schema, "model"); !ok || value != "model-a" {
		t.Fatalf("ConfigStringDefault(model) = %q, %v, want model-a, true", value, ok)
	}
	for _, field := range []string{"count", "empty", "missing"} {
		if value, ok := ConfigStringDefault(schema, field); ok || value != "" {
			t.Fatalf("ConfigStringDefault(%s) = %q, %v, want empty, false", field, value, ok)
		}
	}
	if value, ok := ConfigStringDefault(json.RawMessage(`{"bad"`), "model"); ok || value != "" {
		t.Fatalf("ConfigStringDefault(invalid) = %q, %v, want empty, false", value, ok)
	}
}

func TestValidateConfig(t *testing.T) {
	schema := json.RawMessage(`{
		"type":"object",
		"properties":{
			"model":{"type":"string"},
			"max_tokens":{"type":"integer"},
			"enabled":{"type":"boolean"}
		}
	}`)

	if err := ValidateConfig(schema, json.RawMessage(`{"model":"model-a","max_tokens":128,"enabled":true}`)); err != nil {
		t.Fatalf("ValidateConfig(valid) error = %v", err)
	}
	tests := []struct {
		name   string
		config string
	}{
		{name: "unknown field", config: `{"base_url":"https://attacker.invalid"}`},
		{name: "wrong type", config: `{"max_tokens":"128"}`},
		{name: "non-object", config: `[]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateConfig(schema, json.RawMessage(tc.config)); errors.WhatKind(err) != errors.InvalidInput {
				t.Fatalf("ValidateConfig(%s) error = %v, want invalid input", tc.config, err)
			}
		})
	}
}

func TestRegistryRegisterAndGetProvider(t *testing.T) {
	registry := NewRegistry()
	provider := testProvider("test-provider", CapabilityTools)

	if err := registry.RegisterProvider(provider); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	got, err := registry.GetProvider("test-provider")
	if err != nil {
		t.Fatalf("GetProvider() error = %v", err)
	}
	if got.Metadata.Name != provider.Metadata.Name {
		t.Fatalf("provider name = %q, want %q", got.Metadata.Name, provider.Metadata.Name)
	}
}

func TestRegistryRejectsInvalidProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		wantErr  string
	}{
		{
			name:     "blank name",
			provider: testProvider(" \t\n"),
			wantErr:  "llm.Registry.RegisterProvider: llm provider name is required",
		},
		{
			name: "missing client factory",
			provider: func() Provider {
				provider := testProvider("test-provider")
				provider.NewClient = nil
				return provider
			}(),
			wantErr: `llm.Registry.RegisterProvider: llm provider "test-provider" client factory is required`,
		},
		{
			name: "invalid config schema",
			provider: func() Provider {
				provider := testProvider("test-provider")
				provider.Metadata.ConfigSchema = json.RawMessage(`{"bad"`)
				return provider
			}(),
			wantErr: `llm.Registry.RegisterProvider: llm provider "test-provider" config schema must be valid JSON`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewRegistry().RegisterProvider(tt.provider)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRegistryRejectsDuplicateProvider(t *testing.T) {
	registry := NewRegistry()
	provider := testProvider("test-provider")
	if err := registry.RegisterProvider(provider); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	err := registry.RegisterProvider(provider)
	if err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
	if errors.WhatKind(err) != errors.Conflict {
		t.Fatalf("error = %v, want Conflict", err)
	}
}

func TestProviderSupportsCapabilities(t *testing.T) {
	provider := testProvider("test-provider", CapabilityTools, CapabilityJSONSchema)

	if err := provider.RequireCapabilities(CapabilityTools); err != nil {
		t.Fatalf("RequireCapabilities(CapabilityTools) error = %v", err)
	}
	if err := provider.RequireCapabilities(CapabilityStreaming); errors.WhatKind(err) != KindCapabilityUnsupported {
		t.Fatalf("RequireCapabilities(CapabilityStreaming) error = %v, want KindCapabilityUnsupported", err)
	} else if message := errors.UserMsg(err); message != "The active LLM provider does not support the requested operation." {
		t.Fatalf("UserMsg() = %q", message)
	}
}
