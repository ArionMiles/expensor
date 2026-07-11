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
		name string
		err  error
		kind FailureKind
	}{
		{name: "reader", err: apperrors.E(daemon.KindReaderNotConfigured, "missing"), kind: FailureReaderNotConfigured},
		{name: "credentials", err: apperrors.E(oauth.KindCredentialsMissing, "missing"), kind: FailureNeedsAuth},
		{name: "token", err: apperrors.E(oauth.KindTokenMissing, "missing"), kind: FailureNeedsAuth},
		{name: "invalid grant", err: errors.New("oauth2: invalid_grant"), kind: FailureNeedsAuth},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NewScanRunner(&scannerStub{err: tc.err}).Run(context.Background(), store.Tenant{ID: "tenant-a"}, "gmail")
			var failure *FailureError
			if !errors.As(err, &failure) || failure.Kind != tc.kind {
				t.Fatalf("Run() error = %#v, want failure kind %q", err, tc.kind)
			}
		})
	}
}
