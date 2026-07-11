// Package config provides application configuration loaded from a TOML file and environment variables.
package config

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const ServiceName = "expensor"

// Version is set at build time via -ldflags.
var Version = "dev"

// App holds the application configuration loaded from a TOML file and environment variables.
// Reader plugin selection is driven by the web UI, not env vars.
type App struct {
	// Port is the HTTP server port.
	// Environment variable: PORT
	// Default: 8080
	Port int `toml:"port" env:"PORT" default:"8080" validate:"gte=1,lte=65535"`

	// BaseURL is the public-facing base URL of the server, used as the OAuth
	// redirect URI. Set this when hosting on a local network or remote server.
	// Environment variable: BASE_URL
	// Default: http://localhost:<PORT>
	BaseURL string `toml:"base_url" env:"BASE_URL"`

	// FrontendURL is the URL used for post-auth redirects (e.g. after OAuth).
	// Defaults to BaseURL — only override this for local development when the
	// frontend Vite dev server runs on a different port.
	// Environment variable: FRONTEND_URL
	// Default: same as BASE_URL
	FrontendURL string `toml:"frontend_url" env:"FRONTEND_URL"`

	// ScanInterval is the polling interval in seconds for all readers.
	// Environment variable: EXPENSOR_SCAN_INTERVAL
	// Default: 60
	ScanInterval int `toml:"scan_interval" env:"EXPENSOR_SCAN_INTERVAL" default:"60" validate:"gte=0"`

	// LookbackDays limits how far back readers search for emails on first run.
	// Environment variable: EXPENSOR_LOOKBACK_DAYS
	// Default: 180
	LookbackDays int `toml:"lookback_days" env:"EXPENSOR_LOOKBACK_DAYS" default:"180" validate:"gte=0"`

	// StaticDir is an optional path to serve static frontend files from disk
	// instead of the embedded assets. Useful for development.
	// Environment variable: EXPENSOR_STATIC_DIR
	// Default: "" (use embedded assets)
	StaticDir string `toml:"static_dir" env:"EXPENSOR_STATIC_DIR"`

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
	DataDir string `toml:"data_dir" env:"THUNDERBIRD_DATA_DIR"`
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
	Backend       DatabaseBackend `toml:"backend" env:"EXPENSOR_DB_BACKEND" validate:"omitempty,oneof=sqlite postgres"`
	BatchSize     int             `toml:"batch_size" env:"EXPENSOR_DB_BATCH_SIZE" default:"10" validate:"gte=0"`
	FlushInterval int             `toml:"flush_interval" env:"EXPENSOR_DB_FLUSH_INTERVAL" default:"30" validate:"gte=0"`
	SQLite        SQLite          `toml:"sqlite"`
	Postgres      Postgres        `toml:"postgres"`
}

type DatabaseBackend string

const (
	DatabaseBackendSQLite   DatabaseBackend = "sqlite"
	DatabaseBackendPostgres DatabaseBackend = "postgres"
)

type SQLite struct {
	Path        string        `toml:"path" env:"EXPENSOR_SQLITE_PATH"`
	BusyTimeout time.Duration `toml:"busy_timeout" env:"EXPENSOR_SQLITE_BUSY_TIMEOUT" default:"5s" validate:"gt=0"`
}

// Postgres holds PostgreSQL connection configuration.
type Postgres struct {
	Host     string `toml:"host" env:"POSTGRES_HOST"`
	Port     int    `toml:"port" env:"POSTGRES_PORT" default:"5432" validate:"gte=1,lte=65535"`
	Database string `toml:"database" env:"POSTGRES_DB"`
	User     string `toml:"user" env:"POSTGRES_USER"`
	Password string `toml:"password" env:"POSTGRES_PASSWORD"`
	SSLMode  string `toml:"sslmode" env:"POSTGRES_SSLMODE" default:"disable"`

	// MaxPoolSize is the maximum number of connections in the pool.
	// Environment variable: POSTGRES_MAX_POOL_SIZE
	// Default: 10
	MaxPoolSize int32 `toml:"max_pool_size" env:"POSTGRES_MAX_POOL_SIZE" default:"10" validate:"gte=0"`
}

