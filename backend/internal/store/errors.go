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

// ErrAccessTokenNameConflict is returned when a user already has an active access token with the same name.
var ErrAccessTokenNameConflict = errors.New("access token name conflict")

// ErrUserEmailConflict is returned when a user email is already in use.
var ErrUserEmailConflict = errors.New("user email conflict")

// ErrPaginationOverflow is returned when a requested page cannot be represented as a SQL offset.
var ErrPaginationOverflow = errors.New("pagination offset overflow")

func isDiagnosticOpenConflict(err error) bool {
	return isUniqueViolationConstraint(
		err,
		"extraction_diagnostics_open_unique",
		"extraction_diagnostics_open_legacy_unique",
		"extraction_diagnostics_open_tenant_unique",
	)
}

func isRuleNameConflict(err error) bool {
	return isUniqueViolationConstraint(
		err,
		"rules_name_key",
		"rules_legacy_user_name_key",
		"rules_tenant_user_name_key",
	)
}

func isAccessTokenNameConflict(err error) bool {
	return isUniqueViolationConstraint(
		err,
		"access_tokens_user_id_name_key",
		"access_tokens_active_name_unique",
	)
}

func isUserEmailConflict(err error) bool {
	return isUniqueViolationConstraint(err, "users_email_key")
}

func isUniqueViolationConstraint(err error, constraints ...string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	for _, constraint := range constraints {
		if pgErr.ConstraintName == constraint {
			return true
		}
	}
	return false
}
