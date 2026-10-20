package sqlite

import (
	"context"
	"database/sql"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	moderncsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	apperrors "github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestMapSQLiteErrorUsesDriverErrorCodes(t *testing.T) {
	tests := []struct {
		name        string
		cause       func(*testing.T) error
		wantCode    int
		wantPrimary int
		wantKind    apperrors.Kind
	}{
		{
			name:        "extended constraint and primary constraint mask",
			cause:       uniqueConstraintError,
			wantCode:    sqlite3.SQLITE_CONSTRAINT_UNIQUE,
			wantPrimary: sqlite3.SQLITE_CONSTRAINT,
			wantKind:    apperrors.Conflict,
		},
		{name: "busy", cause: busyError, wantCode: sqlite3.SQLITE_BUSY, wantPrimary: sqlite3.SQLITE_BUSY, wantKind: apperrors.Unavailable},
		{name: "locked", cause: lockedError, wantCode: sqlite3.SQLITE_LOCKED, wantPrimary: sqlite3.SQLITE_LOCKED, wantKind: apperrors.Unavailable},
		{name: "read only", cause: readOnlyError, wantCode: sqlite3.SQLITE_READONLY, wantPrimary: sqlite3.SQLITE_READONLY, wantKind: apperrors.Internal},
		{name: "cannot open", cause: cannotOpenError, wantCode: sqlite3.SQLITE_CANTOPEN, wantPrimary: sqlite3.SQLITE_CANTOPEN, wantKind: apperrors.Internal},
		{name: "corrupt", cause: corruptError, wantCode: sqlite3.SQLITE_CORRUPT, wantPrimary: sqlite3.SQLITE_CORRUPT, wantKind: apperrors.Internal},
		{name: "not a database", cause: notADatabaseError, wantCode: sqlite3.SQLITE_NOTADB, wantPrimary: sqlite3.SQLITE_NOTADB, wantKind: apperrors.Internal},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cause := test.cause(t)
			if cause == nil {
				t.Fatal("driver operation error = nil, want SQLite error")
			}

			var sqliteErr *moderncsqlite.Error
			if !stderrors.As(cause, &sqliteErr) {
				t.Fatalf("driver error type = %T, want *sqlite.Error", cause)
			}
			if code := sqliteErr.Code(); code != test.wantCode {
				t.Fatalf("SQLite error code = %d, want %d", code, test.wantCode)
			}
			if primary := sqliteErr.Code() & 0xff; primary != test.wantPrimary {
				t.Fatalf("SQLite primary error code = %d, want %d", primary, test.wantPrimary)
			}

			mapped := mapSQLiteError("sqlite.test.operation", cause)
			if kind := apperrors.WhatKind(mapped); kind != test.wantKind {
				t.Fatalf("mapped error kind = %v, want %v", kind, test.wantKind)
			}
			if !apperrors.Is(mapped, cause) {
				t.Fatal("mapped error does not retain its driver cause")
			}
		})
	}
}

func TestMapSQLiteErrorMapsNoRowsSeparately(t *testing.T) {
	t.Parallel()

	mapped := mapSQLiteError("sqlite.test.get", sql.ErrNoRows)
	if kind := apperrors.WhatKind(mapped); kind != apperrors.NotFound {
		t.Fatalf("mapped sql.ErrNoRows kind = %v, want %v", kind, apperrors.NotFound)
	}
	if !apperrors.Is(mapped, sql.ErrNoRows) {
		t.Fatal("mapped error does not retain sql.ErrNoRows")
	}
}

func TestMapSQLiteErrorKeepsPublicAndLogDetailsStable(t *testing.T) {
	t.Parallel()

	const sensitiveCause = "database failure for tenant-sensitive-record"
	cause := stderrors.New(sensitiveCause)
	mapped := mapSQLiteError("sqlite.test.query", cause)

	if userMessage := apperrors.UserMsg(mapped); userMessage != "" {
		t.Fatalf("public message = %q, want empty", userMessage)
	}
	for _, attribute := range apperrors.LogDetailAttrs(mapped) {
		if strings.Contains(attribute.Value.String(), sensitiveCause) {
			t.Fatalf("log attribute %q contains raw cause text", attribute.Key)
		}
	}
	if !apperrors.Is(mapped, cause) {
		t.Fatal("mapped error does not retain its cause")
	}

	var appErr *apperrors.Error
	if !apperrors.As(mapped, &appErr) {
		t.Fatalf("mapped error type = %T, want *errors.Error", mapped)
	}
	if appErr.Text != "database operation failed" {
		t.Fatalf("internal text = %q, want stable database operation text", appErr.Text)
	}
}

func TestMapSQLiteErrorAcceptsNil(t *testing.T) {
	t.Parallel()

	if err := mapSQLiteError("sqlite.test", nil); err != nil {
		t.Fatalf("mapSQLiteError(nil) = %v, want nil", err)
	}
}

func uniqueConstraintError(t *testing.T) error {
	t.Helper()

	db := openDriverTestDatabase(t, ":memory:")
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE unique_value (value TEXT NOT NULL UNIQUE)"); err != nil {
		t.Fatalf("create unique table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO unique_value(value) VALUES (?)", "duplicate"); err != nil {
		t.Fatalf("insert first unique value: %v", err)
	}
	_, err := db.ExecContext(ctx, "INSERT INTO unique_value(value) VALUES (?)", "duplicate")
	return err
}

