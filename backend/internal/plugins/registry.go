// Package plugins provides a plugin registry for readers and writers.
package plugins

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

// AuthType describes how a reader plugin authenticates.
type AuthType string

const (
	// AuthTypeOAuth indicates the reader uses an OAuth2 flow (e.g. Gmail, ProtonMail).
	AuthTypeOAuth AuthType = "oauth"
	// AuthTypeConfig indicates the reader only requires local configuration (e.g. Thunderbird).
	AuthTypeConfig AuthType = "config"
)

// ConfigField describes a single user-provided configuration field for a plugin.
type ConfigField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"` // "text", "password", "path"
	Required bool   `json:"required"`
	Help     string `json:"help,omitempty"`
}

// ReaderPlugin defines the interface for transaction reader plugins.
type ReaderPlugin interface {
	// Name returns the plugin name (e.g., "gmail", "thunderbird").
	Name() string
	// Description returns a human-readable description.
	Description() string
	// RequiredScopes returns the OAuth scopes needed by this plugin.
	RequiredScopes() []string
	// AuthType returns how this reader authenticates.
	AuthType() AuthType
	// RequiresCredentialsUpload reports whether the user must upload an OAuth
	// client credentials file (e.g. client_secret.json for Gmail).
	RequiresCredentialsUpload() bool
	// ConfigSchema returns the configuration fields the user must provide.
	// For OAuth readers this is typically empty; for config-only readers
	// (e.g. Thunderbird) it describes the required fields.
	ConfigSchema() []ConfigField
	// NewReader creates a new reader instance with the given config.
	NewReader(
		httpClient *http.Client, cfg *config.Config, rules []api.Rule,
		labels api.Labels, stateManager *state.Manager, logger *slog.Logger,
	) (api.Reader, error)
}

// WriterPlugin defines the interface for transaction writer plugins.
type WriterPlugin interface {
	// Name returns the plugin name (e.g., "sheets", "postgres").
	Name() string
	// Description returns a human-readable description.
	Description() string
	// RequiredScopes returns the OAuth scopes needed by this plugin.
	RequiredScopes() []string
	// NewWriter creates a new writer instance with the given config.
	NewWriter(httpClient *http.Client, cfg *config.Config, logger *slog.Logger) (api.Writer, error)
}

// Registry manages available reader and writer plugins.
type Registry struct {
	readers map[string]ReaderPlugin
	writers map[string]WriterPlugin
}

// NewRegistry creates a new plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		readers: make(map[string]ReaderPlugin),
		writers: make(map[string]WriterPlugin),
	}
}

// RegisterReader registers a reader plugin.
func (r *Registry) RegisterReader(plugin ReaderPlugin) error {
	name := plugin.Name()
	if _, exists := r.readers[name]; exists {
		return fmt.Errorf("reader plugin %q already registered", name)
	}
	r.readers[name] = plugin
	return nil
}

// RegisterWriter registers a writer plugin.
func (r *Registry) RegisterWriter(plugin WriterPlugin) error {
	name := plugin.Name()
	if _, exists := r.writers[name]; exists {
		return fmt.Errorf("writer plugin %q already registered", name)
	}
	r.writers[name] = plugin
	return nil
}

// GetReader returns a reader plugin by name.
func (r *Registry) GetReader(name string) (ReaderPlugin, error) {
	plugin, exists := r.readers[name]
	if !exists {
		return nil, fmt.Errorf("reader plugin %q not found", name)
	}
	return plugin, nil
}

// GetWriter returns a writer plugin by name.
func (r *Registry) GetWriter(name string) (WriterPlugin, error) {
	plugin, exists := r.writers[name]
	if !exists {
		return nil, fmt.Errorf("writer plugin %q not found", name)
	}
	return plugin, nil
}

// ListReaders returns all registered reader plugins.
func (r *Registry) ListReaders() []ReaderPlugin {
	plugins := make([]ReaderPlugin, 0, len(r.readers))
	for _, plugin := range r.readers {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// ListWriters returns all registered writer plugins.
func (r *Registry) ListWriters() []WriterPlugin {
	plugins := make([]WriterPlugin, 0, len(r.writers))
	for _, plugin := range r.writers {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// GetAllScopes returns all OAuth scopes required by the given reader and writer names.
func (r *Registry) GetAllScopes(readerName, writerName string) ([]string, error) {
	reader, err := r.GetReader(readerName)
	if err != nil {
		return nil, err
	}

	writer, err := r.GetWriter(writerName)
	if err != nil {
		return nil, err
	}

	// Combine and deduplicate scopes
	scopeSet := make(map[string]struct{})
	for _, scope := range reader.RequiredScopes() {
		scopeSet[scope] = struct{}{}
	}
	for _, scope := range writer.RequiredScopes() {
		scopeSet[scope] = struct{}{}
	}

	scopes := make([]string, 0, len(scopeSet))
	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}

	return scopes, nil
}

// CreateReader creates a reader instance from a plugin.
func (r *Registry) CreateReader( //nolint:revive // argument count mirrors the ReaderPlugin interface
	name string, httpClient *http.Client, cfg *config.Config, rules []api.Rule,
	labels api.Labels, stateManager *state.Manager, logger *slog.Logger,
) (api.Reader, error) {
	plugin, err := r.GetReader(name)
	if err != nil {
		return nil, err
	}
	return plugin.NewReader(httpClient, cfg, rules, labels, stateManager, logger)
}

// CreateWriter creates a writer instance from a plugin.
func (r *Registry) CreateWriter(name string, httpClient *http.Client, cfg *config.Config, logger *slog.Logger) (api.Writer, error) {
	plugin, err := r.GetWriter(name)
	if err != nil {
		return nil, err
	}
	return plugin.NewWriter(httpClient, cfg, logger)
}
