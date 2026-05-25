package observability

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// Shutdown releases observability resources.
type Shutdown func(context.Context) error

// Setup initializes logging and telemetry providers.
func Setup(ctx context.Context, cfg Config) (Shutdown, *slog.Logger, error) {
	logger := setupLogger(cfg)
	slog.SetDefault(logger)

	if !cfg.Enabled || cfg.Exporter == ExporterNone {
		return noopShutdown, logger, nil
	}
	if cfg.Exporter != ExporterOTLP {
		return nil, nil, fmt.Errorf("unsupported observability exporter %q", cfg.Exporter)
	}

	res, err := newResource(cfg)
	if err != nil {
		return nil, nil, err
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
	if len(shutdowns) == 0 {
		return noopShutdown, logger, nil
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

	opts := &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}
	if cfg.LogJSON {
		return slog.New(slog.NewJSONHandler(output, opts))
	}
	return slog.New(slog.NewTextHandler(output, opts))
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func noopShutdown(context.Context) error {
	return nil
}

func newResource(cfg Config) (*resource.Resource, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating telemetry resource: %w", err)
	}
	return res, nil
}

func newTracerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
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
}

func newMeterProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
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
}

// Scope groups telemetry instruments and logging for a package or subsystem.
type Scope struct {
	logger *slog.Logger
	tracer trace.Tracer
	meter  metric.Meter
}

// NewScope creates an observability scope.
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

// Start starts a span in the scope.
func (s *Scope) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return s.tracer.Start(ctx, name, opts...)
}
