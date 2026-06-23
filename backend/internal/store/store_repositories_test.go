package store_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestStoreEmitsRepresentativeDebugInstrumentation(t *testing.T) {
	ts := newInstrumentedTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	if err := ts.CreateLabel(ctx, store.Tenant{}, "instrumented", "#38bdf8"); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}

	ts.requireOperation(t, "taxonomy.create_label")
}

type instrumentedTestStore struct {
	*store.InstrumentedStore
	base *testStore
	logs *bytes.Buffer
}

func newInstrumentedTestStore(t *testing.T) *instrumentedTestStore {
	t.Helper()
	logs := new(bytes.Buffer)
	ts := newTestStoreWithLogger(t, logs)
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scope := observability.NewScope(logger, "test.store")
	return &instrumentedTestStore{
		InstrumentedStore: store.NewInstrumentedStore(ts.Store, scope, logger),
		base:              ts,
		logs:              logs,
	}
}

func (ts *instrumentedTestStore) cleanup() {
	ts.base.cleanup()
}

func (ts *instrumentedTestStore) requireOperation(t *testing.T, operation string) {
	t.Helper()
	got := ts.logs.String()
	if !strings.Contains(got, "level=DEBUG") {
		t.Fatalf("expected debug logs, got %q", got)
	}
	if !strings.Contains(got, operation) {
		t.Fatalf("expected operation %q in logs, got %q", operation, got)
	}
}
