package sqlite

import (
	"context"
	"database/sql"
	stderrors "errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestNewUsesRelativePathFromWorkingDirectory(t *testing.T) {
	workingDirectory := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	store := openTestStore(t, filepath.Join("relative", "expensor.db"))
	if _, err := os.Stat(filepath.Join(workingDirectory, "relative", "expensor.db")); err != nil {
		t.Fatalf("stat relative database: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workingDirectory, "expensor.db")); !os.IsNotExist(err) {
		t.Fatalf("database resolved outside its relative parent: %v", err)
	}
	if err := store.HealthCheck(context.Background()); err != nil {
		t.Fatalf("health check relative database: %v", err)
	}
}

func TestNewExpandsExplicitHomePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	store := openTestStore(t, "~/private/expensor.db")
	if err := store.HealthCheck(context.Background()); err != nil {
		t.Fatalf("health check home database: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "private", "expensor.db")); err != nil {
		t.Fatalf("stat home database: %v", err)
	}
}

func TestNewRejectsHomePathWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")

	store, err := New(context.Background(), testStoreOptions("~/expensor.db"))
	if store != nil {
		store.Close()
		t.Fatal("New returned a store without a home directory")
	}
	if err == nil {
		t.Fatal("New returned no error without a home directory")
	}
}

func TestNewCanceledBeforePathPreparationCreatesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "expensor.db")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store, err := New(ctx, testStoreOptions(path))
	if store != nil {
		store.Close()
		t.Fatal("New returned a store for a canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("New error = %v, want context.Canceled", err)
	}
	if got := errors.WhatKind(err); got != errors.Canceled {
		t.Fatalf("error kind = %v, want %v", got, errors.Canceled)
	}
	if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
		t.Fatalf("database path exists after cancellation: %v", statErr)
	}
}

func TestNewRejectsUnsupportedDSNOptions(t *testing.T) {
	tests := []string{
		"file:policy?mode=memory&_pragma=journal_mode%3DOFF",
		"file:policy?mode=memory&_journal_mode=DELETE",
		"file:policy?mode=memory&_query_only=1",
		"file:policy?mode=memory&_synchronous=OFF",
		"file:policy?mode=ro",
		"file:policy.db?cache=shared",
		"file:policy?mode=memory&cache=private",
		"file:policy?mode=memory&mode=memory",
		"file:policy?mode=memory&unknown=value",
		"file:policy?mode=memory#fragment",
	}

	for _, dsn := range tests {
		t.Run(dsn, func(t *testing.T) {
			store, err := New(context.Background(), testStoreOptions(dsn))
			if store != nil {
				store.Close()
				t.Fatal("New returned a store for an unsupported DSN")
			}
			if got := errors.WhatKind(err); got != errors.InvalidArgument {
				t.Fatalf("error kind = %v, want %v: %v", got, errors.InvalidArgument, err)
			}
		})
	}
}

func TestNewRejectsPlainPathDSNOptionsBeforeCreatingFiles(t *testing.T) {
	tests := []string{
		"?_pragma=journal_mode%3DOFF",
		"?_journal_mode=DELETE",
		"?mode=ro",
		"#fragment",
	}

	for _, suffix := range tests {
		t.Run(suffix, func(t *testing.T) {
			driverPath := filepath.Join(t.TempDir(), "expensor.db")
			dsn := driverPath + suffix

			store, err := New(context.Background(), testStoreOptions(dsn))
			if store != nil {
				store.Close()
				t.Fatal("New returned a store for an unsupported plain-path DSN")
			}
			if got := errors.WhatKind(err); got != errors.InvalidArgument {
				t.Fatalf("error kind = %v, want %v: %v", got, errors.InvalidArgument, err)
			}
			for _, path := range []string{dsn, driverPath} {
				if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
					t.Fatalf("rejected DSN created a database file: %v", statErr)
				}
			}
		})
	}
}

func TestNewRejectsInvalidParentType(t *testing.T) {
	t.Parallel()

	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("file"), 0o600); err != nil {
		t.Fatalf("create parent file: %v", err)
	}

	store, err := New(context.Background(), testStoreOptions(filepath.Join(parent, "expensor.db")))
	if store != nil {
		store.Close()
		t.Fatal("New returned a store for an invalid parent")
	}
	if err == nil {
		t.Fatal("New returned no error for an invalid parent")
	}
	if got := errors.WhatKind(err); got != errors.InvalidArgument {
		t.Fatalf("error kind = %v, want %v: %v", got, errors.InvalidArgument, err)
	}
}

func TestNewRejectsDeniedDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory mode test")
	}

	parent := filepath.Join(t.TempDir(), "denied")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatalf("create denied directory: %v", err)
	}
	if err := os.Chmod(parent, 0); err != nil {
		t.Fatalf("deny directory access: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(parent, 0o700); err != nil {
			t.Errorf("restore directory mode: %v", err)
		}
	})

	store, err := New(context.Background(), testStoreOptions(filepath.Join(parent, "expensor.db")))
	if err == nil {
		store.Close()
		t.Skip("runtime can bypass directory permission bits")
	}
	if store != nil {
		store.Close()
		t.Fatal("New returned a store for a denied directory")
	}
	if got := errors.WhatKind(err); got != errors.PermissionDenied {
		t.Fatalf("error kind = %v, want %v: %v", got, errors.PermissionDenied, err)
	}
}

func TestNewRejectsReadOnlyDatabase(t *testing.T) {
	path := filepath.Join(privateTestDir(t), "read-only.db")
	store := openTestStore(t, path)
	store.Close()

	t.Run("explicit read-only mode", func(t *testing.T) {
		readOnlyDSN := "file:" + filepath.ToSlash(path) + "?mode=ro"
		store, err := New(context.Background(), testStoreOptions(readOnlyDSN))
		if store != nil {
			store.Close()
			t.Fatal("New returned a read-only store")
		}
		if err == nil {
			t.Fatal("New returned no error for a read-only store")
		}
	})
}

func TestNewRejectsCorruptDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(privateTestDir(t), "corrupt.db")
	if err := os.WriteFile(path, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatalf("create corrupt database: %v", err)
	}

	store, err := New(context.Background(), testStoreOptions(path))
	if store != nil {
		store.Close()
		t.Fatal("New returned a corrupt store")
	}
	if err == nil {
		t.Fatal("New returned no error for a corrupt database")
	}
}

func TestNewSupportsPrivateInMemoryDatabase(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, memoryDSN)
	if got := store.db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("maximum open connections = %d, want 1", got)
	}
	var journalMode string
	if err := store.db.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if journalMode == "wal" {
		t.Fatal("private in-memory database forced WAL mode")
	}
}

func TestSharedMemoryPoolConfiguresEveryConnection(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, "file:configured_shared_memory?mode=memory&cache=shared")
	ctx := context.Background()
	connections := make([]*sql.Conn, 0, connectionPoolSize)
	for range connectionPoolSize {
		conn, err := store.db.Conn(ctx)
		if err != nil {
			t.Fatalf("acquire connection %d: %v", len(connections)+1, err)
		}
		connections = append(connections, conn)

		var foreignKeys, busyTimeout int
		if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatalf("read foreign keys on connection %d: %v", len(connections), err)
		}
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("read busy timeout on connection %d: %v", len(connections), err)
		}
		if foreignKeys != 1 || busyTimeout != int(testBusyTimeout.Milliseconds()) {
			t.Fatalf(
				"connection %d settings = (foreign_keys=%d, busy_timeout=%d), want (1, %d)",
				len(connections), foreignKeys, busyTimeout, testBusyTimeout.Milliseconds(),
			)
		}
	}
	for _, conn := range connections {
		closeConnection(t, conn)
	}

	var journalMode string
	if err := store.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if strings.EqualFold(journalMode, "wal") {
		t.Fatal("shared in-memory database forced WAL mode")
	}
}

func TestNewInitializesMigratedRepositoryDependencies(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, "file:store_dependencies?mode=memory&cache=shared")
	var schemaVersion int
	if err := store.db.QueryRowContext(context.Background(), "PRAGMA user_version").Scan(&schemaVersion); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if schemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1", schemaVersion)
	}
	if store.repositories.db != store.db || store.repositories.query == nil || store.repositories.writeTx == nil ||
		store.repositories.now == nil {
		t.Fatal("repository dependencies were not initialized")
	}
}

