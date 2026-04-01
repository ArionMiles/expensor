package config_test

import (
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/config"
)

func TestConfig_Validate(t *testing.T) {
	validPostgres := config.PostgresConfig{
		Host:     "localhost",
		Database: "expensor",
		User:     "user",
	}

	tests := []struct {
		name    string
		cfg     config.Config
		wantErr string
	}{
		{
			name: "valid gmail+postgres",
			cfg: config.Config{
				ReaderPlugin: "gmail",
				WriterPlugin: "postgres",
				Postgres:     validPostgres,
			},
		},
		{
			name: "valid thunderbird+postgres",
			cfg: config.Config{
				ReaderPlugin: "thunderbird",
				WriterPlugin: "postgres",
				Postgres:     validPostgres,
				Thunderbird: config.ThunderbirdConfig{
					ProfilePath: "/home/user/.thunderbird/abc.default",
					Mailboxes:   "INBOX",
				},
			},
		},
		{
			name:    "missing reader plugin",
			cfg:     config.Config{WriterPlugin: "postgres", Postgres: validPostgres},
			wantErr: "EXPENSOR_READER is required",
		},
		{
			name:    "unknown reader plugin",
			cfg:     config.Config{ReaderPlugin: "foobar", WriterPlugin: "postgres", Postgres: validPostgres},
			wantErr: "unknown reader plugin: foobar",
		},
		{
			name: "thunderbird missing profile path",
			cfg: config.Config{
				ReaderPlugin: "thunderbird",
				WriterPlugin: "postgres",
				Postgres:     validPostgres,
				Thunderbird:  config.ThunderbirdConfig{Mailboxes: "INBOX"},
			},
			wantErr: "THUNDERBIRD_PROFILE is required when using thunderbird reader",
		},
		{
			name: "thunderbird missing mailboxes",
			cfg: config.Config{
				ReaderPlugin: "thunderbird",
				WriterPlugin: "postgres",
				Postgres:     validPostgres,
				Thunderbird:  config.ThunderbirdConfig{ProfilePath: "/some/path"},
			},
			wantErr: "THUNDERBIRD_MAILBOXES is required when using thunderbird reader",
		},
		{
			name:    "missing writer plugin",
			cfg:     config.Config{ReaderPlugin: "gmail"},
			wantErr: "EXPENSOR_WRITER is required",
		},
		{
			name:    "unknown writer plugin",
			cfg:     config.Config{ReaderPlugin: "gmail", WriterPlugin: "sheets"},
			wantErr: "unknown writer plugin: sheets",
		},
		{
			name:    "postgres missing host",
			cfg:     config.Config{ReaderPlugin: "gmail", WriterPlugin: "postgres", Postgres: config.PostgresConfig{Database: "db", User: "u"}},
			wantErr: "POSTGRES_HOST is required when using postgres writer",
		},
		{
			name:    "postgres missing database",
			cfg:     config.Config{ReaderPlugin: "gmail", WriterPlugin: "postgres", Postgres: config.PostgresConfig{Host: "h", User: "u"}},
			wantErr: "POSTGRES_DB is required when using postgres writer",
		},
		{
			name:    "postgres missing user",
			cfg:     config.Config{ReaderPlugin: "gmail", WriterPlugin: "postgres", Postgres: config.PostgresConfig{Host: "h", Database: "db"}},
			wantErr: "POSTGRES_USER is required when using postgres writer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error %q, got nil", tc.wantErr)
				return
			}
			if err.Error() != tc.wantErr {
				t.Errorf("error: got %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestConfig_ApplyDefaults(t *testing.T) {
	t.Run("zero value gets all defaults", func(t *testing.T) {
		cfg := config.Config{}
		cfg.ApplyDefaults()

		if cfg.StateFile != config.DefaultStateFile {
			t.Errorf("StateFile: got %q, want %q", cfg.StateFile, config.DefaultStateFile)
		}
		if cfg.Gmail.Interval != 60 {
			t.Errorf("Gmail.Interval: got %d, want 60", cfg.Gmail.Interval)
		}
		if cfg.Thunderbird.Interval != 60 {
			t.Errorf("Thunderbird.Interval: got %d, want 60", cfg.Thunderbird.Interval)
		}
		if cfg.Postgres.Port != 5432 {
			t.Errorf("Postgres.Port: got %d, want 5432", cfg.Postgres.Port)
		}
		if cfg.Postgres.SSLMode != "disable" {
			t.Errorf("Postgres.SSLMode: got %q, want \"disable\"", cfg.Postgres.SSLMode)
		}
		if cfg.Postgres.BatchSize != 10 {
			t.Errorf("Postgres.BatchSize: got %d, want 10", cfg.Postgres.BatchSize)
		}
		if cfg.Postgres.FlushInterval != 30 {
			t.Errorf("Postgres.FlushInterval: got %d, want 30", cfg.Postgres.FlushInterval)
		}
		if cfg.Postgres.MaxPoolSize != 10 {
			t.Errorf("Postgres.MaxPoolSize: got %d, want 10", cfg.Postgres.MaxPoolSize)
		}
	})

	t.Run("pre-set values are not overwritten", func(t *testing.T) {
		cfg := config.Config{
			StateFile: "custom/state.json",
			Gmail:     config.GmailConfig{Interval: 120},
			Thunderbird: config.ThunderbirdConfig{
				Interval: 300,
			},
			Postgres: config.PostgresConfig{
				Port:          5433,
				SSLMode:       "require",
				BatchSize:     50,
				FlushInterval: 60,
				MaxPoolSize:   20,
			},
		}
		cfg.ApplyDefaults()

		if cfg.StateFile != "custom/state.json" {
			t.Errorf("StateFile: got %q, want \"custom/state.json\"", cfg.StateFile)
		}
		if cfg.Gmail.Interval != 120 {
			t.Errorf("Gmail.Interval: got %d, want 120", cfg.Gmail.Interval)
		}
		if cfg.Thunderbird.Interval != 300 {
			t.Errorf("Thunderbird.Interval: got %d, want 300", cfg.Thunderbird.Interval)
		}
		if cfg.Postgres.Port != 5433 {
			t.Errorf("Postgres.Port: got %d, want 5433", cfg.Postgres.Port)
		}
		if cfg.Postgres.SSLMode != "require" {
			t.Errorf("Postgres.SSLMode: got %q, want \"require\"", cfg.Postgres.SSLMode)
		}
		if cfg.Postgres.BatchSize != 50 {
			t.Errorf("Postgres.BatchSize: got %d, want 50", cfg.Postgres.BatchSize)
		}
		if cfg.Postgres.FlushInterval != 60 {
			t.Errorf("Postgres.FlushInterval: got %d, want 60", cfg.Postgres.FlushInterval)
		}
		if cfg.Postgres.MaxPoolSize != 20 {
			t.Errorf("Postgres.MaxPoolSize: got %d, want 20", cfg.Postgres.MaxPoolSize)
		}
	})

	t.Run("negative intervals get defaults", func(t *testing.T) {
		cfg := config.Config{
			Gmail:       config.GmailConfig{Interval: -1},
			Thunderbird: config.ThunderbirdConfig{Interval: -5},
			Postgres:    config.PostgresConfig{Port: -1, BatchSize: -1, FlushInterval: -1, MaxPoolSize: -1},
		}
		cfg.ApplyDefaults()

		if cfg.Gmail.Interval != 60 {
			t.Errorf("Gmail.Interval: got %d, want 60", cfg.Gmail.Interval)
		}
		if cfg.Thunderbird.Interval != 60 {
			t.Errorf("Thunderbird.Interval: got %d, want 60", cfg.Thunderbird.Interval)
		}
		if cfg.Postgres.Port != 5432 {
			t.Errorf("Postgres.Port: got %d, want 5432", cfg.Postgres.Port)
		}
		if cfg.Postgres.BatchSize != 10 {
			t.Errorf("Postgres.BatchSize: got %d, want 10", cfg.Postgres.BatchSize)
		}
		if cfg.Postgres.FlushInterval != 30 {
			t.Errorf("Postgres.FlushInterval: got %d, want 30", cfg.Postgres.FlushInterval)
		}
		if cfg.Postgres.MaxPoolSize != 10 {
			t.Errorf("Postgres.MaxPoolSize: got %d, want 10", cfg.Postgres.MaxPoolSize)
		}
	})
}

func TestThunderbirdConfig_GetMailboxes(t *testing.T) {
	tests := []struct {
		name      string
		mailboxes string
		want      []string
	}{
		{
			name:      "empty string returns empty slice",
			mailboxes: "",
			want:      []string{},
		},
		{
			name:      "single mailbox",
			mailboxes: "INBOX",
			want:      []string{"INBOX"},
		},
		{
			name:      "multiple mailboxes",
			mailboxes: "INBOX,Archives,Sent",
			want:      []string{"INBOX", "Archives", "Sent"},
		},
		{
			name:      "spaces around commas are trimmed",
			mailboxes: "INBOX , Archives , Sent",
			want:      []string{"INBOX", "Archives", "Sent"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.ThunderbirdConfig{Mailboxes: tc.mailboxes}
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

func TestThunderbirdConfig_GetInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval int
		want     time.Duration
	}{
		{name: "positive interval", interval: 120, want: 120 * time.Second},
		{name: "one second", interval: 1, want: time.Second},
		{name: "zero returns default 60s", interval: 0, want: 60 * time.Second},
		{name: "negative returns default 60s", interval: -10, want: 60 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.ThunderbirdConfig{Interval: tc.interval}
			got := cfg.GetInterval()
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
