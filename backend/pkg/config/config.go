// Package config provides application configuration loaded from a TOML file and environment variables.
package config

import (
	"encoding/base64"
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	apperrors "github.com/ArionMiles/expensor/backend/pkg/errors"
)

const ServiceName = "expensor"

// Version is set at build time via -ldflags.
var Version = "dev"

type DatabaseBackend string

const (
	DatabaseBackendPostgres DatabaseBackend = "postgres"
	DatabaseBackendSQLite   DatabaseBackend = "sqlite"
)

// App holds the application configuration loaded from a TOML file and environment variables.
// Reader plugin selection is driven by the web UI, not env vars.
type App struct {
	// Port is the HTTP server port.
	// Environment variable: PORT
	// Default: 8080
	Port int `toml:"port"`

	// BaseURL is the public-facing base URL of the server, used as the OAuth
	// redirect URI. Set this when hosting on a local network or remote server.
	// Environment variable: BASE_URL
	// Default: http://localhost:<PORT>
	BaseURL string `toml:"base_url"`

	// FrontendURL is the URL used for post-auth redirects (e.g. after OAuth).
	// Defaults to BaseURL — only override this for local development when the
	// frontend Vite dev server runs on a different port.
	// Environment variable: FRONTEND_URL
	// Default: same as BASE_URL
	FrontendURL string `toml:"frontend_url"`

	// ScanInterval is the polling interval in seconds for all readers.
	// Environment variable: EXPENSOR_SCAN_INTERVAL
	// Default: 60
	ScanInterval int `toml:"scan_interval"`

	// LookbackDays limits how far back readers search for emails on first run.
	// Environment variable: EXPENSOR_LOOKBACK_DAYS
	// Default: 180
	LookbackDays int `toml:"lookback_days"`

	// StaticDir is an optional path to serve static frontend files from disk
	// instead of the embedded assets. Useful for development.
	// Environment variable: EXPENSOR_STATIC_DIR
	// Default: "" (use embedded assets)
	StaticDir string `toml:"static_dir"`

	// Runtime-only scanning checkpoint fields.
	// These are set programmatically by the daemon coordinator and are NOT
	// loaded from environment variables.
	LastScanAt    *time.Time      `toml:"-"`
	ForceFullScan bool            `toml:"-"`
	RunOnce       bool            `toml:"-"`
	OnCheckpoint  func(time.Time) `toml:"-"`

	// Thunderbird reader configuration (profile path/mailboxes set via UI wizard).
	Thunderbird Thunderbird `toml:"thunderbird"`

	Database      Database      `toml:"database"`
	SQLite        SQLite        `toml:"sqlite"`
	Postgres      Postgres      `toml:"postgres"`
	Community     Community     `toml:"community"`
	Persisted     Persisted     `toml:"persisted"`
	Security      Security      `toml:"security"`
	Observability Observability `toml:"observability"`
}

// Thunderbird holds Thunderbird reader configuration.
type Thunderbird struct {
	// ProfilePath is the path to the Thunderbird profile directory.
	// Set via the web UI onboarding wizard; not loaded from env vars.
	ProfilePath string `toml:"-"`

	// Mailboxes is a comma-separated list of mailbox names to scan.
	// Set via the web UI onboarding wizard; not loaded from env vars.
	Mailboxes string `toml:"-"`

	// DataDir is an optional extra path hinting where Thunderbird profile
	// directories can be found (used by the profile discovery endpoint).
	// Useful in Docker when a custom profile mount doesn't match platform defaults.
	// Environment variable: THUNDERBIRD_DATA_DIR
	DataDir string `toml:"data_dir"`
}

// GetMailboxes returns the mailboxes as a slice.
func (c *Thunderbird) GetMailboxes() []string {
	if c.Mailboxes == "" {
		return []string{}
	}
	mailboxes := strings.Split(c.Mailboxes, ",")
	for i, m := range mailboxes {
		mailboxes[i] = strings.TrimSpace(m)
	}
	return mailboxes
}

type Database struct {
	Backend           DatabaseBackend `toml:"backend"`
	BackendConfigured bool            `toml:"-"`
	BatchSize         int             `toml:"batch_size"`
	FlushInterval     int             `toml:"flush_interval"`
}

type SQLite struct {
	Path        string        `toml:"path"`
	BusyTimeout time.Duration `toml:"busy_timeout"`
}

