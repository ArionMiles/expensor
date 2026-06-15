# Golang-Migrate Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the current custom migration bookkeeping table so `schema_migrations` is available for the later `golang-migrate` release.

**Architecture:** Keep the existing sequential migration runner in place for now, but change it to track applied files in `legacy_schema_migrations` instead of `schema_migrations`. Cover both the runner and the writer bootstrap path with tests so the next release can assume the canonical `schema_migrations` name is free, while current databases continue to boot exactly as before.

**Tech Stack:** Go, PostgreSQL, pgx, embedded SQL migrations, Task

---

### Task 1: Rename the migration tracking table in the runner

**Files:**
- Create: `backend/internal/migration/migration_test.go`
- Modify: `backend/internal/migration/migration.go`

- [ ] **Step 1: Write the failing test**

Create a migration-package integration test that boots a temporary Postgres container, runs `Run`, and asserts the runner created and populated `legacy_schema_migrations` instead of `schema_migrations`.

```go
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

	if err := Run(ctx, pool, migrations.FS, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./backend/internal/migration -run TestRunUsesLegacySchemaMigrationsTable -v`
Expected: FAIL because the runner still creates and queries `schema_migrations`.

- [ ] **Step 3: Write the minimal implementation**

Change the table name literals in `backend/internal/migration/migration.go` from `schema_migrations` to `legacy_schema_migrations`, including the create-table statement, the existence check, and the insert.

```sql
CREATE TABLE IF NOT EXISTS legacy_schema_migrations (
	filename   TEXT PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
```

```sql
SELECT EXISTS(SELECT 1 FROM legacy_schema_migrations WHERE filename = $1)
```

```sql
INSERT INTO legacy_schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./backend/internal/migration -run TestRunUsesLegacySchemaMigrationsTable -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/migration/migration.go backend/internal/migration/migration_test.go
git commit -m "Rename legacy migration bookkeeping table"
```

### Task 2: Cover the writer bootstrap path and update migration comments

**Files:**
- Modify: `backend/pkg/writer/postgres/postgres_test.go`
- Modify: `backend/migrations/001_init.sql`
- Modify: `backend/internal/migration/migration.go`

- [ ] **Step 1: Write the failing test**

Add a writer integration test that creates a fresh writer, then checks that the bootstrap migration table is `legacy_schema_migrations` and that `schema_migrations` does not exist.

```go
func TestNewWriterUsesLegacySchemaMigrationsTable(t *testing.T) {
	w := newTestWriter(t, Config{})
	defer w.Close()

	ctx := context.Background()
	var legacyExists bool
	if err := w.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'legacy_schema_migrations'
		)
	`).Scan(&legacyExists); err != nil {
		t.Fatalf("check legacy table exists: %v", err)
	}
	if !legacyExists {
		t.Fatal("legacy_schema_migrations table not present after writer bootstrap")
	}

	var oldExists bool
	if err := w.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'schema_migrations'
		)
	`).Scan(&oldExists); err != nil {
		t.Fatalf("check old table exists: %v", err)
	}
	if oldExists {
		t.Fatal("schema_migrations table should not be present after phase 1")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./backend/pkg/writer/postgres -run TestNewWriterUsesLegacySchemaMigrationsTable -v`
Expected: FAIL because the writer bootstrap still records to `schema_migrations`.

- [ ] **Step 3: Write the minimal implementation**

Update the runner comment in `backend/internal/migration/migration.go` and the bootstrap comment in `backend/migrations/001_init.sql` so they describe `legacy_schema_migrations` instead of the retired table name.

```sql
-- This file is intentionally idempotent so the migration runner can safely
-- re-apply it if the legacy_schema_migrations record is lost.
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./backend/pkg/writer/postgres -run TestNewWriterUsesLegacySchemaMigrationsTable -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/pkg/writer/postgres/postgres_test.go backend/migrations/001_init.sql backend/internal/migration/migration.go
git commit -m "Update migration bootstrap coverage"
```

### Task 3: Verify the phase boundary is ready for the next release

**Files:**
- Review only: `backend/internal/migration/migration.go`, `backend/pkg/writer/postgres/postgres.go`, `backend/internal/migration/migration_test.go`, `backend/pkg/writer/postgres/postgres_test.go`

- [ ] **Step 1: Run the backend checks**

Run:
```bash
task test:be
task lint:be:prod
```
Expected: both pass with zero issues.

- [ ] **Step 2: Run the focused migration smoke tests**

Run:
```bash
go test ./backend/internal/migration ./backend/pkg/writer/postgres -run 'TestRunUsesLegacySchemaMigrationsTable|TestNewWriterUsesLegacySchemaMigrationsTable' -v
```
Expected: PASS, and no production code path mentions `schema_migrations` as the bookkeeping table.

- [ ] **Step 3: Stop**

Do not start `golang-migrate` adoption in this PR. Phase 2 gets its own branch after this one is merged and released.
