package store_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestInstrumentRecordsDebugDurationAndError(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug}))
	metrics := store.NewQueryInstrumentation(logger)

	err := metrics.Observe(context.Background(), "labels.list", func(context.Context) error {
		return errors.New("boom")
	})

	if err == nil {
		t.Fatal("expected error")
	}
	got := out.String()
	if !strings.Contains(got, "level=DEBUG") {
		t.Fatalf("expected debug log output, got %q", got)
	}
	if !strings.Contains(got, "labels.list") {
		t.Fatalf("expected operation in log output, got %q", got)
	}
	if !strings.Contains(got, "boom") {
		t.Fatalf("expected error in log output, got %q", got)
	}
}
