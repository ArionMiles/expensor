package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	stderrors "errors"
	"path/filepath"
	"testing"
	"time"

	moderncsqlite "modernc.org/sqlite"
)

const testBusyTimeout = 750 * time.Millisecond

func TestConnectionSetupTimeoutIsBoundedForLocalPragmas(t *testing.T) {
	if connectionSetupTimeout != time.Second {
		t.Fatalf("connection setup timeout = %v, want %v", connectionSetupTimeout, time.Second)
	}
}

func TestConnectorUsesCallerOwnedDriver(t *testing.T) {
	t.Parallel()

	ownedDriver := &moderncsqlite.Driver{}
	sqliteConnector := connector{
		driver: ownedDriver,
		dsn:    memoryDSN,
	}
	var _ driver.Connector = sqliteConnector

	if sqliteConnector.Driver() != ownedDriver {
		t.Fatal("connector returned a different driver")
	}

	db := sql.OpenDB(sqliteConnector)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping database: %v", err)
	}
}

func TestConnectorRejectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	conn, err := (connector{
		driver: &moderncsqlite.Driver{},
		dsn:    memoryDSN,
	}).Connect(ctx)
	if conn != nil {
		if closeErr := conn.Close(); closeErr != nil {
			t.Errorf("close unexpected connection: %v", closeErr)
		}
		t.Fatal("Connect returned a connection for a canceled context")
	}
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("Connect error = %v, want context.Canceled", err)
	}
}

func TestConnectorClosesConnectionCanceledDuringOpen(t *testing.T) {
	t.Parallel()

	opened := make(chan struct{})
	release := make(chan struct{})
	connection := &trackedDriverConnection{}
	blocking := &blockingDriver{
		opened:     opened,
		release:    release,
		connection: connection,
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		conn, err := (connector{driver: blocking, dsn: memoryDSN}).Connect(ctx)
		if conn != nil {
			_ = conn.Close()
		}
		result <- err
	}()

	<-opened
	cancel()
	close(release)
	if err := <-result; !stderrors.Is(err, context.Canceled) {
		t.Fatalf("Connect error = %v, want context.Canceled", err)
	}
	if !connection.closed {
		t.Fatal("connection opened after cancellation was not closed")
	}
}

func TestConfigureConnectionHonorsSetupContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := configureConnection(ctx, blockingSetupConnection{}, memoryDSN, "PRAGMA busy_timeout = 1")
	if !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("configureConnection error = %v, want context deadline exceeded", err)
	}
}

func TestPrivateInMemoryDatabaseUsesOneConnection(t *testing.T) {
	t.Parallel()

	for _, dsn := range []string{
		memoryDSN,
		"file:private_memory?mode=memory",
	} {
		t.Run(dsn, func(t *testing.T) {
			db := openTestDatabase(t, dsn)
			if got := db.Stats().MaxOpenConnections; got != 1 {
				t.Fatalf("maximum open connections = %d, want 1", got)
			}

			ctx := context.Background()
			conn, err := db.Conn(ctx)
			if err != nil {
				t.Fatalf("acquire private connection: %v", err)
			}
			if _, err := conn.ExecContext(ctx, "CREATE TABLE private_value (value TEXT NOT NULL)"); err != nil {
				t.Fatalf("create private table: %v", err)
			}

			waitingCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
			defer cancel()
			unexpected, err := db.Conn(waitingCtx)
			if unexpected != nil {
				closeConnection(t, unexpected)
				t.Fatal("acquired a second connection for a private in-memory database")
			}
			if !stderrors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("second connection error = %v, want context deadline exceeded", err)
			}
			closeConnection(t, conn)

			if _, err := db.ExecContext(ctx, "INSERT INTO private_value(value) VALUES (?)", "visible"); err != nil {
				t.Fatalf("insert private value: %v", err)
			}
			var value string
			if err := db.QueryRowContext(ctx, "SELECT value FROM private_value").Scan(&value); err != nil {
				t.Fatalf("read private value: %v", err)
			}
			if value != "visible" {
				t.Fatalf("private value = %q, want visible", value)
			}
		})
	}
}

