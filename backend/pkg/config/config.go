// Package config provides application configuration loaded from environment variables.
package config

import (
	"fmt"
	"strings"
	"time"
)

// ClientSecretFile is the default path to the Google OAuth credentials JSON file.
const ClientSecretFile = "data/client_secret.json"

// DefaultStateFile is the default path to the state file for tracking processed messages.
const DefaultStateFile = "data/state.json"

// Config holds the application configuration loaded from environment variables.
// Reader and writer plugin selection is driven by the web UI, not env vars.
type Config struct {
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

	// Reader-specific configurations (embedded to flatten the key namespace)
	Gmail       GmailConfig       `koanf:",squash"`
	Thunderbird ThunderbirdConfig `koanf:",squash"`

	// Writer-specific configurations (embedded to flatten the key namespace)
	Postgres PostgresConfig `koanf:",squash"`
}

// GmailConfig holds Gmail reader configuration.
type GmailConfig struct {
	// Interval is the polling interval in seconds.
	// Environment variable: GMAIL_INTERVAL
	// Default: 60
	Interval int `koanf:"GMAIL_INTERVAL"`

	// LookbackDays is how far back in time (in days) to search for emails.
	// Environment variable: GMAIL_LOOKBACK_DAYS
	// Default: 180 (6 months)
	LookbackDays int `koanf:"GMAIL_LOOKBACK_DAYS"`
}

// ThunderbirdConfig holds Thunderbird reader configuration.
type ThunderbirdConfig struct {
	// ProfilePath is the path to the Thunderbird profile directory.
	// Environment variable: THUNDERBIRD_PROFILE
	ProfilePath string `koanf:"THUNDERBIRD_PROFILE"`

	// Mailboxes is a comma-separated list of mailbox names to scan.
	// Environment variable: THUNDERBIRD_MAILBOXES
	// Example: "INBOX,Archives"
	Mailboxes string `koanf:"THUNDERBIRD_MAILBOXES"`

	// Interval is the polling interval in seconds.
	// Environment variable: THUNDERBIRD_INTERVAL
	// Default: 60
	Interval int `koanf:"THUNDERBIRD_INTERVAL"`
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

// GetInterval returns the interval as a time.Duration.
func (c *ThunderbirdConfig) GetInterval() time.Duration {
	if c.Interval <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.Interval) * time.Second
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
	if c.StateFile == "" {
		c.StateFile = DefaultStateFile
	}
	if c.DataDir == "" {
		c.DataDir = "data"
	}
	if c.BaseCurrency == "" {
		c.BaseCurrency = "INR"
	}

	// Gmail defaults
	if c.Gmail.Interval <= 0 {
		c.Gmail.Interval = 60
	}
	if c.Gmail.LookbackDays <= 0 {
		c.Gmail.LookbackDays = 180
	}

	// Thunderbird defaults
	if c.Thunderbird.Interval <= 0 {
		c.Thunderbird.Interval = 60
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
