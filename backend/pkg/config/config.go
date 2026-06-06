// Package config provides application configuration loaded from environment variables.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// Config holds the application configuration loaded from environment variables.
// Reader and writer plugin selection is driven by the web UI, not env vars.
type Config struct {
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
	OnCheckpoint  func(time.Time) `ignored:"true"`

	// Thunderbird reader configuration (profile path/mailboxes set via UI wizard).
	Thunderbird ThunderbirdConfig

	Postgres  PostgresConfig
	Community CommunityConfig
	AppConfig AppConfigConfig
}

// ThunderbirdConfig holds Thunderbird reader configuration.
type ThunderbirdConfig struct {
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
func (c *ThunderbirdConfig) GetMailboxes() []string {
	if c.Mailboxes == "" {
		return []string{}
	}
	mailboxes := strings.Split(c.Mailboxes, ",")
	for i, m := range mailboxes {
		mailboxes[i] = strings.TrimSpace(m)
	}
	return mailboxes
}

// PostgresConfig holds PostgreSQL connection configuration.
type PostgresConfig struct {
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
	MaxPoolSize int `envconfig:"POSTGRES_MAX_POOL_SIZE" default:"10"`

	// ConnectTimeout is the maximum time to wait for PostgreSQL at startup.
	ConnectTimeout time.Duration `envconfig:"POSTGRES_CONNECT_TIMEOUT" default:"30s"`

	// RetryInterval is the delay between PostgreSQL startup connection attempts.
	RetryInterval time.Duration `envconfig:"POSTGRES_RETRY_INTERVAL" default:"2s"`
}

// CommunityConfig controls community content synchronization.
type CommunityConfig struct {
	URL          string        `envconfig:"EXPENSOR_COMMUNITY_URL" default:"https://raw.githubusercontent.com/ArionMiles/expensor/main/content"`
	SyncInterval time.Duration `envconfig:"EXPENSOR_CONTENT_SYNC_INTERVAL" default:"24h"`
	SyncTimeout  time.Duration `envconfig:"EXPENSOR_CONTENT_SYNC_TIMEOUT" default:"2m"`
}

// AppConfigConfig controls reads from the persisted application configuration.
type AppConfigConfig struct {
	ReadTimeout time.Duration `envconfig:"EXPENSOR_APP_CONFIG_READ_TIMEOUT" default:"3s"`
}

// Load reads application configuration from environment variables.
func Load() (Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return Config{}, fmt.Errorf("loading environment configuration: %w", err)
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
	}
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = cfg.BaseURL
	}
	return cfg, nil
}