// Community controls community content synchronization.
type Community struct {
	//nolint:lll // default content URL is intentionally explicit in the config tag.
	URL          string        `toml:"url" env:"EXPENSOR_COMMUNITY_URL" default:"https://raw.githubusercontent.com/ArionMiles/expensor/main/backend/internal/catalog/content"`
	SyncInterval time.Duration `toml:"sync_interval" env:"EXPENSOR_CONTENT_SYNC_INTERVAL" default:"24h" validate:"gt=0"`
	SyncTimeout  time.Duration `toml:"sync_timeout" env:"EXPENSOR_CONTENT_SYNC_TIMEOUT" default:"2m" validate:"gt=0"`
}

// Persisted controls reads from the persisted application configuration.
type Persisted struct {
	ReadTimeout time.Duration `toml:"read_timeout" env:"EXPENSOR_APP_CONFIG_READ_TIMEOUT" default:"3s" validate:"gt=0"`
}

// Security controls authentication and credential encryption settings.
type Security struct {
	SecretKey     []byte        `toml:"-"`
	SecretKeyFile string        `toml:"secret_key_file" env:"EXPENSOR_SECRET_KEY_FILE"`
	SessionTTL    time.Duration `toml:"session_ttl" env:"EXPENSOR_SESSION_TTL" default:"168h" validate:"gt=0"`
	SetupTokenTTL time.Duration `toml:"setup_token_ttl" env:"EXPENSOR_SETUP_TOKEN_TTL" default:"24h" validate:"gt=0"`
}

// Observability controls application logging and telemetry export.
type Observability struct {
	LogLevel     slog.Level `toml:"log_level" env:"LOG_LEVEL" default:"INFO"`
	LogJSON      bool       `toml:"log_json" env:"LOG_JSON" default:"false"`
	Enabled      bool       `toml:"enabled" env:"EXPENSOR_OBSERVABILITY_ENABLED"`
	Exporter     string     `toml:"exporter" env:"EXPENSOR_OBSERVABILITY_EXPORTER" default:"none" validate:"oneof=none otlp"`
	OTLPEndpoint string     `toml:"otlp_endpoint" env:"EXPENSOR_OBSERVABILITY_OTLP_ENDPOINT"`
	OTLPInsecure bool       `toml:"otlp_insecure" env:"EXPENSOR_OBSERVABILITY_OTLP_INSECURE"`
	Output       io.Writer  `toml:"-"`
}

