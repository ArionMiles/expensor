package migrations_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	migrate "github.com/golang-migrate/migrate/v4"
	pgdriver "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
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
	if version != 10 {
		t.Fatalf("schema_migrations version = %d, want 10", version)
	}
}

func TestDiagnosticTenantBackfillMigration(t *testing.T) {
	ctx := context.Background()
	pool := newMigrationTestPool(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := migrations.Run(ctx, pool, logger); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	m := newMigrationTestMigrator(t, pool)
	if err := m.Steps(-1); err != nil {
		t.Fatalf("Steps(-1) error = %v", err)
	}

	const (
		tenantID         = "00000000-0000-0000-0000-000000000001"
		customRuleID     = "00000000-0000-0000-0000-000000000011"
		predefinedRuleID = "00000000-0000-0000-0000-000000000012"
		customDiagID     = "00000000-0000-0000-0000-000000000021"
		predefinedDiagID = "00000000-0000-0000-0000-000000000022"
		unmatchedDiagID  = "00000000-0000-0000-0000-000000000023"
	)
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, display_name, role)
		VALUES ($1, 'diagnostic-backfill@example.com', 'Diagnostic Backfill', 'user')
	`, tenantID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO rules (id, tenant_id, name, amount_regex, merchant_regex, predefined)
		VALUES
			($1, $2, 'Custom diagnostic rule', 'amount', 'merchant', false),
			($3, NULL, 'Predefined diagnostic rule', 'amount', 'merchant', true)
	`, customRuleID, tenantID, predefinedRuleID); err != nil {
		t.Fatalf("insert rules: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO extraction_diagnostics (id, reader, message_id, rule_id, rule_name)
		VALUES
			($1, 'gmail', 'backfill-custom', $2, 'Custom diagnostic rule'),
			($3, 'gmail', 'backfill-predefined', $4, 'Predefined diagnostic rule'),
			($5, 'gmail', 'backfill-unmatched', NULL, '')
	`, customDiagID, customRuleID, predefinedDiagID, predefinedRuleID, unmatchedDiagID); err != nil {
		t.Fatalf("insert diagnostics: %v", err)
	}

	if err := m.Steps(1); err != nil {
		t.Fatalf("Steps(1) error = %v", err)
	}

	assertDiagnosticTenant(ctx, t, pool, customDiagID, tenantID)
	assertDiagnosticTenant(ctx, t, pool, predefinedDiagID, "")
	assertDiagnosticTenant(ctx, t, pool, unmatchedDiagID, "")
}

func newMigrationTestMigrator(t *testing.T, pool *pgxpool.Pool) *migrate.Migrate {
	t.Helper()

	db := stdlib.OpenDBFromPool(pool)
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		_ = db.Close()
		t.Fatalf("create migration source: %v", err)
	}
	driver, err := pgdriver.WithInstance(db, &pgdriver.Config{MigrationsTable: "schema_migrations"})
	if err != nil {
		_ = db.Close()
		t.Fatalf("create migration driver: %v", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		_ = db.Close()
		t.Fatalf("create migrator: %v", err)
	}
	t.Cleanup(func() {
		_, _ = m.Close()
	})
	return m
}

func assertDiagnosticTenant(ctx context.Context, t *testing.T, pool *pgxpool.Pool, diagnosticID, wantTenant string) {
	t.Helper()

	var gotTenant *string
	if err := pool.QueryRow(ctx, `SELECT tenant_id::text FROM extraction_diagnostics WHERE id = $1`, diagnosticID).Scan(&gotTenant); err != nil {
		t.Fatalf("get diagnostic %s tenant: %v", diagnosticID, err)
	}
	if wantTenant == "" {
		if gotTenant != nil {
			t.Fatalf("diagnostic %s tenant = %q, want NULL", diagnosticID, *gotTenant)
		}
		return
	}
	if gotTenant == nil || *gotTenant != wantTenant {
		t.Fatalf("diagnostic %s tenant = %v, want %q", diagnosticID, gotTenant, wantTenant)
	}
}
