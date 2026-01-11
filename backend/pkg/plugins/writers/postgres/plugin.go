// Package postgres provides a plugin wrapper for the PostgreSQL writer.
package postgres

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	pgwriter "github.com/ArionMiles/expensor/backend/pkg/writer/postgres"
)

// Plugin implements the WriterPlugin interface for PostgreSQL.
type Plugin struct{}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "postgres"
}

// Description returns a human-readable description.
func (p *Plugin) Description() string {
	return "Write expense transactions to PostgreSQL database with multi-currency support"
}

// RequiredScopes returns the OAuth scopes needed by this plugin.
// PostgreSQL writer doesn't require OAuth scopes.
func (p *Plugin) RequiredScopes() []string {
	return []string{}
}

// ConfigSchema returns a JSON schema describing the plugin's configuration.
func (p *Plugin) ConfigSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host": map[string]any{
				"type":        "string",
				"description": "PostgreSQL host address",
				"default":     "localhost",
			},
			"port": map[string]any{
				"type":        "integer",
				"description": "PostgreSQL port",
				"default":     5432,
			},
			"database": map[string]any{
				"type":        "string",
				"description": "Database name",
				"default":     "expensor",
			},
			"user": map[string]any{
				"type":        "string",
				"description": "Database user",
			},
			"password": map[string]any{
				"type":        "string",
				"description": "Database password",
			},
			"sslmode": map[string]any{
				"type":        "string",
				"description": "SSL mode (disable, require, verify-ca, verify-full)",
				"default":     "disable",
				"enum":        []string{"disable", "require", "verify-ca", "verify-full"},
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
			"maxPoolSize": map[string]any{
				"type":        "integer",
				"description": "Maximum number of connections in the pool (default: 10)",
				"default":     10,
			},
		},
		"required": []string{"host", "database", "user", "password"},
	}
}

// Config represents the PostgreSQL writer configuration.
type Config struct {
	Host          string `json:"host"`
	Port          int    `json:"port,omitempty"`
	Database      string `json:"database"`
	User          string `json:"user"`
	Password      string `json:"password"`
	SSLMode       string `json:"sslmode,omitempty"`
	BatchSize     int    `json:"batchSize,omitempty"`
	FlushInterval int    `json:"flushInterval,omitempty"` // in seconds
	MaxPoolSize   int    `json:"maxPoolSize,omitempty"`
}

// NewWriter creates a new PostgreSQL writer instance.
// Note: httpClient is ignored as PostgreSQL doesn't need OAuth.
func (p *Plugin) NewWriter(httpClient *http.Client, configData json.RawMessage, logger *slog.Logger) (api.Writer, error) {
	var cfg Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling postgres config: %w", err)
	}

	// Validate required fields
	if cfg.Host == "" {
		return nil, fmt.Errorf("host is required")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("database is required")
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("user is required")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("password is required")
	}

	// Convert flush interval to duration
	flushInterval := time.Duration(cfg.FlushInterval) * time.Second

	writerCfg := pgwriter.Config{
		Host:          cfg.Host,
		Port:          cfg.Port,
		Database:      cfg.Database,
		User:          cfg.User,
		Password:      cfg.Password,
		SSLMode:       cfg.SSLMode,
		BatchSize:     cfg.BatchSize,
		FlushInterval: flushInterval,
		MaxPoolSize:   cfg.MaxPoolSize,
	}

	return pgwriter.New(writerCfg, logger)
}
