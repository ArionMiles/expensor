package postgres_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/config"
	postgresplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/writers/postgres"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestPlugin_Name(t *testing.T) {
	p := &postgresplugin.Plugin{}
	if got := p.Name(); got != "postgres" {
		t.Errorf("Name: got %q, want \"postgres\"", got)
	}
}

func TestPlugin_Description(t *testing.T) {
	p := &postgresplugin.Plugin{}
	if got := p.Description(); got == "" {
		t.Error("Description: got empty string, want non-empty")
	}
}

func TestPlugin_RequiredScopes(t *testing.T) {
	p := &postgresplugin.Plugin{}
	scopes := p.RequiredScopes()
	if len(scopes) != 0 {
		t.Errorf("RequiredScopes: got %v, want empty slice", scopes)
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

	_, err := p.NewWriter(nil, cfg, testLogger())
	if err == nil {
		t.Error("expected error connecting to nonexistent host, got nil")
	}
}