func TestRequiredSQLiteFeatures(t *testing.T) {
	t.Parallel()

	db := openTestDatabase(t, "file:required_features?mode=memory&cache=shared")
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, "CREATE VIRTUAL TABLE documents USING fts5(body)"); err != nil {
		t.Fatalf("create FTS5 table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO documents(body) VALUES (?)", "quarterly café receipt"); err != nil {
		t.Fatalf("insert FTS5 document: %v", err)
	}
	var matches int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM documents WHERE documents MATCH 'cafe'").Scan(&matches); err != nil {
		t.Fatalf("query FTS5 table: %v", err)
	}
	if matches != 1 {
		t.Fatalf("FTS5 matches = %d, want 1", matches)
	}

	var valid, invalid int
	if err := db.QueryRowContext(ctx, "SELECT json_valid(?), json_valid(?)", `{"amount": 1}`, `{`).Scan(&valid, &invalid); err != nil {
		t.Fatalf("call json_valid: %v", err)
	}
	if valid != 1 || invalid != 0 {
		t.Fatalf("json_valid results = (%d, %d), want (1, 0)", valid, invalid)
	}

	var folded string
	if err := db.QueryRowContext(ctx, "SELECT expensor_casefold(?)", "Straße").Scan(&folded); err != nil {
		t.Fatalf("call expensor_casefold: %v", err)
	}
	if folded != "strasse" {
		t.Fatalf("expensor_casefold = %q, want %q", folded, "strasse")
	}
}

func TestEveryPooledFileConnectionHasRequiredSettings(t *testing.T) {
	t.Parallel()

	db := openTestDatabase(t, filepath.Join(t.TempDir(), "pool.db"))
	ctx := context.Background()

	connections := make([]*sql.Conn, 0, 8)
	for range 8 {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("acquire connection %d: %v", len(connections)+1, err)
		}
		connections = append(connections, conn)

		var foreignKeys, busyTimeout, synchronous int
		if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatalf("read foreign_keys on connection %d: %v", len(connections), err)
		}
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("read busy_timeout on connection %d: %v", len(connections), err)
		}
		if err := conn.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil {
			t.Fatalf("read synchronous on connection %d: %v", len(connections), err)
		}
		if foreignKeys != 1 || busyTimeout != int(testBusyTimeout.Milliseconds()) || synchronous != 1 {
			t.Fatalf(
				"connection %d settings = (foreign_keys=%d, busy_timeout=%d, synchronous=%d), want (1, %d, 1)",
				len(connections), foreignKeys, busyTimeout, synchronous, testBusyTimeout.Milliseconds(),
			)
		}
	}

	for index, conn := range connections {
		if err := conn.Close(); err != nil {
			t.Errorf("close connection %d: %v", index+1, err)
		}
	}
	stats := db.Stats()
	if stats.OpenConnections != 8 || stats.Idle != 8 {
		t.Fatalf("pool stats = (open=%d, idle=%d), want (8, 8)", stats.OpenConnections, stats.Idle)
	}

	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
}

func TestSharedInMemoryDatabaseIsVisibleAcrossConnections(t *testing.T) {
	t.Parallel()

	db := openTestDatabase(t, "file:shared_visibility?mode=memory&cache=shared")
	ctx := context.Background()

	writer, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire writer: %v", err)
	}
	t.Cleanup(func() { closeConnection(t, writer) })
	if _, err := writer.ExecContext(ctx, "CREATE TABLE shared_value (value TEXT NOT NULL)"); err != nil {
		t.Fatalf("create shared table: %v", err)
	}
	if _, err := writer.ExecContext(ctx, "INSERT INTO shared_value(value) VALUES (?)", "visible"); err != nil {
		t.Fatalf("insert shared value: %v", err)
	}

	reader, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire reader: %v", err)
	}
	t.Cleanup(func() { closeConnection(t, reader) })
	var value string
	if err := reader.QueryRowContext(ctx, "SELECT value FROM shared_value").Scan(&value); err != nil {
		t.Fatalf("read shared value: %v", err)
	}
	if value != "visible" {
		t.Fatalf("shared value = %q, want visible", value)
	}
}

