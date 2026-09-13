// Package migrations embeds and applies Expensor's numbered SQL migrations.
package migrations

import (
	"context"
	"embed"
	"log/slog"

	migrate "github.com/golang-migrate/migrate/v4"
	pgdriver "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
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
		return errors.B.Op("postgres.migrations.run").KindInternal().Text("applying migrations").Err(err).Build()
	}
	return nil
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS expensor`); err != nil {
		return errors.B.Op("postgres.migrations.ensure_schema").KindInternal().Text("creating expensor schema").Err(err).Build()
	}
	return nil
}

func newMigrator(pool *pgxpool.Pool) (*migrate.Migrate, error) {
	db := stdlib.OpenDBFromPool(pool)
	source, err := iofs.New(FS, ".")
	if err != nil {
		_ = db.Close()
		return nil, errors.B.Op("postgres.migrations.new").KindInternal().Text("creating embedded migration source").Err(err).Build()
	}

	driver, err := pgdriver.WithInstance(db, &pgdriver.Config{
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		_ = db.Close()
		return nil, errors.B.Op("postgres.migrations.new").KindInternal().Text("creating postgres migration driver").Err(err).Build()
	}

	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		_ = db.Close()
		return nil, errors.B.Op("postgres.migrations.new").KindInternal().Text("initializing migrate").Err(err).Build()
	}
	return m, nil
}

func closeMigrator(m *migrate.Migrate) {
	_, _ = m.Close()
}