// Load reads application configuration from a TOML file and environment variables.
func Load() (App, error) {
	var cfg App
	if err := applyDefaults(&cfg); err != nil {
		return App{}, err
	}
	if err := loadConfigFile(&cfg); err != nil {
		return App{}, err
	}
	if err := applyEnvOverrides(&cfg); err != nil {
		return App{}, err
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

func loadConfigFile(cfg *App) error {
	path, explicit := configFilePath()
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) && !explicit {
			return nil
		}
		return errors.E("config.load_file", errors.InvalidArgument, "loading config file", err)
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return errors.E("config.load_file", errors.InvalidArgument, "loading config file", err)
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
	return walkConfigFields(reflect.ValueOf(cfg).Elem(), func(field reflect.Value, structField reflect.StructField) error {
		key := structField.Tag.Get("env")
		if key == "" {
			return nil
		}
		value, ok := os.LookupEnv(key)
		if !ok {
			return nil
		}
		if err := setConfigField(field, value); err != nil {
			return errors.E("config.env", errors.InvalidArgument, fmt.Sprintf("parsing %s", key), err)
		}
		return nil
	})
}

func applyDefaults(cfg *App) error {
	return walkConfigFields(reflect.ValueOf(cfg).Elem(), func(field reflect.Value, structField reflect.StructField) error {
		value := structField.Tag.Get("default")
		if value == "" {
			return nil
		}
		if err := setConfigField(field, value); err != nil {
			return errors.E("config.defaults", errors.InvalidArgument, fmt.Sprintf("applying default for %s", configFieldName(structField)), err)
		}
		return nil
	})
}

func walkConfigFields(value reflect.Value, visit func(reflect.Value, reflect.StructField) error) error {
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		structField := value.Type().Field(i)
		if !field.CanSet() {
			continue
		}
		if err := visit(field, structField); err != nil {
			return err
		}
		if shouldWalkConfigField(field, structField) {
			if err := walkConfigFields(field, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

func shouldWalkConfigField(field reflect.Value, structField reflect.StructField) bool {
	if structField.Tag.Get("toml") == "-" {
		return false
	}
	if structField.Tag.Get("env") != "" || structField.Tag.Get("default") != "" {
		return false
	}
	return field.Kind() == reflect.Struct && field.Type() != reflect.TypeOf(time.Time{})
}

func setConfigField(field reflect.Value, raw string) error {
	raw = strings.TrimSpace(raw)
	switch field.Type() {
	case reflect.TypeOf(time.Duration(0)):
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return err
		}
		field.SetInt(int64(parsed))
		return nil
	case reflect.TypeOf(slog.Level(0)):
		var level slog.Level
		if err := level.UnmarshalText([]byte(raw)); err != nil {
			return err
		}
		field.SetInt(int64(level))
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		if field.Type() == reflect.TypeOf(DatabaseBackend("")) {
			raw = strings.ToLower(raw)
		}
		field.SetString(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(parsed)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		field.SetBool(parsed)
	default:
		return errors.E("config.set_field", errors.InvalidArgument, fmt.Sprintf("unsupported config field type %s", field.Type()))
	}
	return nil
}

func validate(cfg *App) error {
	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		return errors.E("config.validate", errors.InvalidArgument, validationMessage(cfg, err), err)
	}
	if cfg.Database.Backend == DatabaseBackendPostgres {
		if strings.TrimSpace(cfg.Database.Postgres.Host) == "" {
			return errors.E("config.validate", errors.InvalidArgument, "POSTGRES_HOST is required when EXPENSOR_DB_BACKEND=postgres")
		}
		if strings.TrimSpace(cfg.Database.Postgres.Database) == "" {
			return errors.E("config.validate", errors.InvalidArgument, "POSTGRES_DB is required when EXPENSOR_DB_BACKEND=postgres")
		}
		if strings.TrimSpace(cfg.Database.Postgres.User) == "" {
			return errors.E("config.validate", errors.InvalidArgument, "POSTGRES_USER is required when EXPENSOR_DB_BACKEND=postgres")
		}
	}
	return nil
}

func validationMessage(cfg *App, err error) string {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) || len(validationErrors) == 0 {
		return "invalid configuration"
	}
	fieldName := validationFieldName(reflect.TypeOf(*cfg), validationErrors[0].StructNamespace())
	return fmt.Sprintf("%s failed validation %q", fieldName, validationErrors[0].Tag())
}

func validationFieldName(root reflect.Type, namespace string) string {
	parts := strings.Split(namespace, ".")
	if len(parts) > 0 && parts[0] == root.Name() {
		parts = parts[1:]
	}
	current := root
	fieldName := namespace
	for _, part := range parts {
		field, ok := current.FieldByName(part)
		if !ok {
			return part
		}
		fieldName = configFieldName(field)
		current = field.Type
	}
	return fieldName
}

func configFieldName(field reflect.StructField) string {
	if envName := field.Tag.Get("env"); envName != "" {
		return envName
	}
	if tomlName := strings.TrimSuffix(field.Tag.Get("toml"), ",omitempty"); tomlName != "" && tomlName != "-" {
		return tomlName
	}
	return field.Name
}

func loadSecretKey(security *Security) error {
	rawKey, hasRawKey := os.LookupEnv("EXPENSOR_SECRET_KEY")
	hasKeyFile := strings.TrimSpace(security.SecretKeyFile) != ""
	if hasRawKey && hasKeyFile {
		return errors.E("config.security", errors.InvalidArgument, "set exactly one of EXPENSOR_SECRET_KEY or EXPENSOR_SECRET_KEY_FILE")
	}
	if !hasRawKey && !hasKeyFile {
		return errors.E("config.security", errors.InvalidArgument, "set exactly one of EXPENSOR_SECRET_KEY or EXPENSOR_SECRET_KEY_FILE")
	}

	encoded := rawKey
	if hasKeyFile {
		data, err := os.ReadFile(security.SecretKeyFile)
		if err != nil {
			return errors.E("config.security", errors.InvalidArgument, "reading EXPENSOR_SECRET_KEY_FILE", err)
		}
		encoded = string(data)
	}

	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return errors.E("config.security", errors.InvalidArgument, "EXPENSOR_SECRET_KEY must be base64-encoded 32-byte key material")
	}
	if len(key) != 32 {
		return errors.E("config.security", errors.InvalidArgument, fmt.Sprintf("EXPENSOR_SECRET_KEY decoded length is %d bytes, want 32", len(key)))
	}
	security.SecretKey = key
	return nil
}
