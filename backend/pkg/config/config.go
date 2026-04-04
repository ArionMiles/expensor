// Package config provides application configuration loaded from environment variables.
package config

import (
	"fmt"
	"strings"
)

// ClientSecretFile is the default path to the Google OAuth credentials JSON file.
const ClientSecretFile = "data/client_secret.json"

// DefaultStateFile is the default path to the state file for tracking processed messages.
const DefaultStateFile = "data/state.json"

// Config holds the application configuration loaded from environment variables.
// Reader and writer plugin selection is driven by the web UI, not env vars.
type Config struct {
	// Port is the HTTP server port.
	// Environment variable: PORT
	// Default: 8080
	Port int `koanf:"PORT"`

	// BaseURL is the public-facing base URL of the server, used as the OAuth
	// redirect URI. Set this when hosting on a local network or remote server.
	// Environment variable: BASE_URL
	// Default: http://localhost:<PORT>
	BaseURL string `koanf:"BASE_URL"`

	// FrontendURL is the URL used for post-auth redirects (e.g. after OAuth).
	// Defaults to BaseURL — only override this for local development when the
	// frontend Vite dev server runs on a different port.
	// Environment variable: FRONTEND_URL
	// Default: same as BASE_URL
	FrontendURL string `koanf:"FRONTEND_URL"`

	// StateFile is the path to the state file for tracking processed messages.
	// Environment variable: EXPENSOR_STATE_FILE
	// Default: data/state.json
	StateFile string `koanf:"EXPENSOR_STATE_FILE"`

	// DataDir is the directory for state, token, and credential files.
	// Environment variable: EXPENSOR_DATA_DIR
	// Default: data
	DataDir string `koanf:"EXPENSOR_DATA_DIR"`

	// BaseCurrency is the primary currency used for aggregate stats.
	// Environment variable: EXPENSOR_BASE_CURRENCY
	// Default: INR
	BaseCurrency string `koanf:"EXPENSOR_BASE_CURRENCY"`

	// ScanInterval is the polling interval in seconds for all readers.
	// Environment variable: EXPENSOR_SCAN_INTERVAL
	// Default: 60
	ScanInterval int `koanf:"EXPENSOR_SCAN_INTERVAL"`

	// LookbackDays limits how far back readers search for emails on first run.
	// Environment variable: EXPENSOR_LOOKBACK_DAYS
	// Default: 180
	LookbackDays int `koanf:"EXPENSOR_LOOKBACK_DAYS"`

	// StaticDir is an optional path to serve static frontend files from disk
	// instead of the embedded assets. Useful for development.
	// Environment variable: EXPENSOR_STATIC_DIR
	// Default: "" (use embedded assets)
	StaticDir string `koanf:"EXPENSOR_STATIC_DIR"`

	// Reader-specific configurations (embedded to flatten the key namespace)
	Gmail       GmailConfig       `koanf:",squash"`
	Thunderbird ThunderbirdConfig `koanf:",squash"`

	// Writer-specific configurations (embedded to flatten the key namespace)
	Postgres PostgresConfig `koanf:",squash"`
}

// GmailConfig holds Gmail reader configuration.
type GmailConfig struct{}

// ThunderbirdConfig holds Thunderbird reader configuration.
type ThunderbirdConfig struct {
	// ProfilePath is the path to the Thunderbird profile directory.
	// Environment variable: THUNDERBIRD_PROFILE
	ProfilePath string `koanf:"THUNDERBIRD_PROFILE"`

	// Mailboxes is a comma-separated list of mailbox names to scan.
	// Environment variable: THUNDERBIRD_MAILBOXES
	// Example: "INBOX,Archives"
	Mailboxes string `koanf:"THUNDERBIRD_MAILBOXES"`
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
	Host     string `koanf:"POSTGRES_HOST"`
	Port     int    `koanf:"POSTGRES_PORT"`
	Database string `koanf:"POSTGRES_DB"`
	User     string `koanf:"POSTGRES_USER"`
	Password string `koanf:"POSTGRES_PASSWORD"`
	SSLMode  string `koanf:"POSTGRES_SSLMODE"`

	// BatchSize is the number of transactions to batch before writing.
	// Environment variable: POSTGRES_BATCH_SIZE
	// Default: 10
	BatchSize int `koanf:"POSTGRES_BATCH_SIZE"`

	// FlushInterval is the maximum time to wait before flushing a batch (seconds).
	// Environment variable: POSTGRES_FLUSH_INTERVAL
	// Default: 30
	FlushInterval int `koanf:"POSTGRES_FLUSH_INTERVAL"`

	// MaxPoolSize is the maximum number of connections in the pool.
	// Environment variable: POSTGRES_MAX_POOL_SIZE
	// Default: 10
	MaxPoolSize int `koanf:"POSTGRES_MAX_POOL_SIZE"`
}

// ValidatePostgres checks that the minimum postgres connection fields are present.
func (c *Config) ValidatePostgres() error {
	if c.Postgres.Host == "" {
		return fmt.Errorf("POSTGRES_HOST is required")
	}
	if c.Postgres.Database == "" {
		return fmt.Errorf("POSTGRES_DB is required")
	}
	if c.Postgres.User == "" {
		return fmt.Errorf("POSTGRES_USER is required")
	}
	return nil
}

// ApplyDefaults sets default values for unset configuration options.
func (c *Config) ApplyDefaults() {
	if c.Port <= 0 {
		c.Port = 8080
	}
	if c.BaseURL == "" {
		c.BaseURL = fmt.Sprintf("http://localhost:%d", c.Port)
	}
	if c.FrontendURL == "" {
		c.FrontendURL = c.BaseURL
	}

	if c.StateFile == "" {
		c.StateFile = DefaultStateFile
	}
	if c.DataDir == "" {
		c.DataDir = "data"
	}
	if c.BaseCurrency == "" {
		c.BaseCurrency = "INR"
	}

	// Scan settings defaults
	if c.ScanInterval <= 0 {
		c.ScanInterval = 60
	}
	if c.LookbackDays <= 0 {
		c.LookbackDays = 180
	}

	// Postgres defaults
	if c.Postgres.Port <= 0 {
		c.Postgres.Port = 5432
	}
	if c.Postgres.SSLMode == "" {
		c.Postgres.SSLMode = "disable"
	}
	if c.Postgres.BatchSize <= 0 {
		c.Postgres.BatchSize = 10
	}
	if c.Postgres.FlushInterval <= 0 {
		c.Postgres.FlushInterval = 30
	}
	if c.Postgres.MaxPoolSize <= 0 {
		c.Postgres.MaxPoolSize = 10
	}
}