func TestSharedInMemoryDatabaseSupportsEightConnections(t *testing.T) {
	t.Parallel()

	db := openTestDatabase(t, "file:eight_shared_connections?mode=memory&cache=shared")
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE shared_pool_value (value TEXT NOT NULL)"); err != nil {
		t.Fatalf("create shared pool table: %v", err)
	}

	connections := make([]*sql.Conn, 0, 8)
	for range 8 {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("acquire shared connection %d: %v", len(connections)+1, err)
		}
		connections = append(connections, conn)
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM shared_pool_value").Scan(&count); err != nil {
			t.Fatalf("query shared connection %d: %v", len(connections), err)
		}
	}
	for _, conn := range connections {
		closeConnection(t, conn)
	}

	stats := db.Stats()
	if stats.OpenConnections != 8 || stats.Idle != 8 {
		t.Fatalf("shared pool stats = (open=%d, idle=%d), want (8, 8)", stats.OpenConnections, stats.Idle)
	}
}

func TestFileDatabaseReopens(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "reopen.db")
	ctx := context.Background()
	db := openTestDatabase(t, path)
	if _, err := db.ExecContext(ctx, "CREATE TABLE persisted_value (value TEXT NOT NULL)"); err != nil {
		t.Fatalf("create persisted table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO persisted_value(value) VALUES (?)", "persisted"); err != nil {
		t.Fatalf("insert persisted value: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close first database: %v", err)
	}

	db = openTestDatabase(t, path)
	var value string
	if err := db.QueryRowContext(ctx, "SELECT value FROM persisted_value").Scan(&value); err != nil {
		t.Fatalf("read persisted value: %v", err)
	}
	if value != "persisted" {
		t.Fatalf("persisted value = %q, want persisted", value)
	}
}

func TestSecondDatabaseHandleSeesFileChanges(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "second_handle.db")
	first := openTestDatabase(t, path)
	second := openTestDatabase(t, path)
	ctx := context.Background()

	if _, err := first.ExecContext(ctx, "CREATE TABLE handle_value (value TEXT NOT NULL)"); err != nil {
		t.Fatalf("create table through first handle: %v", err)
	}
	if _, err := first.ExecContext(ctx, "INSERT INTO handle_value(value) VALUES (?)", "shared"); err != nil {
		t.Fatalf("insert through first handle: %v", err)
	}
	var value string
	if err := second.QueryRowContext(ctx, "SELECT value FROM handle_value").Scan(&value); err != nil {
		t.Fatalf("read through second handle: %v", err)
	}
	if value != "shared" {
		t.Fatalf("second handle value = %q, want shared", value)
	}
}

func openTestDatabase(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	db, err := openDatabase(context.Background(), dsn, testBusyTimeout)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	return db
}

func closeConnection(t *testing.T, conn *sql.Conn) {
	t.Helper()

	if err := conn.Close(); err != nil {
		t.Errorf("close connection: %v", err)
	}
}

type blockingDriver struct {
	opened     chan struct{}
	release    chan struct{}
	connection driver.Conn
}

func (d *blockingDriver) Open(string) (driver.Conn, error) {
	close(d.opened)
	<-d.release
	return d.connection, nil
}

type trackedDriverConnection struct {
	closed bool
}

func (*trackedDriverConnection) Prepare(string) (driver.Stmt, error) {
	return nil, stderrors.New("not implemented")
}

func (c *trackedDriverConnection) Close() error {
	c.closed = true
	return nil
}

func (*trackedDriverConnection) Begin() (driver.Tx, error) {
	return nil, stderrors.New("not implemented")
}

type blockingSetupConnection struct{}

func (blockingSetupConnection) ExecContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingSetupConnection) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return nil, stderrors.New("not implemented")
}
