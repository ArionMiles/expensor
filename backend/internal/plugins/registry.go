// Package plugins provides a plugin registry for readers and writers.
package plugins

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

// ReaderPlugin defines the interface for transaction reader plugins.
type ReaderPlugin interface {
	// Name returns the plugin name (e.g., "gmail", "thunderbird").
	Name() string
	// Description returns a human-readable description.
	Description() string
	// RequiredScopes returns the OAuth scopes needed by this plugin.
	RequiredScopes() []string
	// ConfigSchema returns a JSON schema describing the plugin's configuration.
	ConfigSchema() map[string]any
	// NewReader creates a new reader instance with the given config.
	NewReader(httpClient *http.Client, config json.RawMessage, logger *slog.Logger) (api.Reader, error)
}

// WriterPlugin defines the interface for transaction writer plugins.
type WriterPlugin interface {
	// Name returns the plugin name (e.g., "sheets", "csv", "json").
	Name() string
	// Description returns a human-readable description.
	Description() string
	// RequiredScopes returns the OAuth scopes needed by this plugin.
	RequiredScopes() []string
	// ConfigSchema returns a JSON schema describing the plugin's configuration.
	ConfigSchema() map[string]any
	// NewWriter creates a new writer instance with the given config.
	NewWriter(httpClient *http.Client, config json.RawMessage, logger *slog.Logger) (api.Writer, error)
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
func (r *Registry) CreateReader(name string, httpClient *http.Client, config json.RawMessage, logger *slog.Logger) (api.Reader, error) {
	plugin, err := r.GetReader(name)
	if err != nil {
		return nil, err
	}
	return plugin.NewReader(httpClient, config, logger)
}

// CreateWriter creates a writer instance from a plugin.
func (r *Registry) CreateWriter(name string, httpClient *http.Client, config json.RawMessage, logger *slog.Logger) (api.Writer, error) {
	plugin, err := r.GetWriter(name)
	if err != nil {
		return nil, err
	}
	return plugin.NewWriter(httpClient, config, logger)
}
