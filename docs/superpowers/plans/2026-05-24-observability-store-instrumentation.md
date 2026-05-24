# Observability Store Instrumentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current logging-only store instrumentation with an observability package that keeps `slog` for logs and uses OpenTelemetry for traces and metrics, then move store operation instrumentation into interface decorators.

**Architecture:** `backend/pkg/observability` owns logger setup, OTel tracer/meter setup, trace-aware log helpers, and low-cardinality operation recording. `backend/internal/store` exposes capability interfaces and instrumentation decorators that wrap concrete store implementations instead of calling instrumentation helpers inside repository methods. `cmd/server` wires an observed store into API/daemon dependencies while preserving default no-op telemetry unless explicitly enabled.

**Tech Stack:** Go 1.26, `log/slog`, OpenTelemetry Go SDK/API, pgx, existing `task` test targets.

---

## Scope Check

The approved spec covers multiple backend subsystems. This plan intentionally implements the first independently testable milestone:

- `pkg/logging` -> `pkg/observability`
- OTel traces and metrics setup through OTLP only, no OTel logs and no stdout telemetry exporters
- trace/span IDs in slog records where context has an active span
- store instrumentation as decorators over capability interfaces
- removal of inline `QueryInstrumentation.Observe` calls from repositories
- `AGENTS.md` backend guidance for observability/store instrumentation

Separate plans should follow for plugin registry cleanup, API handler file decomposition, state context cleanup, and provider-specific domain cleanup.

---

## File Structure

Create:

- `backend/pkg/observability/observability.go`: public setup API, logger setup, OTLP OTel providers, shutdown function.
- `backend/pkg/observability/config.go`: env/default config parsing.
- `backend/pkg/observability/logging.go`: slog trace/span enrichment helpers.
- `backend/pkg/observability/store.go`: operation span/metric helper used by store decorators.
- `backend/pkg/observability/observability_test.go`: config/default/log enrichment tests.
- `tests/otel-collector.local.yml`: local OpenTelemetry Collector config for `task dev`.
- `tests/prometheus.local.yml`: local Prometheus config for `task dev`.
- `backend/internal/store/interfaces.go`: capability interfaces implemented by `*Store` and decorators.
- `backend/internal/store/instrumented.go`: store facade decorator and per-capability operation wrappers.
- `backend/internal/store/instrumented_test.go`: decorator behavior tests.

Modify:

- `backend/pkg/logging/logging.go`: remove after migration.
- `backend/pkg/logging/logging_test.go`: remove after migration.
- `backend/cmd/server/main.go`: import `observability`, initialize it, wrap `store.Store` before passing it to API/daemon helpers, and call shutdown on exit.
- `Taskfile.yml`: make `task dev` start the local observability collector and visualizers.
- `tests/docker-compose.local.yml`: add local OpenTelemetry Collector, Jaeger, and Prometheus services.
- `tests/.env`: enable OTLP telemetry for local dev.
- `backend/internal/store/repository_dependencies.go`: remove `metrics *QueryInstrumentation`.
- `backend/internal/store/*_repository.go`: remove inline `metrics.Observe` wrappers and `metrics` fields.
- `backend/internal/store/store.go`: use repository constructors without metrics, expose concrete facade methods as today.
- `backend/internal/store/instrumentation.go`: delete after decorators replace it.
- `backend/internal/store/instrumentation_test.go`: delete after decorator tests replace it.
- `AGENTS.md`: add backend architecture guidance.

---

### Task 1: Rename Logging Package To Observability

**Files:**
- Create: `backend/pkg/observability/config.go`
- Create: `backend/pkg/observability/observability.go`
- Create: `backend/pkg/observability/observability_test.go`
- Delete: `backend/pkg/logging/logging.go`
- Delete: `backend/pkg/logging/logging_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Write failing observability config tests**

Create `backend/pkg/observability/observability_test.go` with:

```go
package observability_test

