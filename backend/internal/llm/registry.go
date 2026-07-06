package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrProviderNotFound          = errors.New("llm provider not found")
	ErrProviderAlreadyRegistered = errors.New("llm provider already registered")
	ErrCapabilityUnsupported     = errors.New("llm capability unsupported")
)

// AuthType describes how an LLM provider authenticates.
type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeOAuth  AuthType = "oauth"
)

// AuthSpec describes provider authentication requirements.
type AuthSpec struct {
	Type           AuthType `json:"type"`
	Required       bool     `json:"required"`
	RequiredScopes []string `json:"required_scopes,omitempty"`
}

// ProviderMetadata describes an LLM provider for catalog display and routing.
type ProviderMetadata struct {
	Name         string          `json:"name"`
	DisplayName  string          `json:"display_name,omitempty"`
	Description  string          `json:"description,omitempty"`
	Auth         AuthSpec        `json:"auth"`
	ConfigSchema json.RawMessage `json:"config_schema,omitempty"`
	Capabilities []Capability    `json:"capabilities"`
}

// Provider defines an LLM provider and the client it can construct.
type Provider struct {
	Metadata  ProviderMetadata
	NewClient func(ClientConfig) (Client, error)
}

// RequireCapabilities returns an error if the provider lacks a required capability.
func (p Provider) RequireCapabilities(required ...Capability) error {
	if len(required) == 0 {
		return nil
	}
	available := make(map[Capability]struct{}, len(p.Metadata.Capabilities))
	for _, capability := range p.Metadata.Capabilities {
		available[capability] = struct{}{}
	}
	for _, capability := range required {
		if _, ok := available[capability]; !ok {
			return fmt.Errorf("%w: %s", ErrCapabilityUnsupported, capability)
		}
	}
	return nil
}

// Registry manages available LLM providers.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates an empty LLM provider registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// RegisterProvider registers an LLM provider.
func (r *Registry) RegisterProvider(provider Provider) error {
	name := strings.TrimSpace(provider.Metadata.Name)
	if name == "" {
		return errors.New("llm provider name is required")
	}
	if provider.NewClient == nil {
		return fmt.Errorf("llm provider %q client factory is required", name)
	}
	if len(provider.Metadata.ConfigSchema) > 0 && !json.Valid(provider.Metadata.ConfigSchema) {
		return fmt.Errorf("llm provider %q config schema must be valid JSON", name)
	}
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("%w: %s", ErrProviderAlreadyRegistered, name)
	}
	provider.Metadata.Name = name
	r.providers[name] = provider
	return nil
}

// GetProvider returns a provider by name.
func (r *Registry) GetProvider(name string) (Provider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return Provider{}, fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}
	return provider, nil
}

// ListProviders returns all registered providers sorted by name.
func (r *Registry) ListProviders() []Provider {
	providers := make([]Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		providers = append(providers, provider)
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Metadata.Name < providers[j].Metadata.Name
	})
	return providers
}
