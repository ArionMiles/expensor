package postgres_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	postgresplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/writers/postgres"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestPlugin_Metadata(t *testing.T) {
	p := &postgresplugin.Plugin{}
	metadata := p.Metadata()
	if metadata.Name != "postgres" {
		t.Errorf("Name: got %q, want \"postgres\"", metadata.Name)
	}
	if metadata.Description == "" {
		t.Error("Description: got empty string, want non-empty")
	}
	if len(metadata.RequiredScopes) != 0 {
		t.Errorf("RequiredScopes: got %v, want empty slice", metadata.RequiredScopes)
	}
}

func TestPlugin_NewWriter_ConnectionFailure(t *testing.T) {
	p := &postgresplugin.Plugin{}
	cfg := &config.Config{
		Postgres: config.PostgresConfig{
			Host:          "nonexistent-host-12345",
			Port:          5432,
			Database:      "test",
			User:          "test",
			Password:      "test",
			SSLMode:       "disable",
			BatchSize:     10,
			FlushInterval: 30,
			MaxPoolSize:   5,
		},
	}

	_, err := p.NewWriter(plugins.WriterInput{
		AppConfig: cfg,
		Logger:    testLogger(),
	})
	if err == nil {
		t.Error("expected error connecting to nonexistent host, got nil")
	}
}
