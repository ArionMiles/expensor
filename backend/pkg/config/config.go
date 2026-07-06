// Package config provides application configuration loaded from environment variables.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

const ServiceName = "expensor"

// Version is set at build time via -ldflags.
var Version = "dev"

// App holds the application configuration loaded from environment variables.
// Reader plugin selection is driven by the web UI, not env vars.
type App struct {
	// Port is the HTTP server port.
	// Environment variable: PORT
	// Default: 8080
	Port int `envconfig:"PORT" default:"8080"`

	// BaseURL is the public-facing base URL of the server, used as the OAuth
	// redirect URI. Set this when hosting on a local network or remote server.
	// Environment variable: BASE_URL
	// Default: http://localhost:<PORT>
	BaseURL string `envconfig:"BASE_URL"`

	// FrontendURL is the URL used for post-auth redirects (e.g. after OAuth).
	// Defaults to BaseURL — only override this for local development when the
	// frontend Vite dev server runs on a different port.
	// Environment variable: FRONTEND_URL
	// Default: same as BASE_URL
	FrontendURL string `envconfig:"FRONTEND_URL"`

	// ScanInterval is the polling interval in seconds for all readers.
	// Environment variable: EXPENSOR_SCAN_INTERVAL
	// Default: 60
	ScanInterval int `envconfig:"EXPENSOR_SCAN_INTERVAL" default:"60"`

	// LookbackDays limits how far back readers search for emails on first run.
	// Environment variable: EXPENSOR_LOOKBACK_DAYS
	// Default: 180
	LookbackDays int `envconfig:"EXPENSOR_LOOKBACK_DAYS" default:"180"`

	// StaticDir is an optional path to serve static frontend files from disk
	// instead of the embedded assets. Useful for development.
	// Environment variable: EXPENSOR_STATIC_DIR
	// Default: "" (use embedded assets)
	StaticDir string `envconfig:"EXPENSOR_STATIC_DIR"`

	// Runtime-only scanning checkpoint fields.
	// These are set programmatically by the daemon coordinator and are NOT
	// loaded from environment variables.
	LastScanAt    *time.Time      `ignored:"true"`
	ForceFullScan bool            `ignored:"true"`
	RunOnce       bool            `ignored:"true"`
	OnCheckpoint  func(time.Time) `ignored:"true"`

	// Thunderbird reader configuration (profile path/mailboxes set via UI wizard).
	Thunderbird Thunderbird

	Postgres      Postgres
	Community     Community
	Persisted     Persisted
	Security      Security
	Observability Observability
}

