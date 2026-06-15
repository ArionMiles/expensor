package migrations

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

	pool, err := pgxpool.New(ctx, dsn)
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

func TestRunUsesLegacySchemaMigrationsTable(t *testing.T) {
	ctx := context.Background()
	pool := newMigrationTestPool(t)

	if err := Run(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'legacy_schema_migrations'
		)
	`).Scan(&exists); err != nil {
		t.Fatalf("check table exists: %v", err)
	}
	if !exists {
		t.Fatal("legacy_schema_migrations table not created")
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM legacy_schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count legacy migrations: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least one recorded migration")
	}
}
