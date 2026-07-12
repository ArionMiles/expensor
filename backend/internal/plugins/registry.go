// Package plugins provides a provider registry for email-backed integrations.
package plugins

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ArionMiles/expensor/backend/internal/state"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// AuthType describes how a provider authenticates.
type AuthType string

const (
	// AuthTypeOAuth indicates the provider uses an OAuth2 flow (e.g. Gmail, ProtonMail).
	AuthTypeOAuth AuthType = "oauth"
	// AuthTypeConfig indicates the provider only requires local configuration (e.g. Thunderbird).
	AuthTypeConfig AuthType = "config"
)

// ConfigField describes a single user-provided configuration field for a plugin.
type ConfigField struct {
	Key       string `json:"name"` // serialized as "name" for frontend compatibility
	Label     string `json:"label"`
	Type      string `json:"type"` // "text", "password", "path", "thunderbird-profile", "thunderbird-mailboxes"
	Required  bool   `json:"required"`
	Help      string `json:"help,omitempty"`
	DependsOn string `json:"depends_on,omitempty"`
}

// AuthSpec describes how a provider authenticates.
type AuthSpec struct {
	Type                      AuthType `json:"type"`
	RequiredScopes            []string `json:"required_scopes"`
	RequiresCredentialsUpload bool     `json:"requires_credentials_upload"`
}

// ProviderMetadata describes a provider for catalog display and selection.
type ProviderMetadata struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Auth         AuthSpec        `json:"auth"`
	ConfigSchema []ConfigField   `json:"config_schema"`
	SetupGuide   json.RawMessage `json:"setup_guide,omitempty"`
}

// ProviderInput contains dependencies required to create provider capabilities.
type ProviderInput struct {
	HTTPClient     *http.Client
	AppConfig      *config.App
	ReaderConfig   json.RawMessage
	Rules          []api.Rule
	Resolver       api.CategoryResolver
	StateManager   *state.Manager
	DiagnosticSink api.DiagnosticSink
	Logger         *slog.Logger
}

// ProviderGuide is the structured setup guide for a provider.
type ProviderGuide struct {
	Sections []GuideSection `json:"sections"`
	Notes    []GuideNote    `json:"notes,omitempty"`
}

// GuideSection is a titled group of steps in the setup guide.
type GuideSection struct {
	Title string      `json:"title"`
	Steps []GuideStep `json:"steps"`
	Link  *GuideLink  `json:"link,omitempty"`
}

// GuideStep is a single step in a guide section, with optional nested sub-steps.
type GuideStep struct {
	Text     string   `json:"text"`
	SubSteps []string `json:"sub_steps,omitempty"`
}

// GuideLink is an optional external link attached to a guide section.
type GuideLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// GuideNote is a color-coded callout displayed below the guide sections.
// Type: "info" (blue), "warning" (amber), "tip" (green), "docker" (purple).
type GuideNote struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Provider defines an email provider and the capabilities it can construct.
type Provider struct {
	Metadata ProviderMetadata

	NewReader        func(ProviderInput) (api.Reader, error)
	NewEmailSearcher func(ProviderInput) (api.EmailSearcher, error)
}

// Registry manages available providers.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// RegisterProvider registers a provider.
func (r *Registry) RegisterProvider(provider Provider) error {
	name := provider.Metadata.Name
	if strings.TrimSpace(name) == "" {
		return errors.E(errors.InvalidInput, "provider name is required")
	}
	if provider.NewReader == nil {
		return errors.E(errors.InvalidInput, fmt.Sprintf("provider %q reader factory is required", name))
	}
	if provider.NewEmailSearcher == nil {
		return errors.E(errors.InvalidInput, fmt.Sprintf("provider %q email searcher factory is required", name))
	}
	if len(provider.Metadata.SetupGuide) > 0 && !json.Valid(provider.Metadata.SetupGuide) {
		return errors.E(errors.InvalidInput, fmt.Sprintf("provider %q setup guide must be valid JSON", name))
	}
	if _, exists := r.providers[name]; exists {
		return errors.E(errors.Conflict, fmt.Sprintf("provider %q already registered", name))
	}
	r.providers[name] = provider
	return nil
}

// GetProvider returns a provider by name.
func (r *Registry) GetProvider(name string) (Provider, error) {
	provider, exists := r.providers[name]
	if !exists {
		return Provider{}, errors.E(errors.NotFound, fmt.Sprintf("provider %q not found", name))
	}
	return provider, nil
}

// ListProviders returns all registered providers.
func (r *Registry) ListProviders() []Provider {
	providers := make([]Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		providers = append(providers, provider)
	}
	return providers
}

// GetAllScopes returns all OAuth scopes required by the given provider.
func (r *Registry) GetAllScopes(providerName string) ([]string, error) {
	provider, err := r.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	scopeSet := make(map[string]struct{})
	for _, scope := range provider.Metadata.Auth.RequiredScopes {
		scopeSet[scope] = struct{}{}
	}

	scopes := make([]string, 0, len(scopeSet))
	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}

	return scopes, nil
}
