package scheduler

import (
	"context"

	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/oauth"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type scanner interface {
	Run(ctx context.Context, request daemon.ScanRequest) error
}

// ScanRunner adapts shared scan execution to scheduler failure classifications.
type ScanRunner struct {
	scanner scanner
}

// NewScanRunner constructs a scheduled scan adapter.
func NewScanRunner(next scanner) *ScanRunner {
	return &ScanRunner{scanner: next}
}

// Run performs one bounded scheduled scan.
func (r *ScanRunner) Run(ctx context.Context, tenant store.Tenant, reader string) error {
	op := "scheduler.scan_runner.run"
	err := r.scanner.Run(ctx, daemon.ScanRequest{Tenant: tenant, Reader: reader, Mode: daemon.ScanScheduled})
	if err == nil {
		return nil
	}
	switch kind := errors.WhatKind(err); {
	case kind == daemon.KindReaderNotConfigured:
		return errors.E(op, errors.User("Complete reader setup to continue scanning."), err)
	case kind == oauth.KindCredentialsMissing:
		return errors.E(op, errors.User("Upload reader credentials to continue scanning."), err)
	case kind == oauth.KindTokenMissing:
		return errors.E(op, errors.User("Connect your reader account to continue scanning."), err)
	case oauth.IsInvalidGrant(err):
		return errors.E(
			op, errors.FailedPrecondition, errors.User("Reconnect your reader account to continue scanning."), err,
		)
	default:
		return err
	}
}
