// Package postgres provides a plugin wrapper for the PostgreSQL writer.
package postgres

import (
	"log/slog"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	pgwriter "github.com/ArionMiles/expensor/backend/pkg/writer/postgres"
)

// Plugin implements the WriterPlugin interface for PostgreSQL.
type Plugin struct{}

// Metadata returns catalog metadata for the PostgreSQL writer plugin.
func (p *Plugin) Metadata() plugins.WriterMetadata {
	return plugins.WriterMetadata{
		Name:           "postgres",
		Description:    "Write expense transactions to PostgreSQL database with multi-currency support",
		RequiredScopes: []string{},
	}
}

// NewWriter creates a new PostgreSQL writer instance.
func (p *Plugin) NewWriter(input plugins.WriterInput) (api.Writer, error) {
	cfg := input.AppConfig
	if cfg == nil {
		cfg = &config.App{}
	}
	logger := input.Logger
	if logger == nil {
		logger = slog.Default()
	}
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