import (
	"bytes"
	"context"
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

	if got := out.String(); !strings.Contains(got, "hello") || !strings.Contains(got, "component=test") {
		t.Fatalf("log output = %q, want message and component", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
task test:be -- ./pkg/observability
```

Expected: FAIL because `backend/pkg/observability` does not exist.

- [ ] **Step 3: Implement minimal observability package with disabled telemetry path**

Create `backend/pkg/observability/config.go`:

```go
package observability

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

type Exporter string

const (
	ExporterNone   Exporter = "none"
	ExporterOTLP   Exporter = "otlp"
)

type Config struct {
	ServiceName    string
	ServiceVersion string
	LogLevel       slog.Level
	LogJSON        bool
	Output         io.Writer
	Enabled        bool
	Exporter       Exporter
	OTLPEndpoint   string
	OTLPInsecure   bool
	TracesEnabled  bool
	MetricsEnabled bool
}

func DefaultConfig() Config {
	enabled := envBool("EXPENSOR_OBSERVABILITY_ENABLED")
	exporter := Exporter(strings.ToLower(strings.TrimSpace(os.Getenv("EXPENSOR_OBSERVABILITY_EXPORTER"))))
	if exporter == "" {
		exporter = ExporterNone
	}
	if !enabled {
		exporter = ExporterNone
	}
	return Config{
		ServiceName:    "expensor",
		ServiceVersion: "dev",
		LogLevel:       parseLogLevel(os.Getenv("LOG_LEVEL")),
		Output:         os.Stderr,
		Enabled:        enabled,
		Exporter:       exporter,
		OTLPEndpoint:   strings.TrimSpace(os.Getenv("EXPENSOR_OBSERVABILITY_OTLP_ENDPOINT")),
		OTLPInsecure:   envBool("EXPENSOR_OBSERVABILITY_OTLP_INSECURE"),
		TracesEnabled:  enabled,
		MetricsEnabled: enabled,
	}
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
```

Create `backend/pkg/observability/observability.go`:

```go
package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type Shutdown func(context.Context) error

func Setup(_ context.Context, cfg Config) (Shutdown, *slog.Logger, error) {
	logger := setupLogger(cfg)
	slog.SetDefault(logger)
	return func(context.Context) error { return nil }, logger, nil
}

func setupLogger(cfg Config) *slog.Logger {
	output := cfg.Output
	if output == nil {
		output = os.Stderr
	}
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	var handler slog.Handler
	if cfg.LogJSON {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}
	return slog.New(handler)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
```

- [ ] **Step 4: Migrate imports and delete old logging package**

In `backend/cmd/server/main.go`, change:

```go
"github.com/ArionMiles/expensor/backend/pkg/logging"
```

to:

```go
"github.com/ArionMiles/expensor/backend/pkg/observability"
```

Change:

```go
logger := logging.Setup(logging.DefaultConfig())
```

to:

```go
shutdownObservability, logger, err := observability.Setup(context.Background(), observability.DefaultConfig())
if err != nil {
	fmt.Fprintf(os.Stderr, "failed to initialize observability: %v\n", err)
	os.Exit(1)
}
defer func() {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := shutdownObservability(shutdownCtx); err != nil {
		logger.Warn("failed to shutdown observability", "error", err)
	}
}()
```

Delete:

```bash
backend/pkg/logging/logging.go
backend/pkg/logging/logging_test.go
```

- [ ] **Step 5: Run tests**

Run:

```bash
task test:be -- ./pkg/observability ./cmd/server
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/pkg/observability backend/cmd/server/main.go backend/pkg/logging
git commit --no-gpg-sign -m "Introduce backend observability package"
```

---

### Task 2: Add OpenTelemetry Traces And Metrics Setup

**Files:**
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`
- Modify: `backend/pkg/observability/observability.go`
- Create: `backend/pkg/observability/store.go`
- Modify: `backend/pkg/observability/observability_test.go`

- [ ] **Step 1: Write failing tests for no-op telemetry and operation recording**

Append to `backend/pkg/observability/observability_test.go`:

```go
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
	shutdown, logger, err := observability.Setup(context.Background(), observability.Config{
		LogLevel:       slog.LevelDebug,
		Output:         &bytes.Buffer{},
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
		Duration:  time.Millisecond,
		Err:       nil,
	})
	span.End()

	scope.RecordOperation(context.Background(), observability.Operation{
		Namespace: "store",
		Name:      "test.error",
		Duration:  time.Millisecond,
		Err:       errors.New("boom"),
	})
}
```

Add imports:

```go
import (
	"errors"
	"time"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
task test:be -- ./pkg/observability
```

Expected: FAIL because `NewScope`, `Operation`, and `RecordOperation` do not exist.

- [ ] **Step 3: Add OTel dependencies**

Run:

```bash
go get go.opentelemetry.io/otel@latest go.opentelemetry.io/otel/sdk@latest go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@latest go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc@latest
```

If this fails due to network restrictions, rerun with escalation approval.

- [ ] **Step 4: Implement OTel setup and operation scope**

Replace `backend/pkg/observability/observability.go` with:

```go
package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

type Shutdown func(context.Context) error

func Setup(ctx context.Context, cfg Config) (Shutdown, *slog.Logger, error) {
	logger := setupLogger(cfg)
	slog.SetDefault(logger)

	if !cfg.Enabled || cfg.Exporter == ExporterNone {
		return func(context.Context) error { return nil }, logger, nil
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating telemetry resource: %w", err)
	}

	var shutdowns []Shutdown
	if cfg.TracesEnabled {
		tp, err := newTracerProvider(ctx, cfg, res)
		if err != nil {
			return nil, nil, err
		}
		otel.SetTracerProvider(tp)
		shutdowns = append(shutdowns, tp.Shutdown)
	}
	if cfg.MetricsEnabled {
		mp, err := newMeterProvider(ctx, cfg, res)
		if err != nil {
			return nil, nil, err
		}
		otel.SetMeterProvider(mp)
		shutdowns = append(shutdowns, mp.Shutdown)
	}

	return func(ctx context.Context) error {
		var joined error
		for _, shutdown := range shutdowns {
			joined = errors.Join(joined, shutdown(ctx))
		}
		return joined
	}, logger, nil
}

func setupLogger(cfg Config) *slog.Logger {
	output := cfg.Output
	if output == nil {
		output = os.Stderr
	}
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogJSON {
		return slog.New(slog.NewJSONHandler(output, opts))
	}
	return slog.New(slog.NewTextHandler(output, opts))
}

func newTracerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	switch cfg.Exporter {
	case ExporterOTLP:
		opts := []otlptracegrpc.Option{}
		if cfg.OTLPEndpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint))
		}
		if cfg.OTLPInsecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err := otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("creating otlp trace exporter: %w", err)
		}
		return sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithBatcher(exporter)), nil
	default:
		return sdktrace.NewTracerProvider(sdktrace.WithResource(res)), nil
	}
}

func newMeterProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	switch cfg.Exporter {
	case ExporterOTLP:
		opts := []otlpmetricgrpc.Option{}
		if cfg.OTLPEndpoint != "" {
			opts = append(opts, otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint))
		}
		if cfg.OTLPInsecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		exporter, err := otlpmetricgrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("creating otlp metric exporter: %w", err)
		}
		return sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		), nil
	default:
		return sdkmetric.NewMeterProvider(sdkmetric.WithResource(res)), nil
	}
}

type Scope struct {
	logger *slog.Logger
	tracer trace.Tracer
	meter  metric.Meter
}

func NewScope(logger *slog.Logger, name string) *Scope {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scope{
		logger: logger,
		tracer: otel.Tracer(name),
		meter:  otel.Meter(name),
	}
}

func (s *Scope) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return s.tracer.Start(ctx, name, opts...)
}
```

Create `backend/pkg/observability/store.go`:

```go
package observability

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Operation struct {
	Namespace string
	Name      string
	Duration  time.Duration
	Err       error
}

func (s *Scope) RecordOperation(ctx context.Context, op Operation) {
	status := "ok"
	if op.Err != nil {
		status = "error"
	}
	attrs := []attribute.KeyValue{
		attribute.String("namespace", op.Namespace),
		attribute.String("operation", op.Name),
		attribute.String("status", status),
	}

	counter, _ := s.meter.Int64Counter(op.Namespace + ".operations")
	counter.Add(ctx, 1, metric.WithAttributes(attrs...))

	duration, _ := s.meter.Float64Histogram(op.Namespace + ".operation.duration_ms")
	duration.Record(ctx, float64(op.Duration.Milliseconds()), metric.WithAttributes(attrs...))

	span := trace.SpanFromContext(ctx)
	if op.Err != nil {
		span.RecordError(op.Err)
		span.SetStatus(codes.Error, op.Err.Error())
		s.logger.ErrorContext(ctx, "operation failed",
			"namespace", op.Namespace,
			"operation", op.Name,
			"duration_ms", op.Duration.Milliseconds(),
			"trace_id", span.SpanContext().TraceID().String(),
			"span_id", span.SpanContext().SpanID().String(),
			"error", op.Err,
		)
		return
	}
	s.logger.DebugContext(ctx, "operation completed",
		"namespace", op.Namespace,
		"operation", op.Name,
		"duration_ms", op.Duration.Milliseconds(),
		"trace_id", span.SpanContext().TraceID().String(),
		"span_id", span.SpanContext().SpanID().String(),
	)
}
```

- [ ] **Step 5: Run tests**

Run:

```bash
task test:be -- ./pkg/observability
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/go.mod backend/go.sum backend/pkg/observability
git commit --no-gpg-sign -m "Add traces and metrics setup"
```

---

### Task 3: Add Local Development Observability Stack

**Files:**
- Modify: `Taskfile.yml`
- Modify: `tests/docker-compose.local.yml`
- Modify: `tests/.env`
- Create: `tests/otel-collector.local.yml`
- Create: `tests/prometheus.local.yml`

- [ ] **Step 1: Add local collector and metrics config**

Create `tests/otel-collector.local.yml`:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:

exporters:
  otlp/jaeger:
    endpoint: jaeger:4317
    tls:
      insecure: true
  prometheus:
    endpoint: 0.0.0.0:8889

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/jaeger]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
```

Create `tests/prometheus.local.yml`:

```yaml
global:
  scrape_interval: 5s

scrape_configs:
  - job_name: otel-collector
    static_configs:
      - targets: ["otel-collector:8889"]
```

- [ ] **Step 2: Add local compose services**

In `tests/docker-compose.local.yml`, add services:

- `jaeger` using `jaegertracing/jaeger:2.11.0`, exposing UI `16686:16686` and OTLP gRPC `4319:4317` to avoid clashing with the collector's host OTLP port.
- `otel-collector` using `otel/opentelemetry-collector-contrib:0.140.0`, mounting `./otel-collector.local.yml`, exposing `4317:4317`, `4318:4318`, and `8889:8889`, and depending on `jaeger`.
- `prometheus` using `prom/prometheus:v3.7.3`, mounting `./prometheus.local.yml`, exposing `9090:9090`, and depending on `otel-collector`.

Keep the existing `postgres` service behavior unchanged.

- [ ] **Step 3: Enable OTLP telemetry in local env**

Append to `tests/.env`:

```dotenv
# Observability: task dev starts the local collector and visualizers.
EXPENSOR_OBSERVABILITY_ENABLED=true
EXPENSOR_OBSERVABILITY_EXPORTER=otlp
EXPENSOR_OBSERVABILITY_OTLP_ENDPOINT=localhost:4317
EXPENSOR_OBSERVABILITY_OTLP_INSECURE=true
```

- [ ] **Step 4: Make `task dev` start observability services**

Update the `dev` task description to mention local observability services and URLs:

- Jaeger traces: `http://localhost:16686`
- Prometheus metrics: `http://localhost:9090`

In the `dev` command, `task db:start` already runs `docker compose -f docker-compose.local.yml --env-file .env up -d` from `tests`, so all services in the compose file start together. Add echo lines after the health wait:

```bash
echo "Jaeger traces: http://localhost:16686"
echo "Prometheus metrics: http://localhost:9090"
```

Update the `db:start` task summary/description from "PostgreSQL container" to "local development containers" because it now starts PostgreSQL plus observability services.

- [ ] **Step 5: Verify compose config**

Run:

```bash
cd tests && docker compose -f docker-compose.local.yml --env-file .env config >/tmp/expensor-dev-compose.yml
```

Expected: exit 0.

- [ ] **Step 6: Commit**

```bash
git add Taskfile.yml tests/docker-compose.local.yml tests/.env tests/otel-collector.local.yml tests/prometheus.local.yml
git commit --no-gpg-sign -m "Add local observability dev stack"
```

---

### Task 4: Add Store Capability Interfaces And Instrumented Facade

**Files:**
- Create: `backend/internal/store/interfaces.go`
- Create: `backend/internal/store/instrumented.go`
- Create: `backend/internal/store/instrumented_test.go`

- [ ] **Step 1: Write failing decorator tests**

Create `backend/internal/store/instrumented_test.go`:

```go
package store_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/observability"
)

type fakeTransactionStore struct {
	called bool
	err    error
}

func (f *fakeTransactionStore) ListTransactions(ctx context.Context, filter store.ListFilter) ([]store.Transaction, store.TransactionListResult, error) {
	f.called = true
	if f.err != nil {
		return nil, store.TransactionListResult{}, f.err
	}
	return []store.Transaction{{ID: "tx-1", MessageID: "msg-1"}}, store.TransactionListResult{Total: 1, TotalAmount: 42}, nil
}

func TestInstrumentedTransactionStoreDelegatesSuccess(t *testing.T) {
	next := &fakeTransactionStore{}
	scope := observability.NewScope(slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	instrumented := store.NewInstrumentedTransactionStore(next, scope, slog.Default())

	rows, result, err := instrumented.ListTransactions(context.Background(), store.ListFilter{})
	if err != nil {
		t.Fatalf("ListTransactions() error = %v", err)
	}
	if !next.called {
		t.Fatal("next ListTransactions was not called")
	}
	if len(rows) != 1 || rows[0].ID != "tx-1" || result.Total != 1 || result.TotalAmount != 42 {
		t.Fatalf("unexpected response rows=%#v result=%#v", rows, result)
	}
}

func TestInstrumentedTransactionStoreDelegatesError(t *testing.T) {
	wantErr := errors.New("db down")
	next := &fakeTransactionStore{err: wantErr}
	scope := observability.NewScope(slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	instrumented := store.NewInstrumentedTransactionStore(next, scope, slog.Default())

	_, _, err := instrumented.ListTransactions(context.Background(), store.ListFilter{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ListTransactions() error = %v, want %v", err, wantErr)
	}
	if !next.called {
		t.Fatal("next ListTransactions was not called")
	}
}

func TestObservedStoreUsesProvidedClock(t *testing.T) {
	next := &fakeTransactionStore{}
	scope := observability.NewScope(slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	observed := store.NewInstrumentedTransactionStore(next, scope, slog.Default())
	observed.SetNowForTest(func() time.Time { return time.Unix(10, 0) })

	if _, _, err := observed.ListTransactions(context.Background(), store.ListFilter{}); err != nil {
		t.Fatalf("ListTransactions() error = %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
task test:be -- ./internal/store -run 'TestInstrumentedTransactionStore|TestObservedStore'
```

Expected: FAIL because interfaces/decorator do not exist.

- [ ] **Step 3: Create minimal capability interface**

Create `backend/internal/store/interfaces.go`:

```go
package store

import "context"

type TransactionStore interface {
	ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error)
}

var _ TransactionStore = (*Store)(nil)
```

- [ ] **Step 4: Implement instrumented transaction decorator**

Create `backend/internal/store/instrumented.go`:

```go
package store

import (
	"context"
	"log/slog"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/observability"
)

type InstrumentedTransactionStore struct {
	next   TransactionStore
	scope  *observability.Scope
	logger *slog.Logger
	now    func() time.Time
}

func NewInstrumentedTransactionStore(next TransactionStore, scope *observability.Scope, logger *slog.Logger) *InstrumentedTransactionStore {
	if logger == nil {
		logger = slog.Default()
	}
	if scope == nil {
		scope = observability.NewScope(logger, "store")
	}
	return &InstrumentedTransactionStore{
		next:   next,
		scope:  scope,
		logger: logger,
		now:    time.Now,
	}
}

func (s *InstrumentedTransactionStore) SetNowForTest(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *InstrumentedTransactionStore) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.list")
	defer span.End()

	start := s.now()
	rows, result, err := s.next.ListTransactions(ctx, f)
	s.scope.RecordOperation(ctx, observability.Operation{
		Namespace: "store",
		Name:      "transactions.list",
		Duration:  time.Since(start),
		Err:       err,
	})
	return rows, result, err
}
```

- [ ] **Step 5: Run tests**

Run:

```bash
task test:be -- ./internal/store -run 'TestInstrumentedTransactionStore|TestObservedStore'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/store/interfaces.go backend/internal/store/instrumented.go backend/internal/store/instrumented_test.go
git commit --no-gpg-sign -m "Add store instrumentation decorators"
```

---

### Task 5: Expand Store Decorators To API Storer Surface

**Files:**
- Modify: `backend/internal/store/interfaces.go`
- Modify: `backend/internal/store/instrumented.go`
- Modify: `backend/internal/store/instrumented_test.go`
- Modify: `backend/internal/api/store.go`

- [ ] **Step 1: Write compile-time assertions**

Append to `backend/internal/store/instrumented_test.go`:

```go
func TestInstrumentedStoreImplementsAPIStorerAtCompileTime(t *testing.T) {
	t.Skip("compile-time assertion lives in internal/api/store.go after wiring")
}
```

Modify `backend/internal/api/store.go` by adding an assertion near the existing one:

```go
var _ Storer = (*store.InstrumentedStore)(nil)
```

- [ ] **Step 2: Run compile test to verify it fails**

Run:

```bash
task test:be -- ./internal/api ./internal/store
```

Expected: FAIL because `store.InstrumentedStore` does not exist or does not implement `api.Storer`.

- [ ] **Step 3: Implement full facade decorator by delegation**

In `backend/internal/store/interfaces.go`, add:

```go
type FullStore interface {
	ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error)
	GetTransaction(ctx context.Context, id string) (*Transaction, error)
	UpdateDescription(ctx context.Context, id, description string) error
	AddLabel(ctx context.Context, transactionID, label string) error
	AddLabels(ctx context.Context, transactionID string, labels []string) error
	RemoveLabel(ctx context.Context, transactionID, label string) error
	SearchTransactions(ctx context.Context, query string, f ListFilter) ([]Transaction, TransactionListResult, error)
	GetStats(ctx context.Context, baseCurrency string) (*Stats, error)
	GetChartData(ctx context.Context) (*ChartData, error)
	GetDashboardData(ctx context.Context) (*DashboardData, error)
	GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error)
	GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error)
	GetAppConfig(ctx context.Context, key string) (string, error)
	SetAppConfig(ctx context.Context, key, value string) error
	IsMessageProcessed(ctx context.Context, key string) (bool, error)
	MarkMessageProcessed(ctx context.Context, key string, at time.Time) error
	SetActiveReader(ctx context.Context, reader string) error
	GetActiveReader(ctx context.Context) (string, error)
	SetReaderSecret(ctx context.Context, reader string, secret []byte) error
	GetReaderSecret(ctx context.Context, reader string) ([]byte, bool, error)
	SetReaderToken(ctx context.Context, reader string, token []byte) error
	GetReaderToken(ctx context.Context, reader string) ([]byte, bool, error)
	DeleteReaderToken(ctx context.Context, reader string) error
	SetReaderConfig(ctx context.Context, reader string, config json.RawMessage) error
	GetReaderConfig(ctx context.Context, reader string) (json.RawMessage, bool, error)
	DeleteReaderRuntime(ctx context.Context, reader string) error
	GetFacets(ctx context.Context) (*Facets, error)
	ListLabels(ctx context.Context) ([]Label, error)
	CreateLabel(ctx context.Context, name, color string) error
	UpdateLabel(ctx context.Context, name, color string) error
	DeleteLabel(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	RemoveLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	GetLabelMappings(ctx context.Context) (map[string][]string, error)
	GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*MonthlyBreakdownData, error)
	ListCategories(ctx context.Context) ([]Category, error)
	CreateCategory(ctx context.Context, name, description string) error
	DeleteCategory(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error)
	RemoveCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error)
	GetCategoryMappings(ctx context.Context) (map[string][]string, error)
	ListBuckets(ctx context.Context) ([]Bucket, error)
	CreateBucket(ctx context.Context, name, description string) error
	DeleteBucket(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error)
	RemoveBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error)
	GetBucketMappings(ctx context.Context) (map[string][]string, error)
	UpdateTransaction(ctx context.Context, id string, u TransactionUpdate) error
	MuteTransaction(ctx context.Context, id string, muted bool, reason string) error
	UpdateMuteReason(ctx context.Context, id, reason string) error
	MuteByMerchant(ctx context.Context, pattern, reason string) error
	UpdateMerchantReason(ctx context.Context, id, reason string) error
	ListMutedMerchants(ctx context.Context) ([]MutedMerchant, error)
	GetMutedMerchantsWithCount(ctx context.Context) ([]MutedMerchantWithCount, error)
	DeleteMutedMerchant(ctx context.Context, id string) error
	UnmuteByPattern(ctx context.Context, pattern string) error
	DeleteMutedMerchantAndUnmute(ctx context.Context, id string) error
	CategorizeMerchant(ctx context.Context, merchant, category, bucket string) (int, error)
	ListRules(ctx context.Context) ([]RuleRow, error)
	GetRule(ctx context.Context, id string) (*RuleRow, error)
	CreateRule(ctx context.Context, r RuleRow) (*RuleRow, error)
	UpdateRule(ctx context.Context, id string, r RuleRow) (*RuleRow, error)
	DeleteRule(ctx context.Context, id string) error
	SeedPredefinedRules(ctx context.Context, rules []RuleRow) error
	ImportUserRules(ctx context.Context, rules []RuleRow) error
	SeedMCCCodes(ctx context.Context, entries []MCCEntry) error
	SeedMerchantCategories(ctx context.Context, entries []MerchantCategoryEntry) (int, error)
	LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error)
	SeedMCCCategories(ctx context.Context, names []string) error
	GetSyncStatus(ctx context.Context) (SyncStatus, error)
	SetSyncStatus(ctx context.Context, status SyncStatus) error
	ListExtractionDiagnostics(ctx context.Context, filter DiagnosticFilter) ([]ExtractionDiagnosticRow, error)
	GetExtractionDiagnostic(ctx context.Context, id string) (*ExtractionDiagnosticRow, error)
	UpdateExtractionDiagnosticStatus(ctx context.Context, id, status string) (*ExtractionDiagnosticRow, error)
	RecordExtractionDiagnostic(ctx context.Context, diagnostic api.ExtractionDiagnostic) error
}
```

Add missing imports to `interfaces.go`:

```go
import (
	"context"
	"encoding/json"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)
```

In `backend/internal/store/instrumented.go`, create:

```go
type InstrumentedStore struct {
	next   FullStore
	scope  *observability.Scope
	logger *slog.Logger
	now    func() time.Time
}

func NewInstrumentedStore(next FullStore, scope *observability.Scope, logger *slog.Logger) *InstrumentedStore {
	if logger == nil {
		logger = slog.Default()
	}
	if scope == nil {
		scope = observability.NewScope(logger, "store")
	}
	return &InstrumentedStore{next: next, scope: scope, logger: logger, now: time.Now}
}

func (s *InstrumentedStore) observe(ctx context.Context, name string, fn func(context.Context) error) error {
	ctx, span := s.scope.Start(ctx, "store."+name)
	defer span.End()
	start := s.now()
	err := fn(ctx)
	s.scope.RecordOperation(ctx, observability.Operation{
		Namespace: "store",
		Name:      name,
		Duration:  time.Since(start),
		Err:       err,
	})
	return err
}
```

Then implement every `FullStore` method as a thin wrapper. Do not embed `FullStore` anonymously to satisfy the interface; keep `next FullStore` as a named field so missing wrapper methods are compile errors through `var _ Storer = (*store.InstrumentedStore)(nil)`.

Use this operation-name mapping:

- Transaction methods: `transactions.list`, `transactions.get`, `transactions.update_description`, `transactions.add_label`, `transactions.add_labels`, `transactions.remove_label`, `transactions.search`, `transactions.get_facets`, `transactions.update`, `transactions.mute`, `transactions.update_mute_reason`, `transactions.update_merchant_reason`, `transactions.mute_by_merchant`, `transactions.list_muted_merchants`, `transactions.get_muted_merchants_with_count`, `transactions.delete_muted_merchant`, `transactions.unmute_by_pattern`, `transactions.delete_muted_merchant_and_unmute`.
- Read model methods: `read_model.get_stats`, `read_model.get_chart_data`, `read_model.get_dashboard_data`, `read_model.get_spending_heatmap`, `read_model.get_annual_spend`, `read_model.get_monthly_breakdown_spend`.
- Runtime methods: `runtime.get_app_config`, `runtime.set_app_config`, `runtime.is_message_processed`, `runtime.mark_message_processed`, `runtime.set_active_reader`, `runtime.get_active_reader`, `runtime.set_reader_secret`, `runtime.get_reader_secret`, `runtime.set_reader_token`, `runtime.get_reader_token`, `runtime.delete_reader_token`, `runtime.set_reader_config`, `runtime.get_reader_config`, `runtime.delete_reader_runtime`, `runtime.get_sync_status`, `runtime.set_sync_status`.
- Taxonomy/community methods: `taxonomy.list_labels`, `taxonomy.create_label`, `taxonomy.update_label`, `taxonomy.delete_label`, `taxonomy.apply_label_by_merchant`, `taxonomy.remove_label_by_merchant`, `taxonomy.get_label_mappings`, `taxonomy.list_categories`, `taxonomy.create_category`, `taxonomy.delete_category`, `taxonomy.apply_category_by_merchant`, `taxonomy.remove_category_by_merchant`, `taxonomy.get_category_mappings`, `taxonomy.list_buckets`, `taxonomy.create_bucket`, `taxonomy.delete_bucket`, `taxonomy.apply_bucket_by_merchant`, `taxonomy.remove_bucket_by_merchant`, `taxonomy.get_bucket_mappings`, `community.categorize_merchant`, `community.seed_mcc_codes`, `community.seed_merchant_categories`, `community.load_category_snapshot`, `community.seed_mcc_categories`.
- Rule methods: `rules.list`, `rules.get`, `rules.create`, `rules.update`, `rules.delete`, `rules.seed_predefined`, `rules.import_user`.
- Diagnostic methods: `diagnostics.list_extraction`, `diagnostics.get_extraction`, `diagnostics.update_extraction_status`, `diagnostics.record_extraction`.

For example:

```go
func (s *InstrumentedStore) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	var out *Transaction
	err := s.observe(ctx, "transactions.get", func(ctx context.Context) error {
		var err error
		out, err = s.next.GetTransaction(ctx, id)
		return err
	})
	return out, err
}
```

For methods returning multiple values, assign to locals inside `observe` and return locals plus `err`.

- [ ] **Step 4: Run compile test**

Run:

```bash
task test:be -- ./internal/api ./internal/store
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/store/interfaces.go backend/internal/store/instrumented.go backend/internal/store/instrumented_test.go backend/internal/api/store.go
git commit --no-gpg-sign -m "Wrap store operations with observability"
```

---

### Task 6: Remove Inline Repository QueryInstrumentation

**Files:**
- Modify: `backend/internal/store/repository_dependencies.go`
- Modify: `backend/internal/store/community_repository.go`
- Modify: `backend/internal/store/diagnostics_repository.go`
- Modify: `backend/internal/store/label_repository.go`
- Modify: `backend/internal/store/read_model_repository.go`
- Modify: `backend/internal/store/rules_repository.go`
- Modify: `backend/internal/store/runtime_repository.go`
- Modify: `backend/internal/store/transactions_repository.go`
- Modify: `backend/internal/store/store.go`
- Delete: `backend/internal/store/instrumentation.go`
- Delete: `backend/internal/store/instrumentation_test.go`
- Modify: `backend/internal/store/store_repositories_test.go`

- [ ] **Step 1: Run existing repository instrumentation test to establish current behavior**

Run:

```bash
task test:be -- ./internal/store -run TestStoreRepositoriesEmitDebugInstrumentation
```

Expected: PASS before changes.

- [ ] **Step 2: Update repository instrumentation tests to decorator expectations**

In `backend/internal/store/store_repositories_test.go`, replace tests that assert operations like `runtime.get_reader_token` are logged by repository internals with tests that assert `InstrumentedStore` logs operation names.

Add this test:

```go
func TestInstrumentedStoreEmitsDebugOperationLog(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scope := observability.NewScope(logger, "test")
	ts := newTestStoreWithLogger(t, io.Discard)
	observed := store.NewInstrumentedStore(ts.Store, scope, logger)

	if _, _, err := observed.ListTransactions(context.Background(), store.ListFilter{}); err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if got := logs.String(); !strings.Contains(got, "operation=transactions.list") {
		t.Fatalf("logs = %q, want transactions.list operation", got)
	}
}
```

Add imports if missing:

```go
import (
	"io"
	"strings"

	"github.com/ArionMiles/expensor/backend/pkg/observability"
)
```

- [ ] **Step 3: Run test to verify it fails before cleanup**

Run:

```bash
task test:be -- ./internal/store -run TestInstrumentedStoreEmitsDebugOperationLog
```

Expected: FAIL if `InstrumentedStore` is incomplete or if test helpers need adjustment.

- [ ] **Step 4: Remove `QueryInstrumentation` from repositories**

Make these exact structural removals:

- In `backend/internal/store/repository_dependencies.go`, remove `metrics *QueryInstrumentation`.
- In `backend/internal/store/store.go`, remove the `metrics: NewQueryInstrumentation(s.logger),` assignment from repository dependency construction.
- In `backend/internal/store/read_model_repository.go`, remove the `metrics` field from `pgReadModelRepository`, remove the constructor fallback to `NewQueryInstrumentation`, and unwrap these methods so they directly call their existing query helpers: `GetStats`, `GetChartData`, `GetDashboardData`, `GetSpendingHeatmap`, `GetAnnualSpend`, `GetMonthlyBreakdownSpend`.
- In `backend/internal/store/transactions_repository.go`, remove the `metrics` field from `pgTransactionsRepository`, remove the constructor fallback, and unwrap: `ListTransactions`, `GetTransaction`, `UpdateDescription`, `AddLabel`, `AddLabels`, `RemoveLabel`, `SearchTransactions`, `GetFacets`, `UpdateTransaction`, `MuteTransaction`, `UpdateMuteReason`, `UpdateMerchantReason`, `MuteByMerchant`, `ListMutedMerchants`, `GetMutedMerchantsWithCount`, `DeleteMutedMerchant`, `UnmuteByPattern`, `DeleteMutedMerchantAndUnmute`, `GetMutedMerchantPatterns`.
- In `backend/internal/store/runtime_repository.go`, remove the `metrics` field from `pgRuntimeRepository`, remove the constructor fallback, and unwrap: `InitAppConfig`, `GetAppConfig`, `SetAppConfig`, `SetActiveReader`, `GetActiveReader`, `DeleteReaderToken`, `DeleteReaderRuntime`, `IsMessageProcessed`, `MarkMessageProcessed`, `GetSyncStatus`, `SetSyncStatus`, `GetCommunityURL`, `SetCommunityURL`, `setReaderJSON`, `getReaderJSON`.
- In `backend/internal/store/rules_repository.go`, remove the `metrics` field from `pgRulesRepository`, remove the constructor fallback, and unwrap: `InitRules`, `ListRules`, `GetRule`, `CreateRule`, `UpdateRule`, `DeleteRule`, `SeedPredefinedRules`, `ImportUserRules`.
- In `backend/internal/store/diagnostics_repository.go`, remove the `metrics` field from `pgDiagnosticsRepository`, remove the constructor fallback, and unwrap: `RecordExtractionDiagnostic`, `ListExtractionDiagnostics`, `GetExtractionDiagnostic`, `UpdateExtractionDiagnosticStatus`.
- In `backend/internal/store/community_repository.go`, remove the `metrics` field from `pgCommunityRepository`, remove the constructor fallback, and unwrap: `CategorizeMerchant`, `applyTaxonomyByMerchant`, `removeTaxonomyByMerchant`, `getTaxonomyMappings`, `SeedMCCCodes`, `SeedMerchantCategories`, `LoadCategorySnapshot`, `SeedMCCCategories`.
- In `backend/internal/store/label_repository.go`, remove the `metrics` field from `pgTaxonomyRepository`, remove constructor fallbacks and `NewQueryInstrumentation(logger)` creation, and unwrap: `InitLabels`, `InitCategoriesBuckets`, `ListLabels`, `CreateLabel`, `UpdateLabel`, `DeleteLabel`, `ApplyLabelByMerchant`, `RemoveLabelByMerchant`, `GetLabelMappings`, `listTaxonomyItems`, `CreateCategory`, `DeleteCategory`, `CreateBucket`, `DeleteBucket`.

Use the existing private helper methods where they already exist. For example, `ListTransactions` should become:

```go
func (r *pgTransactionsRepository) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error) {
	return r.listTransactionsQuery(ctx, f)
}
```

For methods that currently wrap inline SQL directly, move only the body of the `Observe` closure into the method. Do not change SQL strings or scanning behavior in this task.

After edits, this command must return no matches:

```bash
rg 'QueryInstrumentation|\\.metrics\\.Observe|NewQueryInstrumentation' backend/internal/store
```

Delete:

```bash
backend/internal/store/instrumentation.go
backend/internal/store/instrumentation_test.go
```

- [ ] **Step 5: Run store tests**

Run:

```bash
task test:be -- ./internal/store
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/store
git commit --no-gpg-sign -m "Move store instrumentation to decorators"
```

---

### Task 7: Wire Observability And Instrumented Store In Main

**Files:**
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/store/instrumented.go`

- [ ] **Step 1: Add compile-oriented wiring test if existing main tests cover startup helpers**

Run:

```bash
task test:be -- ./cmd/server
```

Expected: PASS before wiring.

- [ ] **Step 2: Wire `InstrumentedStore` into API and daemon coordinator**

In `backend/cmd/server/main.go`, after `pgStore` is initialized and before assigning `st`, create a scope and observed store:

```go
storeScope := observability.NewScope(logger.With("component", "store"), "github.com/ArionMiles/expensor/backend/internal/store")
observedStore := store.NewInstrumentedStore(pgStore, storeScope, logger.With("component", "store"))
var st httpapi.Storer = observedStore
```

Keep direct `pgStore` for methods not exposed through `httpapi.Storer`, such as `Close()` and startup seeding if seeding should not be instrumented yet.

- [ ] **Step 3: Run backend compile tests**

Run:

```bash
task test:be -- ./cmd/server ./internal/api ./internal/store
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/server/main.go backend/internal/store/instrumented.go
git commit --no-gpg-sign -m "Wire instrumented store into server"
```

---

### Task 8: Add AGENTS.md Backend Guidance

**Files:**
- Modify: `AGENTS.md`

- [ ] **Step 1: Add backend architecture guidance**

In `AGENTS.md`, under `## Architecture Patterns`, add:

```markdown
### Backend code health

Do not add optional plugin interfaces for required metadata. If every reader must provide it, put it in the main metadata struct.

Avoid long constructor signatures. Use a small input/deps struct once a constructor exceeds 4-5 parameters.

`internal/plugins.Registry` is a catalog, not an application assembler. Daemon/runtime wiring belongs outside the registry.

Define Go interfaces at consumer boundaries. Do not create package-local interfaces beside a single implementation unless a decorator or test boundary needs it.

Do not call instrumentation helpers from inside repository implementations. Wrap store/repository interfaces with decorators that own logging, metrics, and tracing, then delegate to the concrete implementation.

Keep concrete Postgres repositories focused on database behavior. New store behavior must live in the owning repository file, not in `internal/store/store.go`.

Do not add provider-specific helpers to `pkg/api`; Gmail/Thunderbird-specific behavior belongs in that reader package.

Do not use `context.Background()` inside request or daemon paths when a caller context is available.

Keep `slog` as the application logging API. Use OpenTelemetry for traces and metrics, not log export.

Do not put high-cardinality or sensitive values in metrics or trace attributes, including email bodies, snippets, sender addresses, message IDs, transaction IDs, merchant names, and raw SQL.
```

- [ ] **Step 2: Commit**

```bash
git add AGENTS.md
git commit --no-gpg-sign -m "Document backend architecture guardrails"
```

---

### Task 9: Final Verification

**Files:**
- No code changes expected.

- [ ] **Step 1: Format backend**

Run:

```bash
task fmt:be
```

Expected: completes successfully.

- [ ] **Step 2: Run backend tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 3: Run strict backend lint**

Run:

```bash
task lint:be:prod
```

Expected: `0 issues`.

- [ ] **Step 4: Check OpenAPI if handler/store public API changed**

Run:

```bash
task openapi:check
```

Expected: PASS or no generated diff.

- [ ] **Step 5: Inspect final diff**

Run:

```bash
git status --short
git log --oneline -8
```

Expected: clean worktree after the planned commits.
