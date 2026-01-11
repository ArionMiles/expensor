package config

import (
	"encoding/json"
)

// ClientSecretFile is the default path to the Google OAuth credentials JSON file.
const ClientSecretFile = "data/client_secret.json"

// Config holds the application configuration loaded from environment variables.
type Config struct {
	// ReaderPlugin is the name of the reader plugin to use.
	// Environment variable: EXPENSOR_READER
	ReaderPlugin string `koanf:"EXPENSOR_READER"`

	// WriterPlugin is the name of the writer plugin to use.
	// Environment variable: EXPENSOR_WRITER
	WriterPlugin string `koanf:"EXPENSOR_WRITER"`

	// ReaderConfig is the JSON configuration for the reader plugin.
	// Environment variable: EXPENSOR_READER_CONFIG
	ReaderConfig json.RawMessage `koanf:"EXPENSOR_READER_CONFIG"`

	// WriterConfig is the JSON configuration for the writer plugin.
	// Environment variable: EXPENSOR_WRITER_CONFIG
	WriterConfig json.RawMessage `koanf:"EXPENSOR_WRITER_CONFIG"`

	// PostgreSQL configuration (can be used by postgres writer plugin)
	Postgres PostgresConfig
}

// PostgresConfig holds PostgreSQL connection configuration.
type PostgresConfig struct {
	Host     string `koanf:"POSTGRES_HOST"`
	Port     int    `koanf:"POSTGRES_PORT"`
	Database string `koanf:"POSTGRES_DB"`
	User     string `koanf:"POSTGRES_USER"`
	Password string `koanf:"POSTGRES_PASSWORD"`
	SSLMode  string `koanf:"POSTGRES_SSLMODE"`
}
