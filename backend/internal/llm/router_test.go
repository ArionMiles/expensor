package llm

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type stubRuntimeStore struct {
	runtime store.LLMProviderRuntime
	found   bool
	err     error
}

func (s stubRuntimeStore) GetActiveLLMProviderRuntime(context.Context, store.Tenant) (store.LLMProviderRuntime, bool, error) {
	return s.runtime, s.found, s.err
}

func TestRouterReturnsNoProviderConfigured(t *testing.T) {
	router := NewRouter(RouterConfig{
		Registry: NewRegistry(),
		Runtime:  stubRuntimeStore{},
	})

	_, err := router.Complete(context.Background(), store.Tenant{ID: "tenant-a"}, Request{})
	if errors.WhatKind(err) != KindNoProviderConfigured {
		t.Fatalf("Complete() error = %v, want KindNoProviderConfigured", err)
	}
	if message := errors.UserMsg(err); message != "No LLM provider is configured." {
		t.Fatalf("UserMsg() = %q", message)
	}
}

func TestRouterRequiresProviderCapabilities(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterProvider(testProvider("test-provider", CapabilityTextGeneration)); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	router := NewRouter(RouterConfig{
		Registry: registry,
		Runtime: stubRuntimeStore{
			found: true,
			runtime: store.LLMProviderRuntime{
				Provider: "test-provider",
				Config:   json.RawMessage(`{}`),
			},
		},
	})

	_, err := router.Complete(context.Background(), store.Tenant{ID: "tenant-a"}, Request{
		RequiredCapabilities: []Capability{CapabilityTools},
	})
	if errors.WhatKind(err) != KindCapabilityUnsupported {
		t.Fatalf("Complete() error = %v, want KindCapabilityUnsupported", err)
	}
	if message := errors.UserMsg(err); message != "The active LLM provider does not support the requested operation." {
		t.Fatalf("UserMsg() = %q", message)
	}
}
