// Package migrations embeds and applies the SQLite baseline schema.
package migrations

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"embed"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const schemaVersion = 1

const migrationRollbackTimeout = 5 * time.Second

// FS contains the SQLite migration files.
//
//go:embed *.up.sql *.down.sql
var FS embed.FS

// Run applies the embedded baseline migration when the database is empty.
func Run(ctx context.Context, db *sql.DB) error {
	migrationSQL, err := FS.ReadFile("001_init.up.sql")
	if err != nil {
		return errors.B.Op("sqlite.migrations.run").KindInternal().Text("reading embedded baseline migration").Err(err).Build()
	}
	return runMigration(ctx, db, string(migrationSQL))
}

func runMigration(ctx context.Context, db *sql.DB, migrationSQL string) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return errors.B.Op("sqlite.migrations.run").KindInternal().Text("acquiring migration connection").Err(err).Build()
	}
	discarded := false
	defer func() {
		if !discarded {
			err = errors.Join(err, migrationConnectionCloseError(conn.Close()))
		}
	}()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		discarded = migrationConnectionDiscarded(err)
		return errors.B.Op("sqlite.migrations.run").KindInternal().Text("starting migration transaction").Err(err).Build()
	}
	rollback := func(cause error) error {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), migrationRollbackTimeout)
		defer cancel()
		_, rollbackErr := conn.ExecContext(rollbackCtx, "ROLLBACK")
		if rollbackErr != nil {
			discardErr := conn.Raw(func(any) error { return driver.ErrBadConn })
			if migrationConnectionDiscarded(discardErr) {
				discarded = true
			} else if discardErr != nil {
				rollbackErr = errors.Join(rollbackErr, discardErr)
			}
		}
		return errors.Join(cause, rollbackErr)
	}

	var currentVersion int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&currentVersion); err != nil {
		return rollback(errors.B.Op("sqlite.migrations.run").KindInternal().Text("reading schema version").Err(err).Build())
	}
	if currentVersion > schemaVersion {
		return rollback(errors.B.Op("sqlite.migrations.run").KindFailedPrecondition().Text("database schema is newer than this binary").Build())
	}
	if currentVersion == schemaVersion {
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return rollback(errors.B.Op("sqlite.migrations.run").KindInternal().Text("committing migration transaction").Err(err).Build())
		}
		return nil
	}

	if _, err := conn.ExecContext(ctx, migrationSQL); err != nil {
		return rollback(errors.B.Op("sqlite.migrations.run").KindInternal().Text("applying baseline migration").Err(err).Build())
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA user_version = 1"); err != nil {
		return rollback(errors.B.Op("sqlite.migrations.run").KindInternal().Text("recording schema version").Err(err).Build())
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return rollback(errors.B.Op("sqlite.migrations.run").KindInternal().Text("committing migration transaction").Err(err).Build())
	}
	return nil
}

func migrationConnectionDiscarded(err error) bool {
	return errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone)
}

func migrationConnectionCloseError(err error) error {
	if errors.Is(err, sql.ErrConnDone) {
		return nil
	}
	return err
}
