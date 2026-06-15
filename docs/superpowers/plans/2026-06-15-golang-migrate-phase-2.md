# Golang-Migrate Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the backend to `golang-migrate` and automatically cut existing databases over from `public` into a new `expensor` schema at startup.

**Architecture:** Add a small shared pgx pool helper so every backend connection uses `search_path=expensor,public`. Build a dedicated startup bootstrap package that detects whether `expensor` already exists, runs the one-time copy-and-baseline bridge when needed, and then hands off to a `golang-migrate` runner that owns `expensor.schema_migrations`. Keep the bridge isolated so phase 3 can delete it cleanly.

**Tech Stack:** Go, PostgreSQL, pgx, `golang-migrate/migrate`, embedded SQL migrations, Task

---

### Task 1: Centralize PostgreSQL pool setup around `search_path=expensor,public`

**Files:**
- Create: `backend/internal/dbconn/dbconn.go`
- Create: `backend/internal/dbconn/dbconn_test.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/pkg/writer/postgres/postgres.go`
- Modify: `backend/cmd/server/bootstrap.go`

- [ ] **Step 1: Write the failing test**

Add a unit test for the new helper that proves every pool config gets the schema search path set before any connection is opened.

```go
func TestParseConfigSetsSearchPath(t *testing.T) {
	cfg, err := ParseConfig("host=localhost port=5432 user=expensor dbname=expensor sslmode=disable")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if got := cfg.ConnConfig.RuntimeParams["search_path"]; got != "expensor,public" {
		t.Fatalf("search_path = %q, want expensor,public", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./backend/internal/dbconn -run TestParseConfigSetsSearchPath -v`

Expected: FAIL because `backend/internal/dbconn/dbconn.go` does not exist yet.

- [ ] **Step 3: Write the minimal implementation**

Create a tiny helper that parses a pgxpool config and sets the runtime search path before callers apply their own pool sizing.

```go
package dbconn

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const SearchPath = "expensor,public"

func ParseConfig(connStr string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = SearchPath
	return cfg, nil
}
```

Update `store.New`, `writer.New`, and the startup migration connection so they all use the helper instead of calling `pgxpool.ParseConfig` directly.

```go
poolCfg, err := dbconn.ParseConfig(connStr)
if err != nil {
	return nil, err
}
poolCfg.MaxConns = cfg.MaxPoolSize
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./backend/internal/dbconn -run TestParseConfigSetsSearchPath -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/dbconn backend/internal/store/store.go backend/pkg/writer/postgres/postgres.go backend/cmd/server/bootstrap.go
git commit -m "Set backend postgres search path"
```

---

### Task 2: Add the startup bridge that copies `public` into `expensor`

**Files:**
- Create: `backend/internal/bootstrapdb/bootstrap.go`
- Create: `backend/internal/bootstrapdb/bootstrap_test.go`
- Modify: `backend/cmd/server/bootstrap.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Write the failing tests**

Add three integration tests against a real Postgres container:

```go
func TestPrepareFreshDatabaseBootstrapsExpensor(t *testing.T) {
	// empty database, no public app tables
	// Prepare should create expensor, run golang-migrate, and leave schema_migrations in expensor
}

func TestPrepareExistingDatabaseCopiesPublicData(t *testing.T) {
	// seed a phase-1 style database: public tables + legacy_schema_migrations
	// insert one transaction row and one runtime row in public
	// Prepare should copy the data into expensor and baseline schema_migrations to the latest version
}

func TestPrepareBridgeFailureLeavesPublicUntouched(t *testing.T) {
	// call PrepareWithHooks with a ValidateCopy hook that returns errors.New("forced validation failure")
	// Prepare should return an error and public tables should still contain the original rows
}
```

The test helpers should assert the state with concrete SQL, not with mocks:

```sql
SELECT EXISTS (
	SELECT 1
	FROM information_schema.schemata
	WHERE schema_name = 'expensor'
);
```

```sql
SELECT COUNT(*) FROM expensor.transactions;
SELECT COUNT(*) FROM public.transactions;
SELECT version, dirty FROM expensor.schema_migrations;
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
go test ./backend/internal/bootstrapdb -run 'TestPrepare(FreshDatabaseBootstrapsExpensor|ExistingDatabaseCopiesPublicData|BridgeFailureLeavesPublicUntouched)' -v
```

Expected: FAIL because the bootstrap package and its tests do not exist yet.

- [ ] **Step 3: Write the minimal implementation**

Create a dedicated bootstrap package with one entry point that the server and tests can call.

```go
type Hooks struct {
	ValidateCopy func(ctx context.Context, pool *pgxpool.Pool) error
}

