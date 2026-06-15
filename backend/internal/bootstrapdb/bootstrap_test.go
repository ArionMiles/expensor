package bootstrapdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ArionMiles/expensor/backend/internal/dbconn"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

func newBootstrapTestPool(t *testing.T) (*pgxpool.Pool, config.Postgres, func()) {
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

	poolCfg, err := dbconn.ParseConfig(dsn)
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

	mappedPort, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		_ = ctr.Terminate(ctx)
		t.Fatalf("mapped port: %v", err)
	}

	cfg := config.Postgres{
		Host:     "localhost",
		Port:     int(mappedPort.Num()),
		Database: "expensor_test",
		User:     "expensor",
		Password: "expensor",
		SSLMode:  "disable",
	}

	cleanup := func() {
		pool.Close()
		_ = ctr.Terminate(context.Background())
	}
	return pool, cfg, cleanup
}

func TestPrepareFreshDatabaseBootstrapsExpensor(t *testing.T) {
	pool, cfg, cleanup := newBootstrapTestPool(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := Prepare(context.Background(), cfg, logger); err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	assertSchemaExists := func(schema string) {
		t.Helper()
		var exists bool
		if err := pool.QueryRow(context.Background(), `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.schemata
				WHERE schema_name = $1
			)
		`, schema).Scan(&exists); err != nil {
			t.Fatalf("check schema %s: %v", schema, err)
		}
		if !exists {
			t.Fatalf("schema %s not created", schema)
		}
	}

	assertSchemaExists("expensor")

	var version int
	var dirty bool
	if err := pool.QueryRow(context.Background(), `SELECT version, dirty FROM expensor.schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	if dirty {
		t.Fatal("schema_migrations marked dirty after fresh bootstrap")
	}
	if version != 3 {
		t.Fatalf("version = %d, want 3", version)
	}
}

func TestPrepareExistingDatabaseCopiesPublicData(t *testing.T) {
	pool, cfg, cleanup := newBootstrapTestPool(t)
	defer cleanup()

	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		CREATE TABLE public.transactions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			message_id TEXT NOT NULL UNIQUE,
			amount NUMERIC(19,4) NOT NULL,
			currency TEXT NOT NULL DEFAULT 'INR',
			source TEXT NOT NULL DEFAULT '',
			timestamp TIMESTAMPTZ NOT NULL,
			merchant_info TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)
	`); err != nil {
		t.Fatalf("create public.transactions: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		CREATE TABLE public.reader_runtime (
			reader TEXT PRIMARY KEY,
			config JSONB,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)
	`); err != nil {
		t.Fatalf("create public.reader_runtime: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.transactions (message_id, amount, currency, source, timestamp, merchant_info)
		VALUES ('msg-1', 123.45, 'INR', 'gmail', NOW(), 'Test Merchant')
	`); err != nil {
		t.Fatalf("seed public.transactions: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.reader_runtime (reader, config)
		VALUES ('gmail', '{"mailboxes":["Inbox"]}')
	`); err != nil {
		t.Fatalf("seed public.reader_runtime: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := Prepare(ctx, cfg, logger); err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	for _, table := range []string{"transactions", "reader_runtime"} {
		var publicCount, expensorCount int
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM public.%s`, table)).Scan(&publicCount); err != nil {
			t.Fatalf("count public.%s: %v", table, err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM expensor.%s`, table)).Scan(&expensorCount); err != nil {
			t.Fatalf("count expensor.%s: %v", table, err)
		}
		if publicCount != expensorCount {
			t.Fatalf("%s count mismatch: public=%d expensor=%d", table, publicCount, expensorCount)
		}
	}

	var version int
	var dirty bool
	if err := pool.QueryRow(ctx, `SELECT version, dirty FROM expensor.schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	if dirty {
		t.Fatal("schema_migrations marked dirty after bridge")
	}
	if version != 3 {
		t.Fatalf("version = %d, want 3", version)
	}
}

func TestPrepareBridgeFailureLeavesPublicUntouched(t *testing.T) {
	pool, cfg, cleanup := newBootstrapTestPool(t)
	defer cleanup()

	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		CREATE TABLE public.transactions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			message_id TEXT NOT NULL UNIQUE,
			amount NUMERIC(19,4) NOT NULL,
			currency TEXT NOT NULL DEFAULT 'INR',
			source TEXT NOT NULL DEFAULT '',
			timestamp TIMESTAMPTZ NOT NULL,
			merchant_info TEXT NOT NULL
		)
	`); err != nil {
		t.Fatalf("create public.transactions: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.transactions (message_id, amount, currency, source, timestamp, merchant_info)
		VALUES ('msg-2', 99.99, 'INR', 'gmail', NOW(), 'Rollback Merchant')
	`); err != nil {
		t.Fatalf("seed public.transactions: %v", err)
	}

	wantErr := errors.New("forced validation failure")
	err := PrepareWithHooks(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), Hooks{
		ValidateCopy: func(context.Context) error { return wantErr },
	})
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("PrepareWithHooks error = %v, want %v", err, wantErr)
	}

	var publicCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.transactions`).Scan(&publicCount); err != nil {
		t.Fatalf("count public.transactions: %v", err)
	}
	if publicCount != 1 {
		t.Fatalf("public.transactions count = %d, want 1", publicCount)
	}

	var expensorExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'expensor'
			  AND table_name = 'transactions'
		)
	`).Scan(&expensorExists); err != nil {
		t.Fatalf("check expensor.transactions: %v", err)
	}
	if expensorExists {
		t.Fatal("expensor.transactions should not exist after a failed bridge")
	}
}
