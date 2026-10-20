package sqlite

import (
	"database/sql"

	moderncsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func mapSQLiteError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return errors.B.Op(op).KindNotFound().Text("database row not found").Err(err).Build()
	}

	kind := errors.Internal
	text := "database operation failed"
	var sqliteErr *moderncsqlite.Error
	if errors.As(err, &sqliteErr) {
		kind = sqliteCodeKind(sqliteErr.Code())
		switch kind {
		case errors.Conflict:
			text = "database constraint failed"
		case errors.Unavailable:
			text = "database is unavailable"
		default:
			text = "database access failed"
		}
	}
	return errors.B.Op(op).Kind(kind).Text(text).Err(err).Build()
}

func sqliteCodeKind(code int) errors.Kind {
	switch code & 0xff {
	case sqlite3.SQLITE_CONSTRAINT:
		return errors.Conflict
	case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
		return errors.Unavailable
	case sqlite3.SQLITE_READONLY, sqlite3.SQLITE_CANTOPEN, sqlite3.SQLITE_CORRUPT, sqlite3.SQLITE_NOTADB:
		return errors.Internal
	default:
		return errors.Internal
	}
}
