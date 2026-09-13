package store

import "github.com/ArionMiles/expensor/backend/pkg/errors"

// ValidateDiagnosticFilterStatus reports whether status is a supported diagnostic filter value.
func ValidateDiagnosticFilterStatus(status string) error {
	switch status {
	case DiagnosticStatusOpen, DiagnosticStatusResolved, DiagnosticStatusIgnored, DiagnosticStatusAll:
		return nil
	default:
		return errors.B.Op("store.diagnostics.validate_filter_status").KindInvalidInput().Text("invalid diagnostic status").Build()
	}
}

func ValidateDiagnosticUpdateStatus(status string) error {
	switch status {
	case DiagnosticStatusOpen, DiagnosticStatusResolved, DiagnosticStatusIgnored:
		return nil
	default:
		return errors.B.Op("store.diagnostics.validate_update_status").KindInvalidInput().Text("invalid diagnostic status").Build()
	}
}
