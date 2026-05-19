// Package migration provides a sequential SQL migration runner with
// file-level tracking via a schema_migrations table.
package migration

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Run applies all pending *.sql files from migrationsFS in alphabetical order.
// Each file is executed exactly once; applied filenames are recorded in the
// schema_migrations table so restarts are safe.
//
// All migration files must be idempotent — Run may re-execute a file if the
// tracking record was lost, though this should not happen in normal operation.
func Run(ctx context.Context, pool *pgxpool.Pool, migrationsFS fs.FS, logger *slog.Logger) error {
	// Bootstrap the tracking table before anything else.
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
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

		content, err := fs.ReadFile(migrationsFS, filename)
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
