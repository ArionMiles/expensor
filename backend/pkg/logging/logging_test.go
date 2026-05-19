package logging_test

import (
	"log/slog"
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/logging"
)

func TestDefaultConfig(t *testing.T) {
	cfg := logging.DefaultConfig()

	if cfg.Level != slog.LevelInfo {
		t.Errorf("Level: got %v, want %v", cfg.Level, slog.LevelInfo)
	}
	if cfg.JSON {
		t.Error("JSON: got true, want false")
	}
	if cfg.Output == nil {
		t.Error("Output: got nil, want non-nil")
	}
}

func TestDefaultConfig_WithEnvVar(t *testing.T) {
	tests := []struct {
		envVal string
		want   slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"invalid", slog.LevelInfo},
		{"debug", slog.LevelDebug}, // case-insensitive
	}

	for _, tc := range tests {
		t.Run(tc.envVal, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tc.envVal)
			cfg := logging.DefaultConfig()
			if cfg.Level != tc.want {
				t.Errorf("Level: got %v, want %v", cfg.Level, tc.want)
			}
		})
	}
}

func TestProductionConfig(t *testing.T) {
	cfg := logging.ProductionConfig()

	if cfg.Level != slog.LevelInfo {
		t.Errorf("Level: got %v, want %v", cfg.Level, slog.LevelInfo)
	}
	if !cfg.JSON {
		t.Error("JSON: got false, want true")
	}
	if cfg.Output == nil {
		t.Error("Output: got nil, want non-nil")
	}
}

func TestSetup(t *testing.T) {
	cfg := logging.DefaultConfig()
	logger := logging.Setup(cfg)

	if logger == nil {
		t.Error("Setup: got nil logger")
	}
}

func TestSetup_NilOutputDefaultsToStderr(t *testing.T) {
	cfg := logging.Config{
		Level:  slog.LevelInfo,
		JSON:   false,
		Output: nil, // should default to os.Stderr
	}
	logger := logging.Setup(cfg)
	if logger == nil {
		t.Error("Setup with nil Output: got nil logger")
	}
}
