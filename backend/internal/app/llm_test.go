package app

import (
	"log/slog"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/catalog"
	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestNewLLMRuntimeRegistersCatalogProviders(t *testing.T) {
	content, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load() error = %v", err)
	}
	runtime, err := newLLMRuntime(content, nil, slog.Default())
	if err != nil {
		t.Fatalf("newLLMRuntime() error = %v", err)
	}
	providers := runtime.registry.ListProviders()
	if len(providers) != 2 || providers[0].Metadata.Name != "gemini" || providers[1].Metadata.Name != "openai" {
		t.Fatalf("providers = %#v, want Gemini and OpenAI", providers)
	}
	for _, provider := range providers {
		if err := provider.RequireCapabilities(llm.CapabilityTextGeneration, llm.CapabilityJSONSchema); err != nil {
			t.Fatalf("provider %q capabilities error = %v", provider.Metadata.Name, err)
		}
	}
}

func TestNewLLMRuntimeRequiresBothProviderCatalogs(t *testing.T) {
	content, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load() error = %v", err)
	}
	for _, name := range []string{"gemini", "openai"} {
		t.Run(name, func(t *testing.T) {
			providers := make(map[string]llm.ProviderMetadata, len(content.LLMProviders)-1)
			for providerName, metadata := range content.LLMProviders {
				if providerName != name {
					providers[providerName] = metadata
				}
			}
			contentWithoutProvider := content
			contentWithoutProvider.LLMProviders = providers
			if _, err := newLLMRuntime(contentWithoutProvider, nil, slog.Default()); errors.WhatKind(err) != errors.Internal {
				t.Fatalf("newLLMRuntime() error = %v, want internal", err)
			}
		})
	}
}
