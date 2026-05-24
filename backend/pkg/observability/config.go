package observability

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Exporter identifies the telemetry exporter backend.
type Exporter string

const (
	ExporterNone   Exporter = "none"
	ExporterStdout Exporter = "stdout"
	ExporterOTLP   Exporter = "otlp"
)

// Config holds logging and telemetry configuration.
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

// DefaultConfig returns the default observability configuration for development.
func DefaultConfig() Config {
	enabled := envBool("EXPENSOR_OBSERVABILITY_ENABLED")
	exporter := Exporter(strings.ToLower(strings.TrimSpace(os.Getenv("EXPENSOR_OBSERVABILITY_EXPORTER"))))
	if exporter == "" || !enabled {
		exporter = ExporterNone
	}

	return Config{
		ServiceName:    "expensor",
		ServiceVersion: "dev",
		LogLevel:       parseLogLevel(os.Getenv("LOG_LEVEL")),
		LogJSON:        false,
		Output:         os.Stderr,
		Enabled:        enabled,
		Exporter:       exporter,
		OTLPEndpoint:   strings.TrimSpace(os.Getenv("EXPENSOR_OBSERVABILITY_OTLP_ENDPOINT")),
		OTLPInsecure:   envBool("EXPENSOR_OBSERVABILITY_OTLP_INSECURE"),
		TracesEnabled:  enabled,
		MetricsEnabled: enabled,
	}
}

// ProductionConfig returns a production-oriented logging configuration with telemetry disabled.
func ProductionConfig() Config {
	cfg := DefaultConfig()
	cfg.LogLevel = slog.LevelInfo
	cfg.LogJSON = true
	cfg.Enabled = false
	cfg.Exporter = ExporterNone
	cfg.TracesEnabled = false
	cfg.MetricsEnabled = false
	return cfg
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
