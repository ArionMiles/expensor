package postgres

import (
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

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
