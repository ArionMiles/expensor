package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/oauth"
	"github.com/ArionMiles/expensor/backend/internal/store"
	apperrors "github.com/ArionMiles/expensor/backend/pkg/errors"
)

type scannerStub struct {
	request daemon.ScanRequest
	err     error
}

func (s *scannerStub) Run(_ context.Context, request daemon.ScanRequest) error {
	s.request = request
	return s.err
}

func TestScanRunnerRequestsScheduledMode(t *testing.T) {
	scanner := &scannerStub{}
	tenant := store.Tenant{ID: "tenant-a"}
	if err := NewScanRunner(scanner).Run(context.Background(), tenant, "gmail"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if scanner.request.Tenant.ID != tenant.ID || scanner.request.Reader != "gmail" || scanner.request.Mode != daemon.ScanScheduled {
		t.Fatalf("scan request = %#v", scanner.request)
	}
}

func TestScanRunnerMapsSharedScanFailures(t *testing.T) {
	tests := []struct {
		name       string
		cause      error
		kind       apperrors.Kind
		publicText string
	}{
		{
			name:       "reader",
			cause:      apperrors.E(daemon.KindReaderNotConfigured, "missing"),
			kind:       daemon.KindReaderNotConfigured,
			publicText: "Complete reader setup to continue scanning.",
		},
		{
			name:       "credentials",
			cause:      apperrors.E(oauth.KindCredentialsMissing, "missing"),
			kind:       oauth.KindCredentialsMissing,
			publicText: "Upload reader credentials to continue scanning.",
		},
		{
			name:       "token",
			cause:      apperrors.E(oauth.KindTokenMissing, "missing"),
			kind:       oauth.KindTokenMissing,
			publicText: "Connect your reader account to continue scanning.",
		},
		{
			name:       "invalid grant",
			cause:      errors.New("oauth2: invalid_grant"),
			kind:       apperrors.FailedPrecondition,
			publicText: "Reconnect your reader account to continue scanning.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NewScanRunner(&scannerStub{err: tc.cause}).Run(context.Background(), store.Tenant{ID: "tenant-a"}, "gmail")
			if !apperrors.Is(err, tc.cause) {
				t.Fatalf("Run() error = %v, want wrapped cause %v", err, tc.cause)
			}
			if got := apperrors.WhatKind(err); got != tc.kind {
				t.Fatalf("WhatKind() = %#v, want %#v", got, tc.kind)
			}
			if got := apperrors.UserMsg(err); got != tc.publicText {
				t.Fatalf("UserMsg() = %q, want %q", got, tc.publicText)
			}
		})
	}
}
