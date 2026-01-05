// Package logging provides structured logging configuration using log/slog.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Config holds logging configuration options.
type Config struct {
	// Level is the minimum log level to output.
	Level slog.Level
	// JSON enables JSON output format (for production).
	JSON bool
	// Output is the writer to write logs to. Defaults to os.Stderr.
	Output io.Writer
}

// DefaultConfig returns a default logging configuration suitable for development.
// It reads the LOG_LEVEL environment variable to set the logging level.
// Valid values: DEBUG, INFO, WARN, ERROR. Defaults to INFO.
func DefaultConfig() Config {
	level := slog.LevelInfo
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		level = parseLogLevel(logLevel)
	}

	return Config{
		Level:  level,
		JSON:   false,
		Output: os.Stderr,
	}
}

// parseLogLevel converts a string log level to slog.Level.
func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ProductionConfig returns a logging configuration suitable for production.
func ProductionConfig() Config {
	return Config{
		Level:  slog.LevelInfo,
		JSON:   true,
		Output: os.Stderr,
	}
}

// Setup initializes the default slog logger with the given configuration.
func Setup(cfg Config) *slog.Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}

	opts := &slog.HandlerOptions{
		Level: cfg.Level,
	}

	var handler slog.Handler
	if cfg.JSON {
		handler = slog.NewJSONHandler(cfg.Output, opts)
	} else {
		handler = slog.NewTextHandler(cfg.Output, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}
