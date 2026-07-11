package config_test

import (
	"bytes"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/config"
)

func TestLoadLeavesDatabaseBackendEmptyWhenUnset(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("EXPENSOR_SECRET_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Database.Backend != "" {
		t.Fatalf("Database.Backend = %q, want empty", cfg.Database.Backend)
	}
}

func TestLoadRequiresPostgresConnectionFieldsWhenPostgresConfigured(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("EXPENSOR_DB_BACKEND", "postgres")
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
	if cfg.Database.Backend != config.DatabaseBackendPostgres || cfg.Database.BatchSize != 10 || cfg.Database.FlushInterval != 30 {
		t.Fatalf("database defaults: %#v", cfg.Database)
	}
	if cfg.Database.Postgres.Port != 5432 || cfg.Database.Postgres.SSLMode != "disable" || cfg.Database.Postgres.MaxPoolSize != 10 {
		t.Fatalf("postgres defaults: %#v", cfg.Database.Postgres)
	}
	if cfg.Community.URL != "https://raw.githubusercontent.com/ArionMiles/expensor/main/backend/internal/catalog/content" ||
		cfg.Community.SyncInterval != 24*time.Hour || cfg.Community.SyncTimeout != 2*time.Minute {
		t.Fatalf("community defaults: %#v", cfg.Community)
	}
	if cfg.Persisted.ReadTimeout != 3*time.Second {
		t.Fatalf("app config read timeout: got %s", cfg.Persisted.ReadTimeout)
	}
	if cfg.Security.SessionTTL != 168*time.Hour || cfg.Security.SetupTokenTTL != 24*time.Hour {
		t.Fatalf("security defaults: %#v", cfg.Security)
	}
	if cfg.Observability.LogLevel != slog.LevelInfo || cfg.Observability.LogJSON ||
		cfg.Observability.Enabled || cfg.Observability.Exporter != "none" {
		t.Fatalf("observability defaults: %#v", cfg.Observability)
	}
}

func TestLoadAuthEncryptionKey(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("EXPENSOR_SECRET_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)))

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.Security.SecretKey) != 32 {
		t.Fatalf("SecretKey length = %d, want 32", len(cfg.Security.SecretKey))
	}
}

func TestLoadRequiresAuthEncryptionKey(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("EXPENSOR_DB_BACKEND", "postgres")
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_DB", "expensor")
	t.Setenv("POSTGRES_USER", "expensor")

	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "EXPENSOR_SECRET_KEY") {
		t.Fatalf("expected missing EXPENSOR_SECRET_KEY error, got %v", err)
	}
}

func TestLoadAuthEncryptionKeyFile(t *testing.T) {
	setRequiredPostgresEnv(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "expensor_secret_key")
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))+"\n"), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	t.Setenv("EXPENSOR_SECRET_KEY_FILE", keyPath)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.Security.SecretKey) != 32 {
		t.Fatalf("SecretKey length = %d, want 32", len(cfg.Security.SecretKey))
	}
}

