package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/auth"
)

type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type immediateWrite func(context.Context, func(queryer) error) error

type repositoryDependencies struct {
	db        *sql.DB
	query     queryer
	writeTx   immediateWrite
	now       func() time.Time
	secretBox *auth.SecretBox
}