func TestNewLifecycleOrderAndFailureCleanup(t *testing.T) {
	t.Parallel()

	stageErr := stderrors.New("forced lifecycle failure")
	tests := []struct {
		name      string
		failStage string
		wantOrder []string
	}{
		{name: "success", wantOrder: []string{"health", "migrations", "repositories"}},
		{name: "health failure", failStage: "health", wantOrder: []string{"health"}},
		{name: "migration failure", failStage: "migrations", wantOrder: []string{"health", "migrations"}},
		{name: "repository failure", failStage: "repositories", wantOrder: []string{"health", "migrations", "repositories"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var order []string
			var captured *Store
			record := func(stage string, store *Store) error {
				order = append(order, stage)
				captured = store
				if tt.failStage == stage {
					return stageErr
				}
				return nil
			}
			hooks := lifecycleHooks{
				health: func(_ context.Context, store *Store) error {
					return record("health", store)
				},
				migrate: func(_ context.Context, store *Store) error {
					return record("migrations", store)
				},
				initRepositories: func(store *Store, _ config.Security) error {
					return record("repositories", store)
				},
			}

			store, err := newWithLifecycle(
				context.Background(),
				testStoreOptions(filepath.Join(privateTestDir(t), "lifecycle.db")),
				hooks,
			)
			if !slices.Equal(order, tt.wantOrder) {
				t.Fatalf("lifecycle order = %v, want %v", order, tt.wantOrder)
			}
			if tt.failStage == "" {
				if err != nil {
					t.Fatalf("newWithLifecycle error = %v", err)
				}
				if store == nil {
					t.Fatal("newWithLifecycle returned no store")
				}
				store.Close()
				return
			}

			if store != nil {
				store.Close()
				t.Fatal("newWithLifecycle returned a store after failure")
			}
			if !stderrors.Is(err, stageErr) {
				t.Fatalf("newWithLifecycle error = %v, want forced failure", err)
			}
			if captured == nil {
				t.Fatal("lifecycle did not receive the store")
			}
			if err := captured.db.PingContext(context.Background()); err == nil {
				t.Fatal("database remained open after lifecycle failure")
			}
		})
	}
}

func TestNewRejectsInvalidSecretAfterMigration(t *testing.T) {
	t.Parallel()

	path := filepath.Join(privateTestDir(t), "invalid-secret.db")
	opts := testStoreOptions(path)
	opts.Security.SecretKey = []byte("too short")
	store, err := New(context.Background(), opts)
	if store != nil {
		store.Close()
		t.Fatal("New returned a store with an invalid secret")
	}
	if err == nil {
		t.Fatal("New returned no error for an invalid secret")
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open database after initialization failure: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("close reopened database: %v", err)
		}
	}()
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping database after initialization failure: %v", err)
	}
}

func TestHealthCheckFailsAfterClose(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, "file:closed_health?mode=memory&cache=shared")
	store.Close()
	if err := store.HealthCheck(context.Background()); err == nil {
		t.Fatal("HealthCheck returned no error after Close")
	}
}

func TestImmediateWriteCommitsAndRollsBack(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, "file:write_transactions?mode=memory&cache=shared")
	ctx := context.Background()
	if _, err := store.db.ExecContext(ctx, "CREATE TABLE write_value (value TEXT NOT NULL)"); err != nil {
		t.Fatalf("create write table: %v", err)
	}
	if err := store.withImmediateWrite(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, "INSERT INTO write_value(value) VALUES (?)", "committed")
		return err
	}); err != nil {
		t.Fatalf("commit immediate write: %v", err)
	}

	wantErr := stderrors.New("stop write")
	err := store.withImmediateWrite(ctx, func(query queryer) error {
		if _, err := query.ExecContext(ctx, "INSERT INTO write_value(value) VALUES (?)", "rolled back"); err != nil {
			return err
		}
		return wantErr
	})
	if !stderrors.Is(err, wantErr) {
		t.Fatalf("rollback error = %v, want %v", err, wantErr)
	}

	var count int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM write_value").Scan(&count); err != nil {
		t.Fatalf("count write values: %v", err)
	}
	if count != 1 {
		t.Fatalf("write value count = %d, want 1", count)
	}
}

func TestCanceledWriteGateWaitDoesNotAcquireConnection(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, "file:canceled_gate?mode=memory&cache=shared")
	if cap(store.writeGate) != 1 {
		t.Fatalf("write gate capacity = %d, want 1", cap(store.writeGate))
	}
	<-store.writeGate

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	err := store.withImmediateWrite(ctx, func(queryer) error {
		t.Fatal("write callback ran after context cancellation")
		return nil
	})
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("write error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("canceled gate wait took %v", elapsed)
	}
	stats := store.db.Stats()
	if stats.InUse != 0 || stats.WaitCount != 0 {
		t.Fatalf("pool after canceled gate wait = (in_use=%d, wait_count=%d), want (0, 0)", stats.InUse, stats.WaitCount)
	}
	store.writeGate <- struct{}{}
}

func openTestStore(t *testing.T, path string) *Store {
	t.Helper()

	store, err := New(context.Background(), testStoreOptions(path))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(store.Close)
	return store
}

func privateTestDir(t *testing.T) string {
	t.Helper()

	directory := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("create private test directory: %v", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatalf("set private test directory mode: %v", err)
	}
	return directory
}

func testStoreOptions(path string) Options {
	return Options{
		Config: config.SQLite{
			Path:        path,
			BusyTimeout: testBusyTimeout,
		},
	}
}
