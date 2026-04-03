// Package postgres provides a plugin wrapper for the PostgreSQL writer.
package postgres

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
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

// NewWriter creates a new PostgreSQL writer instance.
// Note: httpClient is ignored as PostgreSQL doesn't need OAuth.
func (p *Plugin) NewWriter(httpClient *http.Client, cfg *config.Config, logger *slog.Logger) (api.Writer, error) {
	logger.Debug("postgres writer config",
		"host", cfg.Postgres.Host,
		"port", cfg.Postgres.Port,
		"database", cfg.Postgres.Database,
		"user", cfg.Postgres.User,
		"sslmode", cfg.Postgres.SSLMode,
	)
	// Convert flush interval to duration
	flushInterval := time.Duration(cfg.Postgres.FlushInterval) * time.Second

	writerCfg := pgwriter.Config{
		Host:          cfg.Postgres.Host,
		Port:          cfg.Postgres.Port,
		Database:      cfg.Postgres.Database,
		User:          cfg.Postgres.User,
		Password:      cfg.Postgres.Password,
		SSLMode:       cfg.Postgres.SSLMode,
		BatchSize:     cfg.Postgres.BatchSize,
		FlushInterval: flushInterval,
		MaxPoolSize:   cfg.Postgres.MaxPoolSize,
	}

	return pgwriter.New(writerCfg, logger)
}
