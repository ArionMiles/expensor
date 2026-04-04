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
	Key       string `json:"name"` // serialized as "name" for frontend compatibility
	Label     string `json:"label"`
	Type      string `json:"type"` // "text", "password", "path", "thunderbird-profile", "thunderbird-mailboxes"
	Required  bool   `json:"required"`
	Help      string `json:"help,omitempty"`
	DependsOn string `json:"depends_on,omitempty"`
}

// GuideProvider is an optional interface for reader plugins that provide setup guides.
type GuideProvider interface {
	SetupGuide() []byte
}

// ConfigApplier is an optional interface for reader plugins whose settings are
// persisted via the web UI (POST /api/readers/{name}/config) rather than env vars.
// Implement this to map the raw JSON config map onto config.Config before the daemon starts.
type ConfigApplier interface {
	ApplyConfig(cfg *config.Config, raw map[string]any)
}

// ReaderGuide is the structured setup guide for a reader plugin.
type ReaderGuide struct {
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
