// Package plugins provides a plugin registry for readers and writers.
package plugins

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

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

// AuthSpec describes how a reader plugin authenticates.
type AuthSpec struct {
	Type                      AuthType `json:"type"`
	RequiredScopes            []string `json:"required_scopes"`
	RequiresCredentialsUpload bool     `json:"requires_credentials_upload"`
}

// ReaderMetadata describes a reader plugin for catalog display and selection.
type ReaderMetadata struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Auth         AuthSpec        `json:"auth"`
	ConfigSchema []ConfigField   `json:"config_schema"`
	SetupGuide   json.RawMessage `json:"setup_guide,omitempty"`
}

// WriterMetadata describes a writer plugin for catalog display and selection.
type WriterMetadata struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	RequiredScopes []string `json:"required_scopes"`
}

// ReaderInput contains dependencies required to create a reader instance.
type ReaderInput struct {
	HTTPClient     *http.Client
	AppConfig      *config.App
	ReaderConfig   json.RawMessage
	Rules          []api.Rule
	Resolver       api.CategoryResolver
	StateManager   *state.Manager
	DiagnosticSink api.DiagnosticSink
	Logger         *slog.Logger
}

// WriterInput contains dependencies required to create a writer instance.
type WriterInput struct {
	HTTPClient *http.Client
	AppConfig  *config.App
	Logger     *slog.Logger
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
	Metadata() ReaderMetadata
	NewReader(input ReaderInput) (api.Reader, error)
}

// WriterPlugin defines the interface for transaction writer plugins.
type WriterPlugin interface {
	Metadata() WriterMetadata
	NewWriter(input WriterInput) (api.Writer, error)
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
	if isNilPlugin(plugin) {
		return fmt.Errorf("reader plugin is nil")
	}

	metadata := plugin.Metadata()
	name := metadata.Name
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("reader plugin name is required")
	}
	if len(metadata.SetupGuide) > 0 && !json.Valid(metadata.SetupGuide) {
		return fmt.Errorf("reader plugin %q setup guide must be valid JSON", name)
	}
	if _, exists := r.readers[name]; exists {
		return fmt.Errorf("reader plugin %q already registered", name)
	}
	r.readers[name] = plugin
	return nil
}

// RegisterWriter registers a writer plugin.
func (r *Registry) RegisterWriter(plugin WriterPlugin) error {
	if isNilPlugin(plugin) {
		return fmt.Errorf("writer plugin is nil")
	}

	name := plugin.Metadata().Name
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("writer plugin name is required")
	}
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
	for _, scope := range reader.Metadata().Auth.RequiredScopes {
		scopeSet[scope] = struct{}{}
	}
	for _, scope := range writer.Metadata().RequiredScopes {
		scopeSet[scope] = struct{}{}
	}

	scopes := make([]string, 0, len(scopeSet))
	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}

	return scopes, nil
}

func isNilPlugin(plugin any) bool {
	if plugin == nil {
		return true
	}

	value := reflect.ValueOf(plugin)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