func TestLoadRejectsBothSecretKeyInputs(t *testing.T) {
	setRequiredConfigEnv(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "expensor_secret_key")
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	t.Setenv("EXPENSOR_SECRET_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)))
	t.Setenv("EXPENSOR_SECRET_KEY_FILE", keyPath)

	if _, err := config.Load(); err == nil {
		t.Fatal("Load() succeeded with both EXPENSOR_SECRET_KEY and EXPENSOR_SECRET_KEY_FILE")
	}
}

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("PORT", "9090")
	t.Setenv("BASE_URL", "https://api.example.com")
	t.Setenv("FRONTEND_URL", "https://app.example.com")
	t.Setenv("EXPENSOR_DB_BATCH_SIZE", "25")
	t.Setenv("EXPENSOR_DB_FLUSH_INTERVAL", "45")
	t.Setenv("EXPENSOR_COMMUNITY_URL", "https://content.example.com")
	t.Setenv("EXPENSOR_CONTENT_SYNC_INTERVAL", "12h")
	t.Setenv("EXPENSOR_CONTENT_SYNC_TIMEOUT", "90s")
	t.Setenv("EXPENSOR_APP_CONFIG_READ_TIMEOUT", "7s")
	t.Setenv("EXPENSOR_SESSION_TTL", "72h")
	t.Setenv("EXPENSOR_SETUP_TOKEN_TTL", "12h")
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
	if cfg.Database.BatchSize != 25 || cfg.Database.FlushInterval != 45 {
		t.Fatalf("database overrides: %#v", cfg.Database)
	}
	if cfg.Community.URL != "https://content.example.com" || cfg.Community.SyncInterval != 12*time.Hour ||
		cfg.Community.SyncTimeout != 90*time.Second {
		t.Fatalf("community overrides: %#v", cfg.Community)
	}
	if cfg.Persisted.ReadTimeout != 7*time.Second {
		t.Fatalf("app config read timeout: got %s", cfg.Persisted.ReadTimeout)
	}
	if cfg.Security.SessionTTL != 72*time.Hour || cfg.Security.SetupTokenTTL != 12*time.Hour {
		t.Fatalf("security overrides: %#v", cfg.Security)
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

func TestLoadReadsTOMLConfigFile(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	keyPath := filepath.Join(dir, "secret_key")
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32))), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	toml := `
port = 9091
base_url = "https://api.toml.example"
scan_interval = 120

[database]
backend = "postgres"
batch_size = 20
flush_interval = 15

[database.postgres]
host = "db"
database = "expensor_toml"
user = "toml_user"
password = "toml_password"

[security]
secret_key_file = "` + filepath.ToSlash(keyPath) + `"
`
	if err := os.WriteFile(configPath, []byte(toml), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv("EXPENSOR_CONFIG_FILE", configPath)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 9091 || cfg.BaseURL != "https://api.toml.example" || cfg.ScanInterval != 120 {
		t.Fatalf("server TOML values not applied: %#v", cfg)
	}
	if cfg.Database.Backend != config.DatabaseBackendPostgres || cfg.Database.BatchSize != 20 || cfg.Database.FlushInterval != 15 {
		t.Fatalf("database TOML values not applied: %#v", cfg.Database)
	}
	if cfg.Database.Postgres.Host != "db" || cfg.Database.Postgres.Database != "expensor_toml" || cfg.Database.Postgres.User != "toml_user" ||
		cfg.Database.Postgres.Password != "toml_password" {
		t.Fatalf("postgres TOML values not applied: %#v", cfg.Database.Postgres)
	}
}

func TestLoadEnvironmentOverridesTOMLConfigFile(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	keyPath := filepath.Join(dir, "secret_key")
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32))), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	toml := `
port = 9091

[database]
backend = "postgres"
batch_size = 20

[database.postgres]
host = "db"
database = "expensor_toml"
user = "toml_user"

[security]
secret_key_file = "` + filepath.ToSlash(keyPath) + `"
`
	if err := os.WriteFile(configPath, []byte(toml), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv("EXPENSOR_CONFIG_FILE", configPath)
	t.Setenv("PORT", "9092")
	t.Setenv("EXPENSOR_DB_BATCH_SIZE", "30")
	t.Setenv("POSTGRES_HOST", "env-db")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 9092 {
		t.Fatalf("Port = %d, want env override 9092", cfg.Port)
	}
	if cfg.Database.BatchSize != 30 {
		t.Fatalf("Database.BatchSize = %d, want env override 30", cfg.Database.BatchSize)
	}
	if cfg.Database.Postgres.Host != "env-db" {
		t.Fatalf("Postgres.Host = %q, want env override env-db", cfg.Database.Postgres.Host)
	}
}

func TestLoadUsesXDGConfigHomeFallback(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "secret_key")
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32))), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	configDir := filepath.Join(dir, "xdg", "expensor")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	toml := `
[database]
backend = "postgres"

[database.postgres]
host = "xdg-db"
database = "expensor_xdg"
user = "xdg_user"

[security]
secret_key_file = "` + filepath.ToSlash(keyPath) + `"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(toml), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Database.Postgres.Host != "xdg-db" {
		t.Fatalf("Postgres.Host = %q, want xdg-db", cfg.Database.Postgres.Host)
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
	setRequiredPostgresEnv(t)
	t.Setenv("EXPENSOR_SECRET_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
}

func setRequiredPostgresEnv(t *testing.T) {
	t.Helper()
	clearConfigEnv(t)
	t.Setenv("EXPENSOR_DB_BACKEND", "postgres")
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
		"EXPENSOR_CONFIG_FILE",
		"EXPENSOR_DB_BACKEND",
		"EXPENSOR_DB_BATCH_SIZE",
		"EXPENSOR_DB_FLUSH_INTERVAL",
		"EXPENSOR_SQLITE_PATH",
		"EXPENSOR_SQLITE_BUSY_TIMEOUT",
		"EXPENSOR_COMMUNITY_URL",
		"EXPENSOR_CONTENT_SYNC_INTERVAL",
		"EXPENSOR_CONTENT_SYNC_TIMEOUT",
		"EXPENSOR_APP_CONFIG_READ_TIMEOUT",
		"EXPENSOR_SECRET_KEY",
		"EXPENSOR_SECRET_KEY_FILE",
		"EXPENSOR_SESSION_TTL",
		"EXPENSOR_SETUP_TOKEN_TTL",
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
