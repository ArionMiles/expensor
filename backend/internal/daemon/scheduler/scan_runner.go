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
	err := r.scanner.Run(ctx, daemon.ScanRequest{Tenant: tenant, Reader: reader, Mode: daemon.ScanScheduled})
	if err == nil {
		return nil
	}
	switch {
	case errors.WhatKind(err) == daemon.KindReaderNotConfigured:
		return NewReaderNotConfiguredFailure(err)
	case errors.WhatKind(err) == oauth.KindCredentialsMissing:
		return NewMissingCredentialsFailure(err)
	case errors.WhatKind(err) == oauth.KindTokenMissing:
		return NewMissingTokenFailure(err)
	case oauth.IsInvalidGrant(err):
		return NewInvalidGrantFailure(err)
	default:
		return err
	}
}
