// Package migrations embeds and applies Expensor's numbered SQL migrations.
package migrations

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strconv"
	"strings"

	migrate "github.com/golang-migrate/migrate/v4"
	pgdriver "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// FS contains every migration file in this directory.
//
//go:embed *.up.sql *.down.sql
var FS embed.FS

// Run applies all pending SQL migrations in version order.
func Run(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	if err := ensureSchema(ctx, pool); err != nil {
		return err
	}

	m, err := newMigrator(pool)
	if err != nil {
		return err
	}
	defer closeMigrator(m)

	logger.Debug("running embedded migrations")
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}

// Baseline marks the current database as already having the latest embedded
// schema version. It is used by the startup bridge after copying legacy data
// into the new schema.
func Baseline(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	if err := ensureSchema(ctx, pool); err != nil {
		return err
	}

	m, err := newMigrator(pool)
	if err != nil {
		return err
	}
	defer closeMigrator(m)

	v, err := LatestVersion()
	if err != nil {
		return err
	}
	logger.Info("baselining embedded migrations", "version", v)
	if err := m.Force(v); err != nil {
		return fmt.Errorf("forcing migration version %d: %w", v, err)
	}
	return nil
}

// LatestVersion returns the highest migration version embedded in FS.
func LatestVersion() (int, error) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		return 0, fmt.Errorf("reading migration directory: %w", err)
	}

	latest := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return 0, err
		}
		if version > latest {
			latest = version
		}
	}
	if latest == 0 {
		return 0, errors.New("no embedded up migrations found")
	}
	return latest, nil
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS expensor`); err != nil {
		return fmt.Errorf("creating expensor schema: %w", err)
	}
	return nil
}

func newMigrator(pool *pgxpool.Pool) (*migrate.Migrate, error) {
	db := stdlib.OpenDBFromPool(pool)
	source, err := iofs.New(FS, ".")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating embedded migration source: %w", err)
	}

	driver, err := pgdriver.WithInstance(db, &pgdriver.Config{
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating postgres migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing migrate: %w", err)
	}
	return m, nil
}

func closeMigrator(m *migrate.Migrate) {
	_, _ = m.Close()
}

func migrationVersion(name string) (int, error) {
	base := strings.TrimSuffix(name, ".up.sql")
	prefix, _, ok := strings.Cut(base, "_")
	if !ok {
		return 0, fmt.Errorf("migration file %q does not have a numeric prefix", name)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("parsing migration version from %q: %w", name, err)
	}
	return version, nil
}
