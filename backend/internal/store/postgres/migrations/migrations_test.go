package migrations_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ArionMiles/expensor/backend/internal/store/postgres"
	"github.com/ArionMiles/expensor/backend/internal/store/postgres/migrations"
)

func newMigrationTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("expensor_test"),
		tcpostgres.WithUsername("expensor"),
		tcpostgres.WithPassword("expensor"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = ctr.Terminate(ctx)
		t.Fatalf("connection string: %v", err)
	}

	poolCfg, err := postgres.ParsePoolConfig(dsn)
	if err != nil {
		_ = ctr.Terminate(ctx)
		t.Fatalf("parse pool config: %v", err)
	}
	poolCfg.MaxConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		_ = ctr.Terminate(ctx)
		t.Fatalf("open pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_ = ctr.Terminate(context.Background())
	})

	return pool
}

func TestRunUsesSchemaMigrationsInExpensor(t *testing.T) {
	ctx := context.Background()
	pool := newMigrationTestPool(t)

	if err := migrations.Run(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var expensorExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.schemata
			WHERE schema_name = 'expensor'
		)
	`).Scan(&expensorExists); err != nil {
		t.Fatalf("check expensor schema: %v", err)
	}
	if !expensorExists {
		t.Fatal("expensor schema not created")
	}

	var schemaMigrationsExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'expensor'
			  AND table_name = 'schema_migrations'
		)
	`).Scan(&schemaMigrationsExists); err != nil {
		t.Fatalf("check schema_migrations table: %v", err)
	}
	if !schemaMigrationsExists {
		t.Fatal("schema_migrations table not created in expensor")
	}

	var version int
	var dirty bool
	if err := pool.QueryRow(ctx, `SELECT version, dirty FROM expensor.schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatalf("read schema_migrations state: %v", err)
	}
	if dirty {
		t.Fatal("schema_migrations marked dirty after migration run")
	}
	if version != 9 {
		t.Fatalf("schema_migrations version = %d, want 9", version)
	}
}
