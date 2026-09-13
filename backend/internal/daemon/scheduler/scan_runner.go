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
		return errors.B.Op(op).UserMsg("Complete reader setup to continue scanning.").Err(err).Build()
	case kind == oauth.KindCredentialsMissing:
		return errors.B.Op(op).UserMsg("Upload reader credentials to continue scanning.").Err(err).Build()
	case kind == oauth.KindTokenMissing:
		return errors.B.Op(op).UserMsg("Connect your reader account to continue scanning.").Err(err).Build()
	case oauth.IsInvalidGrant(err):
		return errors.B.Op(op).KindFailedPrecondition().UserMsg("Reconnect your reader account to continue scanning.").Err(err).Build()

	default:
		return err
	}
}
