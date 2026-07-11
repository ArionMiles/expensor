package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/app"
	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const shutdownTimeout = 10 * time.Second

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		return 1
	}

	observabilityRuntime, err := observability.Setup(context.Background(), cfg.Observability)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize observability: %v\n", err)
		return 1
	}
	logger := observabilityRuntime.Logger
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := observabilityRuntime.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to shutdown observability", "error", err)
		}
	}()

	ctx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	application, err := app.New(ctx, app.Options{Config: cfg, Logger: logger, LogLevel: observabilityRuntime.LogLevel})
	if err != nil {
		logger.Error("failed to initialize application", "error", err)
		return 1
	}

	runErr := application.Run(ctx)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := application.Close(shutdownCtx); err != nil {
		logger.Warn("application shutdown incomplete", "error", err)
	}
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		logger.Error("application stopped with error", "error", runErr)
		return 1
	}
	logger.Info("shutdown complete")
	return 0
}