func Prepare(ctx context.Context, cfg config.Postgres, logger *slog.Logger) error {
	return PrepareWithHooks(ctx, cfg, logger, Hooks{})
}

func PrepareWithHooks(ctx context.Context, cfg config.Postgres, logger *slog.Logger, hooks Hooks) error {
	pool, err := openPool(cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	schemaExists, err := expensorSchemaExists(ctx, pool)
	if err != nil {
		return err
	}
	if !schemaExists && legacyAppExists(ctx, pool) {
		if err := ensureExpensorSchema(ctx, pool); err != nil {
			return err
		}
		if err := copyPublicToExpensor(ctx, pool, logger); err != nil {
			return err
		}
		if hooks.ValidateCopy != nil {
			if err := hooks.ValidateCopy(ctx, pool); err != nil {
				return err
			}
		}
		if err := baselineMigrations(ctx, pool, logger); err != nil {
			return err
		}
		return nil
	}

	if !schemaExists {
		if err := ensureExpensorSchema(ctx, pool); err != nil {
			return err
		}
	}

	return migrations.Run(ctx, pool, logger)
}
```

`openPool` should live inside `backend/internal/bootstrapdb` and use `dbconn.ParseConfig` so the startup bridge uses the same connection behavior as the store and writer pools, just with the admin pool size set to `1`.

The bridge should stay transactional for the copy-and-validate window. A failure before commit must leave `public` usable. Use an injected hook or equivalent test seam for the failure test so the production path stays clean.

Update `backend/cmd/server/bootstrap.go` so `runMigrations` becomes a thin wrapper around `bootstrapdb.Prepare`, and keep the startup order the same: wait for Postgres, then prepare the schema, then build the store.

```go
func runMigrations(pgCfg config.Postgres, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return bootstrapdb.Prepare(ctx, pgCfg, logger)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:
```bash
go test ./backend/internal/bootstrapdb -run 'TestPrepare(FreshDatabaseBootstrapsExpensor|ExistingDatabaseCopiesPublicData|BridgeFailureLeavesPublicUntouched)' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/bootstrapdb backend/cmd/server/bootstrap.go backend/cmd/server/main.go
git commit -m "Add startup schema bridge"
```

---

### Task 3: Replace the custom runner with `golang-migrate` and convert the embedded SQL files

**Files:**
- Modify: `backend/migrations/migrations.go`
- Modify: `backend/migrations/migrations_test.go`
- Create: `backend/migrations/001_init.up.sql`
- Create: `backend/migrations/001_init.down.sql`
- Create: `backend/migrations/002_source_struct_and_rule_senders.up.sql`
- Create: `backend/migrations/002_source_struct_and_rule_senders.down.sql`
- Create: `backend/migrations/003_predefined_rules_v2.up.sql`
- Create: `backend/migrations/003_predefined_rules_v2.down.sql`
- Delete: `backend/migrations/001_init.sql`
- Delete: `backend/migrations/002_source_struct_and_rule_senders.sql`
- Delete: `backend/migrations/003_predefined_rules_v2.sql`

- [ ] **Step 1: Write the failing tests**

Update the migration-package test so it proves the permanent runner now owns `schema_migrations` in `expensor`, not the retired custom table in `public`.

```go
func TestRunUsesSchemaMigrationsInExpensor(t *testing.T) {
	ctx := context.Background()
	pool := newMigrationTestPool(t)

	if err := Run(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
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
}

func TestLatestVersionMatchesEmbeddedFiles(t *testing.T) {
	v, err := LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if v != 3 {
		t.Fatalf("latest version = %d, want 3", v)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
go test ./backend/migrations -run 'TestRunUsesSchemaMigrationsInExpensor|TestLatestVersionMatchesEmbeddedFiles' -v
```

Expected: FAIL because the runner still uses the old custom bookkeeping table and the new `.up.sql` / `.down.sql` files do not exist yet.

- [ ] **Step 3: Write the minimal implementation**

Replace the sequential filename-tracking loop in `backend/migrations/migrations.go` with a `golang-migrate` runner backed by the embedded FS source driver.

```go
func Run(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	src, err := iofs.New(FS, ".")
	if err != nil {
		return fmt.Errorf("creating embedded migration source: %w", err)
	}

	driver, err := pgdriver.WithInstance(pool, &pgdriver.Config{
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		return fmt.Errorf("creating postgres migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("initializing migrate: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}
```

Add a `LatestVersion()` helper that reads the embedded filenames, extracts the highest numeric prefix, and returns that version so the startup bridge can baseline the database after the copy.

Convert the existing SQL into standard `golang-migrate` pairs:

- `001_init.up.sql` contains the current initial schema creation.
- `001_init.down.sql` drops the objects created by the initial migration in reverse dependency order: `transaction_label_sources`, `transaction_labels`, `label_merchants`, `merchant_categories`, `rules`, `muted_merchants`, `mcc_codes`, `labels`, `categories`, `buckets`, `app_config`, `extraction_diagnostics`, `reader_runtime`, `processed_messages`, `transactions`, the `update_updated_at_column()` function, its trigger on `transactions`, and finally `pg_trgm` if no later migration still needs it.
- `002_source_struct_and_rule_senders.up.sql` contains the current `ALTER TABLE` updates for structured source fields and sender arrays.
- `002_source_struct_and_rule_senders.down.sql` restores the phase-1 column set by dropping the structured source columns from `transactions` and `rules`, then leaving the legacy `source` and `sender_email` data in place.
- `003_predefined_rules_v2.up.sql` contains the current predefined rule seed/update migration.
- `003_predefined_rules_v2.down.sql` removes the predefined rows introduced by that migration by deleting only `rules.predefined = TRUE` rows whose `name` appears in the v2 set, and it leaves user-created rules untouched.

Keep the embedded migration files idempotent where the current schema model requires it, but stop relying on the custom table-name bookkeeping entirely.

- [ ] **Step 4: Run the tests to verify they pass**

Run:
```bash
go test ./backend/migrations -run 'TestRunUsesSchemaMigrationsInExpensor|TestLatestVersionMatchesEmbeddedFiles' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/migrations
git commit -m "Adopt golang-migrate for backend migrations"
```

---

### Task 4: Repoint the backend integration suites and run the full verification pass

**Files:**
- Modify: `backend/internal/store/store_test.go`
- Modify: `backend/pkg/writer/postgres/postgres_test.go`
- Modify: `backend/cmd/server/main.go`
- Review: `backend/internal/store/store.go`, `backend/pkg/writer/postgres/postgres.go`, `backend/internal/bootstrapdb/bootstrap.go`, `backend/migrations/migrations.go`

- [ ] **Step 1: Update the integration suite setup**

Replace the current direct call to `migrations.Run` in the test bootstrap code with `bootstrapdb.Prepare`, so the tests exercise the same startup-owned path that production uses.

```go
if err := bootstrapdb.Prepare(ctx, cfg, slog.Default()); err != nil {
	pool.Close()
	_ = ctr.Terminate(ctx)
	t.Fatalf("bootstrap failed: %v", err)
}
```

Do the same for the writer package `TestMain`, keeping the shared container but preparing the schema through the new bootstrap path before any writer is opened.

- [ ] **Step 2: Re-run the integration tests**

Run:
```bash
go test ./backend/internal/store ./backend/pkg/writer/postgres -count=1 -v
```

Expected: PASS against the migrated `expensor` schema.

- [ ] **Step 3: Run the backend-wide checks**

Run:
```bash
task test:be
task lint:be:prod
```

Expected: both pass with zero issues.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/store/store_test.go backend/pkg/writer/postgres/postgres_test.go backend/cmd/server/main.go
git commit -m "Switch backend tests to startup schema bridge"
```

---

### Task 5: Final review and release boundary check

**Files:**
- Review only: `backend/internal/dbconn/dbconn.go`, `backend/internal/bootstrapdb/bootstrap.go`, `backend/migrations/migrations.go`, `backend/internal/store/store.go`, `backend/pkg/writer/postgres/postgres.go`

- [ ] **Step 1: Confirm the runtime path matches the phase-2 spec**

Verify that:

```bash
rg -n "legacy_schema_migrations|schema_migrations|search_path=expensor,public|CREATE SCHEMA expensor" backend
```

Expected:
- `legacy_schema_migrations` remains only where phase 1 support still matters during the transition.
- `schema_migrations` is owned by `golang-migrate`.
- every backend pool uses `search_path=expensor,public`.
- `expensor` is created before runtime code touches the database.

- [ ] **Step 2: Stop**

Do not start phase 3 cleanup in this PR. The bridge must remain until the next release window closes.
