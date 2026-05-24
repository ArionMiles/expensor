package observability_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/observability"
)

func TestDefaultConfigPreservesLoggingDefaults(t *testing.T) {
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("EXPENSOR_OBSERVABILITY_ENABLED", "")
	t.Setenv("EXPENSOR_OBSERVABILITY_EXPORTER", "")

	cfg := observability.DefaultConfig()

	if cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("LogLevel = %v, want INFO", cfg.LogLevel)
	}
	if cfg.LogJSON {
		t.Fatal("LogJSON = true, want false")
	}
	if cfg.Output == nil {
		t.Fatal("Output = nil, want non-nil")
	}
	if cfg.Enabled {
		t.Fatal("Enabled = true, want false")
	}
	if cfg.Exporter != observability.ExporterNone {
		t.Fatalf("Exporter = %q, want %q", cfg.Exporter, observability.ExporterNone)
	}
}

func TestDefaultConfigReadsLogLevelCaseInsensitively(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")

	cfg := observability.DefaultConfig()

	if cfg.LogLevel != slog.LevelDebug {
		t.Fatalf("LogLevel = %v, want DEBUG", cfg.LogLevel)
	}
}

func TestProductionConfigPreservesLoggingDefaults(t *testing.T) {
	cfg := observability.ProductionConfig()

	if cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("LogLevel = %v, want INFO", cfg.LogLevel)
	}
	if !cfg.LogJSON {
		t.Fatal("LogJSON = false, want true")
	}
	if cfg.Output == nil {
		t.Fatal("Output = nil, want non-nil")
	}
	if cfg.Enabled {
		t.Fatal("Enabled = true, want false")
	}
	if cfg.Exporter != observability.ExporterNone {
		t.Fatalf("Exporter = %q, want %q", cfg.Exporter, observability.ExporterNone)
	}
}

func TestSetupInstallsSlogLogger(t *testing.T) {
	var out bytes.Buffer
	shutdown, logger, err := observability.Setup(context.Background(), observability.Config{
		LogLevel: slog.LevelInfo,
		Output:   &out,
		Exporter: observability.ExporterNone,
	})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	logger.Info("hello", "component", "test")
	slog.Default().Info("from default", "component", "slog")

	got := out.String()
	if !strings.Contains(got, "hello") || !strings.Contains(got, "component=test") {
		t.Fatalf("log output = %q, want returned logger message and component", got)
	}
	if !strings.Contains(got, "from default") || !strings.Contains(got, "component=slog") {
		t.Fatalf("log output = %q, want default slog logger message and component", got)
	}
}

func TestSetupSupportsDisabledTelemetry(t *testing.T) {
	shutdown, logger, err := observability.Setup(context.Background(), observability.Config{
		LogLevel: slog.LevelInfo,
		Output:   &bytes.Buffer{},
		Exporter: observability.ExporterNone,
	})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if logger == nil {
		t.Fatal("logger is nil")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestOperationRecorderAcceptsSuccessAndError(t *testing.T) {
	logs := &bytes.Buffer{}
	shutdown, logger, err := observability.Setup(context.Background(), observability.Config{
		LogLevel:       slog.LevelDebug,
		Output:         logs,
		Enabled:        true,
		Exporter:       observability.ExporterNone,
		TracesEnabled:  true,
		MetricsEnabled: true,
	})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	scope := observability.NewScope(logger, "test")
	ctx, span := scope.Start(context.Background(), "store.test.success")
	scope.RecordOperation(ctx, observability.Operation{
		Namespace: "store",
		Name:      "test.success",
		Err:       nil,
	})
	span.End()

	scope.RecordOperation(context.Background(), observability.Operation{
		Namespace: "store",
		Name:      "test.error",
		Err:       errors.New("boom"),
	})
	if strings.Contains(logs.String(), "duration_ms") {
		t.Fatalf("operation logs should not include duration_ms, got %q", logs.String())
	}
}
