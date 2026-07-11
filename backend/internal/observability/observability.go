package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/ArionMiles/expensor/backend/pkg/config"
)

// Exporter identifies the telemetry exporter backend.
type Exporter string

const (
	ExporterNone Exporter = "none"
	ExporterOTLP Exporter = "otlp"
)

// Shutdown releases observability resources.
type Shutdown func(context.Context) error

// SetupResult contains initialized observability runtime dependencies.
type SetupResult struct {
	Shutdown Shutdown
	Logger   *slog.Logger
	LogLevel *slog.LevelVar
}

// Setup initializes logging and telemetry providers.
func Setup(ctx context.Context, cfg config.Observability) (SetupResult, error) {
	logLevel := new(slog.LevelVar)
	logLevel.Set(cfg.LogLevel)
	logger := setupLogger(cfg, logLevel)
	slog.SetDefault(logger)

	exporter := Exporter(strings.ToLower(strings.TrimSpace(cfg.Exporter)))
	if !cfg.Enabled || exporter == ExporterNone {
		return SetupResult{Shutdown: noopShutdown, Logger: logger, LogLevel: logLevel}, nil
	}
	if exporter != ExporterOTLP {
		return SetupResult{}, fmt.Errorf("unsupported observability exporter %q", cfg.Exporter)
	}

	res, err := newResource(cfg)
	if err != nil {
		return SetupResult{}, err
	}

	tp, err := newTracerProvider(ctx, cfg, res)
	if err != nil {
		return SetupResult{}, err
	}
	otel.SetTracerProvider(tp)

	mp, err := newMeterProvider(ctx, cfg, res)
	if err != nil {
		return SetupResult{}, err
	}
	otel.SetMeterProvider(mp)

	shutdowns := []Shutdown{tp.Shutdown, mp.Shutdown}

	return SetupResult{
		Shutdown: func(ctx context.Context) error {
			var joined error
			for _, shutdown := range shutdowns {
				joined = errors.Join(joined, shutdown(ctx))
			}
			return joined
		},
		Logger:   logger,
		LogLevel: logLevel,
	}, nil
}

func setupLogger(cfg config.Observability, logLevel *slog.LevelVar) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	if cfg.LogJSON {
		return slog.New(slog.NewJSONHandler(cfg.Output, opts))
	}
	return slog.New(slog.NewTextHandler(cfg.Output, opts))
}

func noopShutdown(context.Context) error {
	return nil
}

func newResource(cfg config.Observability) (*resource.Resource, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.Version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating telemetry resource: %w", err)
	}
	return res, nil
}

func newTracerProvider(ctx context.Context, cfg config.Observability, res *resource.Resource) (*sdktrace.TracerProvider, error) {
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

func newMeterProvider(ctx context.Context, cfg config.Observability, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
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
