// Package json provides a plugin wrapper for the JSON writer.
package json

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	jsonwriter "github.com/ArionMiles/expensor/backend/pkg/writer/json"
)

// Plugin implements the WriterPlugin interface for JSON files.
type Plugin struct{}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "json"
}

// Description returns a human-readable description.
func (p *Plugin) Description() string {
	return "Write expense transactions to JSON file"
}

// RequiredScopes returns the OAuth scopes needed by this plugin.
func (p *Plugin) RequiredScopes() []string {
	// JSON writer doesn't need OAuth scopes
	return nil
}

// ConfigSchema returns a JSON schema describing the plugin's configuration.
func (p *Plugin) ConfigSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath": map[string]any{
				"type":        "string",
				"description": "Path to the JSON output file",
			},
			"batchSize": map[string]any{
				"type":        "integer",
				"description": "Number of transactions to buffer before writing (default: 10)",
				"default":     10,
			},
			"flushInterval": map[string]any{
				"type":        "integer",
				"description": "Interval in seconds between automatic flushes (default: 30)",
				"default":     30,
			},
		},
		"required": []string{"filePath"},
	}
}

// Config represents the JSON writer configuration.
type Config struct {
	FilePath      string `json:"filePath"`
	BatchSize     int    `json:"batchSize,omitempty"`
	FlushInterval int    `json:"flushInterval,omitempty"` // in seconds
}

// NewWriter creates a new JSON writer instance.
func (p *Plugin) NewWriter(httpClient *http.Client, configData json.RawMessage, logger *slog.Logger) (api.Writer, error) {
	var cfg Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling json config: %w", err)
	}

	// Validate required fields
	if cfg.FilePath == "" {
		return nil, fmt.Errorf("filePath is required")
	}

	writerCfg := jsonwriter.Config{
		FilePath:      cfg.FilePath,
		BatchSize:     cfg.BatchSize,
		FlushInterval: cfg.FlushInterval,
	}

	return jsonwriter.New(writerCfg, logger)
}
