package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrNotFound is returned when an operation targets a row that does not exist.
var ErrNotFound = errors.New("not found")

// ErrBootstrapUnavailable is returned when first-admin bootstrap has already been completed.
var ErrBootstrapUnavailable = errors.New("bootstrap unavailable")

// ErrRuleNameConflict is returned when a rule name is already in use.
var ErrRuleNameConflict = errors.New("rule name conflict")

// ErrDiagnosticConflict is returned when reopening a diagnostic would duplicate an existing open diagnostic.
var ErrDiagnosticConflict = errors.New("diagnostic conflict")

// ErrPaginationOverflow is returned when a requested page cannot be represented as a SQL offset.
var ErrPaginationOverflow = errors.New("pagination offset overflow")

func isDiagnosticOpenConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "extraction_diagnostics_open_unique"
}

func isRuleNameConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "rules_name_key"
}
