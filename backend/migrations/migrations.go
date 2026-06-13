// Package migrations embeds and applies Expensor's numbered SQL migrations.
package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FS contains every *.sql file in this directory.
//
//go:embed *.sql
var FS embed.FS

// Run applies all pending SQL migrations in filename order.
func Run(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	// Bootstrap the tracking table before anything else.
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, filename := range files {
		var applied bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`, filename,
		).Scan(&applied); err != nil {
			return fmt.Errorf("checking migration %s: %w", filename, err)
		}
		if applied {
			logger.Debug("migration already applied", "file", filename)
			continue
		}

		content, err := fs.ReadFile(FS, filename)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", filename, err)
		}

		logger.Info("applying migration", "file", filename)
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("applying migration %s: %w", filename, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING`, filename,
		); err != nil {
			return fmt.Errorf("recording migration %s: %w", filename, err)
		}
		logger.Info("migration applied", "file", filename)
	}
	return nil
}
