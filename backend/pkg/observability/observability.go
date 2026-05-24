package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Shutdown releases observability resources.
type Shutdown func(context.Context) error

// Setup initializes logging and telemetry providers.
func Setup(_ context.Context, cfg Config) (Shutdown, *slog.Logger, error) {
	logger := setupLogger(cfg)
	slog.SetDefault(logger)

	return func(context.Context) error {
		return nil
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
