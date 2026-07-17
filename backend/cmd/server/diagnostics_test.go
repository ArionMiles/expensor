package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestDaemonCoordinatorScopesDiagnosticToRunTenant(t *testing.T) {
	tenant := store.Tenant{ID: "tenant-interactive"}
	diagnostics := &recordingDiagnosticStore{}
	coordinator := &daemonCoordinator{
		ctx:               t.Context(),
		registry:          diagnosticTestRegistry(t),
		diagnostics:       diagnostics,
		transactionWriter: &discardTransactionWriter{},
		dm:                &daemonManager{},
		logger:            testLogger(),
	}

	coordinator.runDaemon(t.Context(), daemonRun{readerName: "diagnostic-reader", tenant: tenant, cfg: config.App{}})

	assertRecordedDiagnosticTenant(t, diagnostics, tenant)
}

func TestScheduledScanScopesDiagnosticToRunTenant(t *testing.T) {
	tenant := store.Tenant{ID: "tenant-scheduled"}
	diagnostics := &recordingDiagnosticStore{}
	runner := &scheduledScanRunner{
		registry:          diagnosticTestRegistry(t),
		st:                diagnosticTestStore{},
		diagnostics:       diagnostics,
		transactionWriter: &discardTransactionWriter{},
		logger:            testLogger(),
	}

	if err := runner.Run(t.Context(), tenant, "diagnostic-reader"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	assertRecordedDiagnosticTenant(t, diagnostics, tenant)
}

type recordingDiagnosticStore struct {
	tenant     store.Tenant
	diagnostic api.ExtractionDiagnostic
}

func (s *recordingDiagnosticStore) RecordExtractionDiagnostic(
	_ context.Context,
	tenant store.Tenant,
	diagnostic api.ExtractionDiagnostic,
) error {
	s.tenant = tenant
	s.diagnostic = diagnostic
	return nil
}

type discardTransactionWriter struct{}

func (*discardTransactionWriter) Write(context.Context, store.IngestionBatch) error {
	return nil
}

type diagnosticTestStore struct{}

func (diagnosticTestStore) GetAppConfig(context.Context, store.Tenant, string) (string, error) {
	return "", errors.E(errors.NotFound, "not found")
}

func (diagnosticTestStore) SetAppConfig(context.Context, store.Tenant, string, string) error {
	return nil
}

func (diagnosticTestStore) SetActiveScanningReader(context.Context, store.Tenant, string) error {
	return nil
}

func (diagnosticTestStore) ListRules(context.Context, store.Tenant) ([]store.RuleRow, error) {
	return nil, nil
}

type diagnosticEmittingReader struct {
	sink api.DiagnosticSink
}

func (r diagnosticEmittingReader) Read(ctx context.Context, out chan<- *api.TransactionDetails, _ <-chan string) error {
	defer close(out)
	return r.sink.RecordExtractionDiagnostic(ctx, api.ExtractionDiagnostic{MessageID: "diagnostic-message"})
}

func diagnosticTestRegistry(t *testing.T) *plugins.Registry {
	t.Helper()

	registry := plugins.NewRegistry()
	err := registry.RegisterProvider(plugins.Provider{
		Metadata: plugins.ProviderMetadata{
			Name: "diagnostic-reader",
			Auth: plugins.AuthSpec{Type: plugins.AuthTypeConfig},
		},
		NewReader: func(input plugins.ProviderInput) (api.Reader, error) {
			return diagnosticEmittingReader{sink: input.DiagnosticSink}, nil
		},
		NewEmailSearcher: func(plugins.ProviderInput) (api.EmailSearcher, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	return registry
}

func assertRecordedDiagnosticTenant(t *testing.T, diagnostics *recordingDiagnosticStore, want store.Tenant) {
	t.Helper()

	if diagnostics.tenant != want {
		t.Fatalf("diagnostic tenant = %#v, want %#v", diagnostics.tenant, want)
	}
	if diagnostics.diagnostic.MessageID != "diagnostic-message" {
		t.Fatalf("diagnostic = %#v, want emitted diagnostic", diagnostics.diagnostic)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