// Thunderbird holds Thunderbird reader configuration.
type Thunderbird struct {
	// ProfilePath is the path to the Thunderbird profile directory.
	// Set via the web UI onboarding wizard; not loaded from env vars.
	ProfilePath string

	// Mailboxes is a comma-separated list of mailbox names to scan.
	// Set via the web UI onboarding wizard; not loaded from env vars.
	Mailboxes string

	// DataDir is an optional extra path hinting where Thunderbird profile
	// directories can be found (used by the profile discovery endpoint).
	// Useful in Docker when a custom profile mount doesn't match platform defaults.
	// Environment variable: THUNDERBIRD_DATA_DIR
	DataDir string `envconfig:"THUNDERBIRD_DATA_DIR"`
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

// Postgres holds PostgreSQL connection configuration.
type Postgres struct {
	Host     string `envconfig:"POSTGRES_HOST" required:"true"`
	Port     int    `envconfig:"POSTGRES_PORT" default:"5432"`
	Database string `envconfig:"POSTGRES_DB" required:"true"`
	User     string `envconfig:"POSTGRES_USER" required:"true"`
	Password string `envconfig:"POSTGRES_PASSWORD"`
	SSLMode  string `envconfig:"POSTGRES_SSLMODE" default:"disable"`

	// BatchSize is the number of transactions to batch before writing.
	// Environment variable: POSTGRES_BATCH_SIZE
	// Default: 10
	BatchSize int `envconfig:"POSTGRES_BATCH_SIZE" default:"10"`

	// FlushInterval is the maximum time to wait before flushing a batch (seconds).
	// Environment variable: POSTGRES_FLUSH_INTERVAL
	// Default: 30
	FlushInterval int `envconfig:"POSTGRES_FLUSH_INTERVAL" default:"30"`

	// MaxPoolSize is the maximum number of connections in the pool.
	// Environment variable: POSTGRES_MAX_POOL_SIZE
	// Default: 10
	MaxPoolSize int32 `envconfig:"POSTGRES_MAX_POOL_SIZE" default:"10"`

	// ConnectTimeout is the maximum time to wait for PostgreSQL at startup.
	ConnectTimeout time.Duration `envconfig:"POSTGRES_CONNECT_TIMEOUT" default:"30s"`

	// RetryInterval is the delay between PostgreSQL startup connection attempts.
	RetryInterval time.Duration `envconfig:"POSTGRES_RETRY_INTERVAL" default:"2s"`
}

// Community controls community content synchronization.
type Community struct {
	URL          string        `envconfig:"EXPENSOR_COMMUNITY_URL" default:"https://raw.githubusercontent.com/ArionMiles/expensor/main/backend/cmd/server/content"`
	SyncInterval time.Duration `envconfig:"EXPENSOR_CONTENT_SYNC_INTERVAL" default:"24h"`
	SyncTimeout  time.Duration `envconfig:"EXPENSOR_CONTENT_SYNC_TIMEOUT" default:"2m"`
}

// Persisted controls reads from the persisted application configuration.
type Persisted struct {
	ReadTimeout time.Duration `envconfig:"EXPENSOR_APP_CONFIG_READ_TIMEOUT" default:"3s"`
}

// Security controls authentication and credential encryption settings.
type Security struct {
	SecretKey     []byte        `ignored:"true"`
	SecretKeyFile string        `envconfig:"EXPENSOR_SECRET_KEY_FILE"`
	SessionTTL    time.Duration `envconfig:"EXPENSOR_SESSION_TTL" default:"168h"`
	SetupTokenTTL time.Duration `envconfig:"EXPENSOR_SETUP_TOKEN_TTL" default:"24h"`
}

// Observability controls application logging and telemetry export.
type Observability struct {
	LogLevel     slog.Level `envconfig:"LOG_LEVEL" default:"INFO"`
	LogJSON      bool       `envconfig:"LOG_JSON" default:"false"`
	Enabled      bool       `envconfig:"EXPENSOR_OBSERVABILITY_ENABLED" default:"false"`
	Exporter     string     `envconfig:"EXPENSOR_OBSERVABILITY_EXPORTER" default:"none"`
	OTLPEndpoint string     `envconfig:"EXPENSOR_OBSERVABILITY_OTLP_ENDPOINT"`
	OTLPInsecure bool       `envconfig:"EXPENSOR_OBSERVABILITY_OTLP_INSECURE" default:"false"`
	Output       io.Writer  `ignored:"true"`
}

// Load reads application configuration from environment variables.
func Load() (App, error) {
	var cfg App
	if err := envconfig.Process("", &cfg); err != nil {
		return App{}, fmt.Errorf("loading environment configuration: %w", err)
	}
	if cfg.Postgres.MaxPoolSize < 0 {
		return App{}, errors.New("POSTGRES_MAX_POOL_SIZE must be non-negative")
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

func loadSecretKey(security *Security) error {
	rawKey, hasRawKey := os.LookupEnv("EXPENSOR_SECRET_KEY")
	hasKeyFile := strings.TrimSpace(security.SecretKeyFile) != ""
	if hasRawKey && hasKeyFile {
		return errors.New("set exactly one of EXPENSOR_SECRET_KEY or EXPENSOR_SECRET_KEY_FILE")
	}
	if !hasRawKey && !hasKeyFile {
		return errors.New("set exactly one of EXPENSOR_SECRET_KEY or EXPENSOR_SECRET_KEY_FILE")
	}

	encoded := rawKey
	if hasKeyFile {
		data, err := os.ReadFile(security.SecretKeyFile)
		if err != nil {
			return fmt.Errorf("reading EXPENSOR_SECRET_KEY_FILE: %w", err)
		}
		encoded = string(data)
	}

	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return errors.New("EXPENSOR_SECRET_KEY must be base64-encoded 32-byte key material")
	}
	if len(key) != 32 {
		return fmt.Errorf("EXPENSOR_SECRET_KEY decoded length is %d bytes, want 32", len(key))
	}
	security.SecretKey = key
	return nil
}