func busyError(t *testing.T) error {
	t.Helper()

	path := filepath.Join(t.TempDir(), "busy.db")
	first := openDriverTestDatabase(t, path)
	second := openDriverTestDatabase(t, path)
	ctx := context.Background()
	for _, db := range []*sql.DB{first, second} {
		if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 0"); err != nil {
			t.Fatalf("disable busy timeout: %v", err)
		}
	}
	if _, err := first.ExecContext(ctx, "CREATE TABLE busy_value (value INTEGER NOT NULL)"); err != nil {
		t.Fatalf("create busy table: %v", err)
	}
	if _, err := first.ExecContext(ctx, "INSERT INTO busy_value(value) VALUES (1)"); err != nil {
		t.Fatalf("insert busy fixture: %v", err)
	}

	writer, err := first.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin writer transaction: %v", err)
	}
	t.Cleanup(func() { _ = writer.Rollback() })
	reader, err := second.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin reader transaction: %v", err)
	}
	t.Cleanup(func() { _ = reader.Rollback() })
	rows, err := reader.QueryContext(ctx, "SELECT value FROM busy_value")
	if err != nil {
		t.Fatalf("hold read lock: %v", err)
	}
	t.Cleanup(func() { _ = rows.Close() })
	if !rows.Next() {
		t.Fatal("busy fixture query returned no row")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read busy fixture: %v", err)
	}
	if _, err := writer.ExecContext(ctx, "INSERT INTO busy_value(value) VALUES (2)"); err != nil {
		t.Fatalf("write before blocked commit: %v", err)
	}
	return writer.Commit()
}

func lockedError(t *testing.T) error {
	t.Helper()

	db := openDriverTestDatabase(t, filepath.Join(t.TempDir(), "locked.db"))
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE locked_value (value INTEGER NOT NULL)"); err != nil {
		t.Fatalf("create locked table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO locked_value(value) VALUES (1)"); err != nil {
		t.Fatalf("insert locked fixture: %v", err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire locked connection: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	rows, err := conn.QueryContext(ctx, "SELECT value FROM locked_value")
	if err != nil {
		t.Fatalf("hold active statement: %v", err)
	}
	t.Cleanup(func() { _ = rows.Close() })
	if !rows.Next() {
		t.Fatal("locked fixture query returned no row")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read locked fixture: %v", err)
	}
	_, err = conn.ExecContext(ctx, "DROP TABLE locked_value")
	return err
}

func readOnlyError(t *testing.T) error {
	t.Helper()

	path := filepath.Join(t.TempDir(), "read_only.db")
	writable := openDriverTestDatabase(t, path)
	ctx := context.Background()
	if _, err := writable.ExecContext(ctx, "CREATE TABLE read_only_value (value INTEGER NOT NULL)"); err != nil {
		t.Fatalf("create read-only fixture: %v", err)
	}
	if err := writable.Close(); err != nil {
		t.Fatalf("close writable database: %v", err)
	}

	readOnly := openDriverTestDatabase(t, "file:"+filepath.ToSlash(path)+"?mode=ro")
	_, err := readOnly.ExecContext(ctx, "INSERT INTO read_only_value(value) VALUES (1)")
	return err
}

func cannotOpenError(t *testing.T) error {
	t.Helper()

	notDirectory := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(notDirectory, []byte("file"), 0o600); err != nil {
		t.Fatalf("create non-directory parent: %v", err)
	}
	db := openDriverTestDatabase(t, filepath.Join(notDirectory, "database.db"))
	return db.PingContext(context.Background())
}

func corruptError(t *testing.T) error {
	t.Helper()

	path := filepath.Join(t.TempDir(), "corrupt.db")
	db := openDriverTestDatabase(t, path)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE corrupt_value (value INTEGER NOT NULL)"); err != nil {
		t.Fatalf("create corrupt fixture: %v", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA writable_schema = ON"); err != nil {
		t.Fatalf("enable writable schema: %v", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE sqlite_schema SET sql = 'CREATE TABLE corrupt_value(' WHERE name = 'corrupt_value'"); err != nil {
		t.Fatalf("corrupt schema: %v", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA writable_schema = OFF"); err != nil {
		t.Fatalf("disable writable schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close corrupt fixture: %v", err)
	}

	db = openDriverTestDatabase(t, path)
	var count int
	return db.QueryRowContext(ctx, "SELECT count(*) FROM corrupt_value").Scan(&count)
}

func notADatabaseError(t *testing.T) error {
	t.Helper()

	path := filepath.Join(t.TempDir(), "not_a_database.db")
	if err := os.WriteFile(path, []byte("this file has no SQLite database header and is long enough to read"), 0o600); err != nil {
		t.Fatalf("create invalid database: %v", err)
	}
	db := openDriverTestDatabase(t, path)
	var schemaVersion int
	return db.QueryRowContext(context.Background(), "PRAGMA schema_version").Scan(&schemaVersion)
}

func openDriverTestDatabase(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open driver database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close driver database: %v", err)
		}
	})
	return db
}
