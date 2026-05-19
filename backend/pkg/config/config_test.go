package config_test

import (
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/config"
)

func TestConfig_ValidatePostgres(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr string
	}{
		{
			name: "valid postgres config",
			cfg: config.Config{
				Postgres: config.PostgresConfig{Host: "localhost", Database: "expensor", User: "user"},
			},
		},
		{
			name:    "missing host",
			cfg:     config.Config{Postgres: config.PostgresConfig{Database: "db", User: "u"}},
			wantErr: "POSTGRES_HOST is required",
		},
		{
			name:    "missing database",
			cfg:     config.Config{Postgres: config.PostgresConfig{Host: "h", User: "u"}},
			wantErr: "POSTGRES_DB is required",
		},
		{
			name:    "missing user",
			cfg:     config.Config{Postgres: config.PostgresConfig{Host: "h", Database: "db"}},
			wantErr: "POSTGRES_USER is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.ValidatePostgres()
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
		if cfg.ScanInterval != 60 {
			t.Errorf("ScanInterval: got %d, want 60", cfg.ScanInterval)
		}
		if cfg.LookbackDays != 180 {
			t.Errorf("LookbackDays: got %d, want 180", cfg.LookbackDays)
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
			StateFile:    "custom/state.json",
			ScanInterval: 120,
			LookbackDays: 365,
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
		if cfg.ScanInterval != 120 {
			t.Errorf("ScanInterval: got %d, want 120", cfg.ScanInterval)
		}
		if cfg.LookbackDays != 365 {
			t.Errorf("LookbackDays: got %d, want 365", cfg.LookbackDays)
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
			ScanInterval: -1,
			LookbackDays: -5,
			Postgres:     config.PostgresConfig{Port: -1, BatchSize: -1, FlushInterval: -1, MaxPoolSize: -1},
		}
		cfg.ApplyDefaults()

		if cfg.ScanInterval != 60 {
			t.Errorf("ScanInterval: got %d, want 60", cfg.ScanInterval)
		}
		if cfg.LookbackDays != 180 {
			t.Errorf("LookbackDays: got %d, want 180", cfg.LookbackDays)
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

func TestApplyDefaults_DataDirAndCurrency(t *testing.T) {
	c := config.Config{}
	c.ApplyDefaults()
	if c.DataDir != "data" {
		t.Errorf("expected DataDir=data, got %q", c.DataDir)
	}
	if c.BaseCurrency != "INR" {
		t.Errorf("expected BaseCurrency=INR, got %q", c.BaseCurrency)
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

func TestApplyDefaults_ScanSettings(t *testing.T) {
	c := config.Config{}
	c.ApplyDefaults()
	if c.ScanInterval != 60 {
		t.Errorf("expected ScanInterval=60, got %d", c.ScanInterval)
	}
	if c.LookbackDays != 180 {
		t.Errorf("expected LookbackDays=180, got %d", c.LookbackDays)
	}
}
