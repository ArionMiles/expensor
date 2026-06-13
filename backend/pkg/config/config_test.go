package config_test

import (
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/config"
)

func TestLoadRequiresPostgresConnectionFields(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("POSTGRES_DB", "expensor")
	t.Setenv("POSTGRES_USER", "expensor")

	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "POSTGRES_HOST") {
		t.Fatalf("expected missing POSTGRES_HOST error, got %v", err)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	setRequiredConfigEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 8080 || cfg.BaseURL != "http://localhost:8080" || cfg.FrontendURL != cfg.BaseURL {
		t.Fatalf("server defaults: got port=%d base=%q frontend=%q", cfg.Port, cfg.BaseURL, cfg.FrontendURL)
	}
	if cfg.ScanInterval != 60 || cfg.LookbackDays != 180 {
		t.Fatalf("application defaults: got scan=%d lookback=%d", cfg.ScanInterval, cfg.LookbackDays)
	}
	if cfg.Postgres.Port != 5432 || cfg.Postgres.SSLMode != "disable" || cfg.Postgres.BatchSize != 10 ||
		cfg.Postgres.FlushInterval != 30 || cfg.Postgres.MaxPoolSize != 10 {
		t.Fatalf("postgres defaults: %#v", cfg.Postgres)
	}
	if cfg.Postgres.ConnectTimeout != 30*time.Second || cfg.Postgres.RetryInterval != 2*time.Second {
		t.Fatalf("postgres timing defaults: %#v", cfg.Postgres)
	}
	if cfg.Community.URL != "https://raw.githubusercontent.com/ArionMiles/expensor/main/content" ||
		cfg.Community.SyncInterval != 24*time.Hour || cfg.Community.SyncTimeout != 2*time.Minute {
		t.Fatalf("community defaults: %#v", cfg.Community)
	}
	if cfg.Persisted.ReadTimeout != 3*time.Second {
		t.Fatalf("app config read timeout: got %s", cfg.Persisted.ReadTimeout)
	}
	if cfg.Observability.LogLevel != slog.LevelInfo || cfg.Observability.LogJSON ||
		cfg.Observability.Enabled || cfg.Observability.Exporter != "none" {
		t.Fatalf("observability defaults: %#v", cfg.Observability)
	}
}

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("PORT", "9090")
	t.Setenv("BASE_URL", "https://api.example.com")
	t.Setenv("FRONTEND_URL", "https://app.example.com")
	t.Setenv("POSTGRES_CONNECT_TIMEOUT", "45s")
	t.Setenv("POSTGRES_RETRY_INTERVAL", "5s")
	t.Setenv("EXPENSOR_COMMUNITY_URL", "https://content.example.com")
	t.Setenv("EXPENSOR_CONTENT_SYNC_INTERVAL", "12h")
	t.Setenv("EXPENSOR_CONTENT_SYNC_TIMEOUT", "90s")
	t.Setenv("EXPENSOR_APP_CONFIG_READ_TIMEOUT", "7s")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_JSON", "true")
	t.Setenv("EXPENSOR_OBSERVABILITY_ENABLED", "true")
	t.Setenv("EXPENSOR_OBSERVABILITY_EXPORTER", "otlp")
	t.Setenv("EXPENSOR_OBSERVABILITY_OTLP_ENDPOINT", "collector:4317")
	t.Setenv("EXPENSOR_OBSERVABILITY_OTLP_INSECURE", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 9090 || cfg.BaseURL != "https://api.example.com" || cfg.FrontendURL != "https://app.example.com" {
		t.Fatalf("server overrides: got port=%d base=%q frontend=%q", cfg.Port, cfg.BaseURL, cfg.FrontendURL)
	}
	if cfg.Postgres.ConnectTimeout != 45*time.Second || cfg.Postgres.RetryInterval != 5*time.Second {
		t.Fatalf("postgres timing overrides: %#v", cfg.Postgres)
	}
	if cfg.Community.URL != "https://content.example.com" || cfg.Community.SyncInterval != 12*time.Hour ||
		cfg.Community.SyncTimeout != 90*time.Second {
		t.Fatalf("community overrides: %#v", cfg.Community)
	}
	if cfg.Persisted.ReadTimeout != 7*time.Second {
		t.Fatalf("app config read timeout: got %s", cfg.Persisted.ReadTimeout)
	}
	if cfg.Observability.LogLevel != slog.LevelDebug || !cfg.Observability.LogJSON ||
		!cfg.Observability.Enabled || cfg.Observability.Exporter != "otlp" ||
		cfg.Observability.OTLPEndpoint != "collector:4317" || !cfg.Observability.OTLPInsecure {
		t.Fatalf("observability overrides: %#v", cfg.Observability)
	}
}

func TestLoadRejectsInvalidObservabilityValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "log level", key: "LOG_LEVEL", value: "verbose"},
		{name: "JSON logging boolean", key: "LOG_JSON", value: "sometimes"},
		{name: "observability enabled boolean", key: "EXPENSOR_OBSERVABILITY_ENABLED", value: "sometimes"},
		{name: "OTLP insecure boolean", key: "EXPENSOR_OBSERVABILITY_OTLP_INSECURE", value: "sometimes"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setRequiredConfigEnv(t)
			t.Setenv(tc.key, tc.value)

			if _, err := config.Load(); err == nil {
				t.Fatalf("Load accepted %s=%q", tc.key, tc.value)
			}
		})
	}
}

func TestLoadRejectsNegativePostgresMaxPoolSize(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("POSTGRES_MAX_POOL_SIZE", "-1")

	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "POSTGRES_MAX_POOL_SIZE") {
		t.Fatalf("expected invalid POSTGRES_MAX_POOL_SIZE error, got %v", err)
	}
}

func TestThunderbirdConfig_GetMailboxes(t *testing.T) {
	tests := []struct {
		name      string
		mailboxes string
		want      []string
	}{
		{name: "empty string returns empty slice", mailboxes: "", want: []string{}},
		{name: "single mailbox", mailboxes: "INBOX", want: []string{"INBOX"}},
		{name: "multiple mailboxes", mailboxes: "INBOX,Archives,Sent", want: []string{"INBOX", "Archives", "Sent"}},
		{name: "spaces around commas are trimmed", mailboxes: "INBOX , Archives , Sent", want: []string{"INBOX", "Archives", "Sent"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Thunderbird{Mailboxes: tc.mailboxes}
			got := cfg.GetMailboxes()
			if len(got) != len(tc.want) {
				t.Fatalf("len: got %d, want %d — got %v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func setRequiredConfigEnv(t *testing.T) {
	t.Helper()
	clearConfigEnv(t)
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_DB", "expensor")
	t.Setenv("POSTGRES_USER", "expensor")
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PORT",
		"BASE_URL",
		"FRONTEND_URL",
		"EXPENSOR_SCAN_INTERVAL",
		"EXPENSOR_LOOKBACK_DAYS",
		"EXPENSOR_STATIC_DIR",
		"EXPENSOR_COMMUNITY_URL",
		"EXPENSOR_CONTENT_SYNC_INTERVAL",
		"EXPENSOR_CONTENT_SYNC_TIMEOUT",
		"EXPENSOR_APP_CONFIG_READ_TIMEOUT",
		"LOG_LEVEL",
		"LOG_JSON",
		"EXPENSOR_OBSERVABILITY_ENABLED",
		"EXPENSOR_OBSERVABILITY_EXPORTER",
		"EXPENSOR_OBSERVABILITY_OTLP_ENDPOINT",
		"EXPENSOR_OBSERVABILITY_OTLP_INSECURE",
		"THUNDERBIRD_DATA_DIR",
		"POSTGRES_HOST",
		"POSTGRES_PORT",
		"POSTGRES_DB",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"POSTGRES_SSLMODE",
		"POSTGRES_BATCH_SIZE",
		"POSTGRES_FLUSH_INTERVAL",
		"POSTGRES_MAX_POOL_SIZE",
		"POSTGRES_CONNECT_TIMEOUT",
		"POSTGRES_RETRY_INTERVAL",
	} {
		value, exists := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		t.Cleanup(func() {
			if exists {
				_ = os.Setenv(key, value)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
}
