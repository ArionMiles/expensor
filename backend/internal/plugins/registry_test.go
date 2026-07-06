package plugins

import (
	"context"
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

type mockReader struct{}

func (m *mockReader) Read(context.Context, chan<- *api.TransactionDetails, <-chan string) error {
	return nil
}

type mockEmailSearcher struct{}

func (m *mockEmailSearcher) Search(context.Context, api.EmailSearchQuery) ([]api.EmailSearchResult, error) {
	return nil, nil
}

func testProvider(name string) Provider {
	return Provider{
		Metadata: ProviderMetadata{
			Name:        name,
			Description: "Test provider",
			Auth: AuthSpec{
				Type: AuthTypeOAuth,
			},
		},
		NewReader: func(ProviderInput) (api.Reader, error) {
			return &mockReader{}, nil
		},
		NewEmailSearcher: func(ProviderInput) (api.EmailSearcher, error) {
			return &mockEmailSearcher{}, nil
		},
	}
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if registry.providers == nil {
		t.Error("expected providers map to be initialized")
	}
	if len(registry.providers) != 0 {
		t.Errorf("expected empty providers map, got %d entries", len(registry.providers))
	}
}

func TestRegisterProvider(t *testing.T) {
	registry := NewRegistry()
	provider := testProvider("test-provider")

	if err := registry.RegisterProvider(provider); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	got, err := registry.GetProvider("test-provider")
	if err != nil {
		t.Fatalf("GetProvider() error = %v", err)
	}
	if got.Metadata.Name != provider.Metadata.Name {
		t.Errorf("provider name = %q, want %q", got.Metadata.Name, provider.Metadata.Name)
	}
}

func TestRegisterProvider_Duplicate(t *testing.T) {
	registry := NewRegistry()
	provider := testProvider("test-provider")

	if err := registry.RegisterProvider(provider); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	err := registry.RegisterProvider(provider)
	if err == nil {
		t.Fatal("expected error for duplicate registration, got nil")
	}
	expectedErr := `provider "test-provider" already registered`
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestRegisterProvider_Validation(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		wantErr  string
	}{
		{
			name:     "blank name",
			provider: testProvider(" \t\n"),
			wantErr:  "provider name is required",
		},
		{
			name: "missing reader factory",
			provider: func() Provider {
				provider := testProvider("test-provider")
				provider.NewReader = nil
				return provider
			}(),
			wantErr: `provider "test-provider" reader factory is required`,
		},
		{
			name: "missing email searcher factory",
			provider: func() Provider {
				provider := testProvider("test-provider")
				provider.NewEmailSearcher = nil
				return provider
			}(),
			wantErr: `provider "test-provider" email searcher factory is required`,
		},
		{
			name: "invalid setup guide",
			provider: func() Provider {
				provider := testProvider("test-provider")
				provider.Metadata.SetupGuide = []byte("{invalid")
				return provider
			}(),
			wantErr: `provider "test-provider" setup guide must be valid JSON`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewRegistry().RegisterProvider(tt.provider)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGetProvider(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterProvider(testProvider("test-provider")); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	got, err := registry.GetProvider("test-provider")
	if err != nil {
		t.Fatalf("GetProvider() error = %v", err)
	}
	if got.Metadata.Name != "test-provider" {
		t.Errorf("provider name = %q, want test-provider", got.Metadata.Name)
	}

	_, err = registry.GetProvider("unknown")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	expectedErr := `provider "unknown" not found`
	if err.Error() != expectedErr {
		t.Errorf("error = %q, want %q", err.Error(), expectedErr)
	}
}

func TestListProviders(t *testing.T) {
	registry := NewRegistry()

	if providers := registry.ListProviders(); len(providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(providers))
	}
	if err := registry.RegisterProvider(testProvider("provider1")); err != nil {
		t.Fatalf("RegisterProvider(provider1) error = %v", err)
	}
	if err := registry.RegisterProvider(testProvider("provider2")); err != nil {
		t.Fatalf("RegisterProvider(provider2) error = %v", err)
	}

	providers := registry.ListProviders()
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}

	names := make(map[string]bool)
	for _, provider := range providers {
		names[provider.Metadata.Name] = true
	}
	if !names["provider1"] {
		t.Error("provider1 not found in list")
	}
	if !names["provider2"] {
		t.Error("provider2 not found in list")
	}
}

func TestGetAllScopes(t *testing.T) {
	registry := NewRegistry()
	provider := testProvider("test-provider")
	provider.Metadata.Auth.RequiredScopes = []string{"scope1", "scope2"}
	if err := registry.RegisterProvider(provider); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	scopes, err := registry.GetAllScopes("test-provider")
	if err != nil {
		t.Fatalf("GetAllScopes() error = %v", err)
	}

	scopeMap := make(map[string]bool)
	for _, scope := range scopes {
		scopeMap[scope] = true
	}
	for _, scope := range []string{"scope1", "scope2"} {
		if !scopeMap[scope] {
			t.Errorf("expected scope %q not found", scope)
		}
	}
}

func TestGetAllScopes_ProviderNotFound(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.GetAllScopes("unknown-provider")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}
