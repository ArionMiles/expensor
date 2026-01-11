// Package sheets provides a plugin wrapper for the Google Sheets writer.
package sheets

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	sheetsapi "google.golang.org/api/sheets/v4"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	sheetswriter "github.com/ArionMiles/expensor/backend/pkg/writer/sheets"
)

// Plugin implements the WriterPlugin interface for Google Sheets.
type Plugin struct{}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "sheets"
}

// Description returns a human-readable description.
func (p *Plugin) Description() string {
	return "Write expense transactions to Google Sheets"
}

// RequiredScopes returns the OAuth scopes needed by this plugin.
func (p *Plugin) RequiredScopes() []string {
	return []string{
		sheetsapi.SpreadsheetsScope,
	}
}

// ConfigSchema returns a JSON schema describing the plugin's configuration.
func (p *Plugin) ConfigSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sheetTitle": map[string]any{
				"type":        "string",
				"description": "Title for a new spreadsheet (used if sheetId is not provided)",
			},
			"sheetId": map[string]any{
				"type":        "string",
				"description": "ID of an existing spreadsheet to use",
			},
			"sheetName": map[string]any{
				"type":        "string",
				"description": "Name of the sheet/tab within the spreadsheet",
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
		"required": []string{"sheetName"},
	}
}

// Config represents the Sheets writer configuration.
type Config struct {
	SheetTitle    string `json:"sheetTitle,omitempty"`
	SheetID       string `json:"sheetId,omitempty"`
	SheetName     string `json:"sheetName"`
	BatchSize     int    `json:"batchSize,omitempty"`
	FlushInterval int    `json:"flushInterval,omitempty"` // in seconds
}

// NewWriter creates a new Sheets writer instance.
func (p *Plugin) NewWriter(httpClient *http.Client, configData json.RawMessage, logger *slog.Logger) (api.Writer, error) {
	var cfg Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling sheets config: %w", err)
	}

	// Validate required fields
	if cfg.SheetName == "" {
		return nil, fmt.Errorf("sheetName is required")
	}
	if cfg.SheetID == "" && cfg.SheetTitle == "" {
		return nil, fmt.Errorf("either sheetId or sheetTitle is required")
	}

	// Convert flush interval to duration
	flushInterval := time.Duration(cfg.FlushInterval) * time.Second

	writerCfg := sheetswriter.Config{
		SheetTitle:    cfg.SheetTitle,
		SheetID:       cfg.SheetID,
		SheetName:     cfg.SheetName,
		BatchSize:     cfg.BatchSize,
		FlushInterval: flushInterval,
	}

	return sheetswriter.New(httpClient, writerCfg, logger)
}
