// Package scheduler coordinates tenant scanning work.
package scheduler

import (
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// FailureKind identifies the user-visible class of a scan failure.
type FailureKind string

const (
	FailureTemporary           FailureKind = "temporary"
	FailureNeedsAuth           FailureKind = "needs_auth"
	FailureReaderNotConfigured FailureKind = "reader_not_configured"
)

// FailureError is a scan error with a safe user-facing state transition.
type FailureError struct {
	Kind          FailureKind
	ReasonCode    store.ScanningReasonCode
	PublicMessage string
	Err           error
}

func (f *FailureError) Error() string {
	if f.Err != nil {
		return f.Err.Error()
	}
	return string(f.Kind)
}

func (f *FailureError) Unwrap() error {
	return f.Err
}

// ErrorKind maps scheduler failures onto the application's error taxonomy.
func (f *FailureError) ErrorKind() errors.Kind {
	switch f.Kind {
	case FailureNeedsAuth, FailureReaderNotConfigured:
		return errors.FailedPrecondition
	case FailureTemporary:
		return errors.Unavailable
	default:
		return errors.Unknown
	}
}

func NewMissingCredentialsFailure(err error) error {
	return &FailureError{
		Kind:          FailureNeedsAuth,
		ReasonCode:    store.ScanningReasonMissingCredentials,
		PublicMessage: "Upload reader credentials to continue scanning.",
		Err:           err,
	}
}

func NewMissingTokenFailure(err error) error {
	return &FailureError{
		Kind:          FailureNeedsAuth,
		ReasonCode:    store.ScanningReasonMissingToken,
		PublicMessage: "Connect your reader account to continue scanning.",
		Err:           err,
	}
}

func NewInvalidGrantFailure(err error) error {
	return &FailureError{
		Kind:          FailureNeedsAuth,
		ReasonCode:    store.ScanningReasonInvalidGrant,
		PublicMessage: "Reconnect your reader account to continue scanning.",
		Err:           err,
	}
}

func NewReaderNotConfiguredFailure(err error) error {
	return &FailureError{
		Kind:          FailureReaderNotConfigured,
		ReasonCode:    store.ScanningReasonReaderNotConfigured,
		PublicMessage: "Complete reader setup to continue scanning.",
		Err:           err,
	}
}

func classifyFailure(err error) FailureError {
	var failure *FailureError
	if errors.As(err, &failure) {
		return *failure
	}
	return FailureError{
		Kind:          FailureTemporary,
		ReasonCode:    store.ScanningReasonTemporaryFailure,
		PublicMessage: "Scanning hit a temporary problem. We will retry automatically.",
		Err:           err,
	}
}
