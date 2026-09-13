package llm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
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

// DataUseMode describes how a provider may use API request content.
type DataUseMode string

const (
	DataUseNoTrainingByDefault DataUseMode = "no_training_by_default"
	DataUseFreeTierImprovement DataUseMode = "free_tier_improvement"
)

// DataUseSpec links provider metadata to its current data-use policy.
type DataUseSpec struct {
	Mode      DataUseMode `json:"mode"`
	PolicyURL string      `json:"policy_url"`
}

// ModelOption describes one provider model preset exposed to setup UIs.
type ModelOption struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Quality     string `json:"quality"`
	Cost        string `json:"cost"`
	Description string `json:"description,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
}

// ProviderMetadata describes an LLM provider for catalog display and routing.
type ProviderMetadata struct {
	Name           string          `json:"name"`
	DisplayName    string          `json:"display_name,omitempty"`
	APIKeyURL      string          `json:"api_key_url,omitempty"`
	APIKeyLinkText string          `json:"api_key_link_text,omitempty"`
	DataUse        DataUseSpec     `json:"data_use"`
	Auth           AuthSpec        `json:"auth"`
	ConfigSchema   json.RawMessage `json:"config_schema,omitempty"`
	Capabilities   []Capability    `json:"capabilities"`
	ModelOptions   []ModelOption   `json:"model_options,omitempty"`
}

// ConfigStringDefault returns a string property's default from provider configuration metadata.
func ConfigStringDefault(schema json.RawMessage, propertyName string) (string, bool) {
	var document struct {
		Properties map[string]struct {
			Type    string          `json:"type"`
			Default json.RawMessage `json:"default"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema, &document); err != nil {
		return "", false
	}
	property, ok := document.Properties[propertyName]
	if !ok || property.Type != "string" {
		return "", false
	}
	var value string
	if err := json.Unmarshal(property.Default, &value); err != nil {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

// ValidateConfig checks persisted provider configuration against the fields declared by its catalog schema.
func ValidateConfig(schema, config json.RawMessage) error {
	const op = "llm.ValidateConfig"

	var document struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema, &document); err != nil {
		return errors.E(op, errors.Internal, "decoding provider configuration schema", err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(config, &values); err != nil {
		return errors.E(op, errors.InvalidInput, errors.User("Provider configuration must be a JSON object."), err)
	}
	if values == nil {
		return errors.E(op, errors.InvalidInput, errors.User("Provider configuration must be a JSON object."))
	}
	for name, value := range values {
		property, ok := document.Properties[name]
		if !ok {
			return errors.E(
				op,
				errors.InvalidInput,
				errors.User(fmt.Sprintf("Provider configuration field %q is not supported.", name)),
			)
		}
		if !configValueMatchesType(value, property.Type) {
			return errors.E(
				op,
				errors.InvalidInput,
				errors.User(fmt.Sprintf("Provider configuration field %q must be a %s.", name, property.Type)),
			)
		}
	}
	return nil
}

func configValueMatchesType(raw json.RawMessage, want string) bool {
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return false
	}
	switch want {
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number":
		_, ok := value.(json.Number)
		return ok
	case "integer":
		number, ok := value.(json.Number)
		return ok && !strings.ContainsAny(number.String(), ".eE")
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	default:
		return false
	}
}

// Provider defines an LLM provider and the client it can construct.
type Provider struct {
	Metadata  ProviderMetadata
	NewClient func(ClientConfig) (Client, error)
}

// RequireCapabilities returns an error if the provider lacks a required capability.
func (p Provider) RequireCapabilities(required ...Capability) error {
	const op = "llm.Provider.RequireCapabilities"

	if len(required) == 0 {
		return nil
	}
	available := make(map[Capability]struct{}, len(p.Metadata.Capabilities))
	for _, capability := range p.Metadata.Capabilities {
		available[capability] = struct{}{}
	}
	for _, capability := range required {
		if _, ok := available[capability]; !ok {
			return errors.E(
				op,
				KindCapabilityUnsupported,
				errors.User("The active LLM provider does not support the requested operation."),
				fmt.Sprintf("llm capability unsupported: %s", capability),
			)
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
	const op = "llm.Registry.RegisterProvider"

	name := strings.TrimSpace(provider.Metadata.Name)
	if name == "" {
		return errors.E(op, errors.InvalidInput, "llm provider name is required")
	}
	if provider.NewClient == nil {
		return errors.E(op, errors.InvalidInput, fmt.Sprintf("llm provider %q client factory is required", name))
	}
	if len(provider.Metadata.ConfigSchema) > 0 && !json.Valid(provider.Metadata.ConfigSchema) {
		return errors.E(op, errors.InvalidInput, fmt.Sprintf("llm provider %q config schema must be valid JSON", name))
	}
	for _, option := range provider.Metadata.ModelOptions {
		if strings.TrimSpace(option.ID) == "" {
			return errors.E(op, errors.InvalidInput, fmt.Sprintf("llm provider %q model option id is required", name))
		}
	}
	if _, exists := r.providers[name]; exists {
		return errors.E(op, errors.Conflict, fmt.Sprintf("llm provider %q already registered", name))
	}
	provider.Metadata.Name = name
	r.providers[name] = provider
	return nil
}

// GetProvider returns a provider by name.
func (r *Registry) GetProvider(name string) (Provider, error) {
	const op = "llm.Registry.GetProvider"

	provider, ok := r.providers[name]
	if !ok {
		return Provider{}, errors.E(op, errors.NotFound, fmt.Sprintf("llm provider %q not found", name))
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
