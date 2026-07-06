package llm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
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
	if !errors.Is(err, ErrNoProviderConfigured) {
		t.Fatalf("Complete() error = %v, want ErrNoProviderConfigured", err)
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
	if !errors.Is(err, ErrCapabilityUnsupported) {
		t.Fatalf("Complete() error = %v, want ErrCapabilityUnsupported", err)
	}
}