// Postgres holds PostgreSQL connection configuration.
type Postgres struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Database string `toml:"database"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	SSLMode  string `toml:"sslmode"`

	// MaxPoolSize is the maximum number of connections in the pool.
	// Environment variable: POSTGRES_MAX_POOL_SIZE
	// Default: 10
	MaxPoolSize int32 `toml:"max_pool_size"`

	// ConnectTimeout is the maximum time to wait for PostgreSQL at startup.
	ConnectTimeout time.Duration `toml:"connect_timeout"`

	// RetryInterval is the delay between PostgreSQL startup connection attempts.
	RetryInterval time.Duration `toml:"retry_interval"`
}

// Community controls community content synchronization.
type Community struct {
	URL          string        `toml:"url"`
	SyncInterval time.Duration `toml:"sync_interval"`
	SyncTimeout  time.Duration `toml:"sync_timeout"`
}

// Persisted controls reads from the persisted application configuration.
type Persisted struct {
	ReadTimeout time.Duration `toml:"read_timeout"`
}

// Security controls authentication and credential encryption settings.
type Security struct {
	SecretKey     []byte        `toml:"-"`
	SecretKeyFile string        `toml:"secret_key_file"`
	SessionTTL    time.Duration `toml:"session_ttl"`
	SetupTokenTTL time.Duration `toml:"setup_token_ttl"`
}

// Observability controls application logging and telemetry export.
type Observability struct {
	LogLevel     slog.Level `toml:"log_level"`
	LogJSON      bool       `toml:"log_json"`
	Enabled      bool       `toml:"enabled"`
	Exporter     string     `toml:"exporter"`
	OTLPEndpoint string     `toml:"otlp_endpoint"`
	OTLPInsecure bool       `toml:"otlp_insecure"`
	Output       io.Writer  `toml:"-"`
}

// Load reads application configuration from a TOML file and environment variables.
func Load() (App, error) {
	cfg := defaults()
	if err := loadConfigFile(&cfg); err != nil {
		return App{}, err
	}
	if err := applyEnvOverrides(&cfg); err != nil {
		return App{}, err
	}
	if cfg.Database.Backend == "" {
		cfg.Database.Backend = DatabaseBackendSQLite
	}
	if err := validate(&cfg); err != nil {
		return App{}, err
	}
	if err := loadSecretKey(&cfg.Security); err != nil {
		return App{}, err
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
	}
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = cfg.BaseURL
	}
	cfg.Observability.Output = os.Stderr
	return cfg, nil
}

func defaults() App {
	return App{
		Port:         8080,
		ScanInterval: 60,
		LookbackDays: 180,
		Database:     Database{BatchSize: 10, FlushInterval: 30},
		SQLite:       SQLite{BusyTimeout: 5 * time.Second},
		Postgres:     Postgres{Port: 5432, SSLMode: "disable", MaxPoolSize: 10, ConnectTimeout: 30 * time.Second, RetryInterval: 2 * time.Second},
		Community: Community{
			URL:          "https://raw.githubusercontent.com/ArionMiles/expensor/main/backend/cmd/server/content",
			SyncInterval: 24 * time.Hour,
			SyncTimeout:  2 * time.Minute,
		},
		Persisted:     Persisted{ReadTimeout: 3 * time.Second},
		Security:      Security{SessionTTL: 168 * time.Hour, SetupTokenTTL: 24 * time.Hour},
		Observability: Observability{LogLevel: slog.LevelInfo, Exporter: "none"},
	}
}

func loadConfigFile(cfg *App) error {
	path, explicit := configFilePath()
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		if stderrors.Is(err, os.ErrNotExist) && !explicit {
			return nil
		}
		return apperrors.E("config.load_file", apperrors.InvalidArgument, "loading config file", err)
	}
	meta, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return apperrors.E("config.load_file", apperrors.InvalidArgument, "loading config file", err)
	}
	if meta.IsDefined("database", "backend") {
		cfg.Database.BackendConfigured = true
	}
	return nil
}

func configFilePath() (string, bool) {
	if path, ok := os.LookupEnv("EXPENSOR_CONFIG_FILE"); ok {
		return path, true
	}
	configHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "expensor", "config.toml"), false
}

func applyEnvOverrides(cfg *App) error {
	if err := setIntFromEnv("PORT", &cfg.Port); err != nil {
		return err
	}
	setStringFromEnv("BASE_URL", &cfg.BaseURL)
	setStringFromEnv("FRONTEND_URL", &cfg.FrontendURL)
	if err := setIntFromEnv("EXPENSOR_SCAN_INTERVAL", &cfg.ScanInterval); err != nil {
		return err
	}
	if err := setIntFromEnv("EXPENSOR_LOOKBACK_DAYS", &cfg.LookbackDays); err != nil {
		return err
	}
	setStringFromEnv("EXPENSOR_STATIC_DIR", &cfg.StaticDir)
	setStringFromEnv("THUNDERBIRD_DATA_DIR", &cfg.Thunderbird.DataDir)
	setDatabaseBackendFromEnv("EXPENSOR_DB_BACKEND", &cfg.Database)
	if err := setIntFromEnv("EXPENSOR_DB_BATCH_SIZE", &cfg.Database.BatchSize); err != nil {
		return err
	}
	if err := setIntFromEnv("EXPENSOR_DB_FLUSH_INTERVAL", &cfg.Database.FlushInterval); err != nil {
		return err
	}
	setStringFromEnv("EXPENSOR_SQLITE_PATH", &cfg.SQLite.Path)
	if err := setDurationFromEnv("EXPENSOR_SQLITE_BUSY_TIMEOUT", &cfg.SQLite.BusyTimeout); err != nil {
		return err
	}
	setStringFromEnv("POSTGRES_HOST", &cfg.Postgres.Host)
	if err := setIntFromEnv("POSTGRES_PORT", &cfg.Postgres.Port); err != nil {
		return err
	}
	setStringFromEnv("POSTGRES_DB", &cfg.Postgres.Database)
	setStringFromEnv("POSTGRES_USER", &cfg.Postgres.User)
	setStringFromEnv("POSTGRES_PASSWORD", &cfg.Postgres.Password)
	setStringFromEnv("POSTGRES_SSLMODE", &cfg.Postgres.SSLMode)
	if err := setInt32FromEnv("POSTGRES_MAX_POOL_SIZE", &cfg.Postgres.MaxPoolSize); err != nil {
		return err
	}
	if err := setDurationFromEnv("POSTGRES_CONNECT_TIMEOUT", &cfg.Postgres.ConnectTimeout); err != nil {
		return err
	}
	if err := setDurationFromEnv("POSTGRES_RETRY_INTERVAL", &cfg.Postgres.RetryInterval); err != nil {
		return err
	}
	setStringFromEnv("EXPENSOR_COMMUNITY_URL", &cfg.Community.URL)
	if err := setDurationFromEnv("EXPENSOR_CONTENT_SYNC_INTERVAL", &cfg.Community.SyncInterval); err != nil {
		return err
	}
	if err := setDurationFromEnv("EXPENSOR_CONTENT_SYNC_TIMEOUT", &cfg.Community.SyncTimeout); err != nil {
		return err
	}
	if err := setDurationFromEnv("EXPENSOR_APP_CONFIG_READ_TIMEOUT", &cfg.Persisted.ReadTimeout); err != nil {
		return err
	}
	setStringFromEnv("EXPENSOR_SECRET_KEY_FILE", &cfg.Security.SecretKeyFile)
	if err := setDurationFromEnv("EXPENSOR_SESSION_TTL", &cfg.Security.SessionTTL); err != nil {
		return err
	}
	if err := setDurationFromEnv("EXPENSOR_SETUP_TOKEN_TTL", &cfg.Security.SetupTokenTTL); err != nil {
		return err
	}
	if err := setLogLevelFromEnv("LOG_LEVEL", &cfg.Observability.LogLevel); err != nil {
		return err
	}
	if err := setBoolFromEnv("LOG_JSON", &cfg.Observability.LogJSON); err != nil {
		return err
	}
	if err := setBoolFromEnv("EXPENSOR_OBSERVABILITY_ENABLED", &cfg.Observability.Enabled); err != nil {
		return err
	}
	setStringFromEnv("EXPENSOR_OBSERVABILITY_EXPORTER", &cfg.Observability.Exporter)
	setStringFromEnv("EXPENSOR_OBSERVABILITY_OTLP_ENDPOINT", &cfg.Observability.OTLPEndpoint)
	return setBoolFromEnv("EXPENSOR_OBSERVABILITY_OTLP_INSECURE", &cfg.Observability.OTLPInsecure)
}

func validate(cfg *App) error {
	switch cfg.Database.Backend {
	case DatabaseBackendSQLite, DatabaseBackendPostgres:
	default:
		return apperrors.E("config.validate", apperrors.InvalidArgument, "EXPENSOR_DB_BACKEND must be one of sqlite or postgres")
	}
	if cfg.Database.BatchSize < 0 {
		return apperrors.E("config.validate", apperrors.InvalidArgument, "EXPENSOR_DB_BATCH_SIZE must be non-negative")
	}
	if cfg.Database.FlushInterval < 0 {
		return apperrors.E("config.validate", apperrors.InvalidArgument, "EXPENSOR_DB_FLUSH_INTERVAL must be non-negative")
	}
	if cfg.Postgres.MaxPoolSize < 0 {
		return apperrors.E("config.validate", apperrors.InvalidArgument, "POSTGRES_MAX_POOL_SIZE must be non-negative")
	}
	if cfg.Database.Backend == DatabaseBackendPostgres {
		if strings.TrimSpace(cfg.Postgres.Host) == "" {
			return apperrors.E("config.validate", apperrors.InvalidArgument, "POSTGRES_HOST is required when EXPENSOR_DB_BACKEND=postgres")
		}
		if strings.TrimSpace(cfg.Postgres.Database) == "" {
			return apperrors.E("config.validate", apperrors.InvalidArgument, "POSTGRES_DB is required when EXPENSOR_DB_BACKEND=postgres")
		}
		if strings.TrimSpace(cfg.Postgres.User) == "" {
			return apperrors.E("config.validate", apperrors.InvalidArgument, "POSTGRES_USER is required when EXPENSOR_DB_BACKEND=postgres")
		}
	}
	return nil
}

func setStringFromEnv(key string, target *string) {
	if value, ok := os.LookupEnv(key); ok {
		*target = value
	}
}

func setDatabaseBackendFromEnv(key string, database *Database) {
	if value, ok := os.LookupEnv(key); ok {
		database.Backend = DatabaseBackend(strings.TrimSpace(strings.ToLower(value)))
		database.BackendConfigured = true
	}
}

func setIntFromEnv(key string, target *int) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return apperrors.E("config.env", apperrors.InvalidArgument, fmt.Sprintf("parsing %s", key), err)
	}
	*target = parsed
	return nil
}

func setInt32FromEnv(key string, target *int32) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return apperrors.E("config.env", apperrors.InvalidArgument, fmt.Sprintf("parsing %s", key), err)
	}
	*target = int32(parsed)
	return nil
}

func setDurationFromEnv(key string, target *time.Duration) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return apperrors.E("config.env", apperrors.InvalidArgument, fmt.Sprintf("parsing %s", key), err)
	}
	*target = parsed
	return nil
}

func setBoolFromEnv(key string, target *bool) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return apperrors.E("config.env", apperrors.InvalidArgument, fmt.Sprintf("parsing %s", key), err)
	}
	*target = parsed
	return nil
}

func setLogLevelFromEnv(key string, target *slog.Level) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	if err := target.UnmarshalText([]byte(value)); err != nil {
		return apperrors.E("config.env", apperrors.InvalidArgument, fmt.Sprintf("parsing %s", key), err)
	}
	return nil
}

func loadSecretKey(security *Security) error {
	rawKey, hasRawKey := os.LookupEnv("EXPENSOR_SECRET_KEY")
	hasKeyFile := strings.TrimSpace(security.SecretKeyFile) != ""
	if hasRawKey && hasKeyFile {
		return apperrors.E("config.security", apperrors.InvalidArgument, "set exactly one of EXPENSOR_SECRET_KEY or EXPENSOR_SECRET_KEY_FILE")
	}
	if !hasRawKey && !hasKeyFile {
		return apperrors.E("config.security", apperrors.InvalidArgument, "set exactly one of EXPENSOR_SECRET_KEY or EXPENSOR_SECRET_KEY_FILE")
	}

	encoded := rawKey
	if hasKeyFile {
		data, err := os.ReadFile(security.SecretKeyFile)
		if err != nil {
			return apperrors.E("config.security", apperrors.InvalidArgument, "reading EXPENSOR_SECRET_KEY_FILE", err)
		}
		encoded = string(data)
	}

	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return apperrors.E("config.security", apperrors.InvalidArgument, "EXPENSOR_SECRET_KEY must be base64-encoded 32-byte key material")
	}
	if len(key) != 32 {
		return apperrors.E("config.security", apperrors.InvalidArgument, fmt.Sprintf("EXPENSOR_SECRET_KEY decoded length is %d bytes, want 32", len(key)))
	}
	security.SecretKey = key
	return nil
}
