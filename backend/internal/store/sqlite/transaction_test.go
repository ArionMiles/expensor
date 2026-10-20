package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	stderrors "errors"
	"slices"
	"testing"
)

func TestImmediateWriteSequenceAndCleanup(t *testing.T) {
	t.Parallel()

	callbackErr := stderrors.New("callback failure")
	beginErr := stderrors.New("begin failure")
	commitErr := stderrors.New("commit failure")
	rollbackErr := stderrors.New("rollback failure")
	tests := []struct {
		name          string
		failures      map[string]error
		rawError      error
		closeError    error
		callbackError error
		wantError     error
		wantEvents    []string
		wantDiscarded bool
	}{
		{
			name:       "commit",
			wantEvents: []string{"BEGIN IMMEDIATE", "callback", "COMMIT", "close"},
		},
		{
			name:          "begin failure discards uncertain connection",
			failures:      map[string]error{"BEGIN IMMEDIATE": beginErr},
			wantError:     beginErr,
			wantEvents:    []string{"BEGIN IMMEDIATE", "discard"},
			wantDiscarded: true,
		},
		{
			name:          "begin bad connection was already discarded",
			failures:      map[string]error{"BEGIN IMMEDIATE": driver.ErrBadConn},
			rawError:      sql.ErrConnDone,
			closeError:    sql.ErrConnDone,
			wantError:     driver.ErrBadConn,
			wantEvents:    []string{"BEGIN IMMEDIATE", "discard"},
			wantDiscarded: true,
		},
		{
			name:          "callback failure rolls back",
			callbackError: callbackErr,
			wantError:     callbackErr,
			wantEvents:    []string{"BEGIN IMMEDIATE", "callback", "ROLLBACK", "close"},
		},
		{
			name:       "commit failure rolls back",
			failures:   map[string]error{"COMMIT": commitErr},
			wantError:  commitErr,
			wantEvents: []string{"BEGIN IMMEDIATE", "callback", "COMMIT", "ROLLBACK", "close"},
		},
		{
			name:          "rollback failure discards connection",
			failures:      map[string]error{"ROLLBACK": rollbackErr},
			callbackError: callbackErr,
			wantError:     rollbackErr,
			wantEvents:    []string{"BEGIN IMMEDIATE", "callback", "ROLLBACK", "discard"},
			wantDiscarded: true,
		},
		{
			name:          "rollback bad connection was already discarded",
			failures:      map[string]error{"ROLLBACK": driver.ErrBadConn},
			rawError:      sql.ErrConnDone,
			closeError:    sql.ErrConnDone,
			callbackError: callbackErr,
			wantError:     driver.ErrBadConn,
			wantEvents:    []string{"BEGIN IMMEDIATE", "callback", "ROLLBACK", "discard"},
			wantDiscarded: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gate := make(chan struct{}, 1)
			gate <- struct{}{}
			connection := &scriptedWriteConnection{
				failures:   tt.failures,
				rawError:   tt.rawError,
				closeError: tt.closeError,
			}
			acquisitions := 0
			store := &Store{
				writeGate: gate,
				acquireWriteConnection: func(context.Context) (writeConnection, error) {
					acquisitions++
					return connection, nil
				},
			}

			err := store.withImmediateWrite(context.Background(), func(queryer) error {
				connection.events = append(connection.events, "callback")
				return tt.callbackError
			})
			if tt.wantError == nil && err != nil {
				t.Fatalf("withImmediateWrite error = %v", err)
			}
			if tt.wantError != nil && !stderrors.Is(err, tt.wantError) {
				t.Fatalf("withImmediateWrite error = %v, want %v", err, tt.wantError)
			}
			if stderrors.Is(err, sql.ErrConnDone) {
				t.Fatalf("withImmediateWrite error includes false cleanup failure: %v", err)
			}
			if acquisitions != 1 {
				t.Fatalf("connection acquisitions = %d, want 1", acquisitions)
			}
			if !slices.Equal(connection.events, tt.wantEvents) {
				t.Fatalf("events = %v, want %v", connection.events, tt.wantEvents)
			}
			if connection.discarded != tt.wantDiscarded {
				t.Fatalf("discarded = %t, want %t", connection.discarded, tt.wantDiscarded)
			}
			select {
			case <-gate:
			default:
				t.Fatal("write gate was not released")
			}
		})
	}
}

func TestImmediateWriteReleasesGateAfterConnectionAcquisitionFailure(t *testing.T) {
	t.Parallel()

	wantErr := stderrors.New("acquisition failure")
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	store := &Store{
		writeGate: gate,
		acquireWriteConnection: func(context.Context) (writeConnection, error) {
			return nil, wantErr
		},
	}

	err := store.withImmediateWrite(context.Background(), func(queryer) error {
		t.Fatal("callback ran without a connection")
		return nil
	})
	if !stderrors.Is(err, wantErr) {
		t.Fatalf("withImmediateWrite error = %v, want %v", err, wantErr)
	}
	select {
	case <-gate:
	default:
		t.Fatal("write gate was not released")
	}
}

func TestDiscardedSQLConnectionDoesNotReportCloseError(t *testing.T) {
	db := openTestDatabase(t, memoryDSN)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("acquire real connection: %v", err)
	}

	discarded, err := discardWriteConnection(conn)
	if err != nil {
		t.Fatalf("discard real connection: %v", err)
	}
	if !discarded {
		t.Fatal("real connection was not marked discarded")
	}
	if err := closeWriteConnection(conn, discarded); err != nil {
		t.Fatalf("close discarded connection: %v", err)
	}
	if err := conn.Close(); !stderrors.Is(err, sql.ErrConnDone) {
		t.Fatalf("direct Close error = %v, want sql.ErrConnDone", err)
	}
}

type scriptedWriteConnection struct {
	failures   map[string]error
	rawError   error
	closeError error
	events     []string
	discarded  bool
}

func (c *scriptedWriteConnection) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	c.events = append(c.events, query)
	if stderrors.Is(c.failures[query], driver.ErrBadConn) {
		c.discarded = true
	}
	return driver.RowsAffected(1), c.failures[query]
}

func (*scriptedWriteConnection) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, stderrors.New("not implemented")
}

func (*scriptedWriteConnection) QueryRowContext(context.Context, string, ...any) *sql.Row {
	return nil
}

func (c *scriptedWriteConnection) Raw(callback func(any) error) error {
	c.events = append(c.events, "discard")
	if c.rawError != nil {
		return c.rawError
	}
	c.discarded = true
	return callback(nil)
}

func (c *scriptedWriteConnection) Close() error {
	c.events = append(c.events, "close")
	return c.closeError
}
