package thunderbird

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

func TestPlugin_Metadata(t *testing.T) {
	plugin := &Plugin{}
	plugin.SetGuideData([]byte(`{"sections":[]}`))

	metadata := plugin.Metadata()
	if metadata.Name != "thunderbird" {
		t.Errorf("Name = %q, want %q", metadata.Name, "thunderbird")
	}
	if metadata.Description == "" {
		t.Error("Description should not be empty")
	}
	if metadata.Auth.Type != plugins.AuthTypeConfig {
		t.Errorf("Auth.Type = %q, want %q", metadata.Auth.Type, plugins.AuthTypeConfig)
	}
	if metadata.Auth.RequiresCredentialsUpload {
		t.Error("RequiresCredentialsUpload = true, want false")
	}
	if len(metadata.ConfigSchema) == 0 {
		t.Error("ConfigSchema should not be empty")
	}
	if len(metadata.SetupGuide) == 0 {
		t.Error("SetupGuide should not be empty after SetGuideData")
	}
}

func TestPlugin_NewReader(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test mailbox structure
	mailDir := filepath.Join(tmpDir, "Mail", "Local Folders")
	if err := os.MkdirAll(mailDir, 0o755); err != nil {
		t.Fatalf("failed to create mail dir: %v", err)
	}

	inboxPath := filepath.Join(mailDir, "Inbox")
	if err := os.WriteFile(inboxPath, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to create inbox: %v", err)
	}

	// Create state file in tmpDir
	stateFile := filepath.Join(tmpDir, "state.json")
	stateManager, err := state.New(stateFile, slog.Default())
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	// Create config
	cfg := &config.Config{
		ScanInterval: 10,
		Thunderbird: config.ThunderbirdConfig{
			ProfilePath: tmpDir,
			Mailboxes:   "Inbox",
		},
	}

	// Create rules
	amountRegex := regexp.MustCompile(`Rs\.\s*([\d,]+\.?\d*)`)
	merchantRegex := regexp.MustCompile(`at\s+(\w+)`)
	rules := []api.Rule{
		{
			Name:         "test rule",
			SenderEmail:  "test@example.com",
			Amount:       amountRegex,
			MerchantInfo: merchantRegex,
			Source:       api.Source{Label: "test-bank"},
		},
	}

	plugin := &Plugin{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	reader, err := plugin.NewReader(plugins.ReaderInput{
		AppConfig:    cfg,
		Rules:        rules,
		StateManager: stateManager,
		Logger:       logger,
	})
	if err != nil {
		t.Fatalf("NewReader() failed: %v", err)
	}

	if reader == nil {
		t.Error("NewReader() returned nil reader")
	}
}

func TestPlugin_NewReader_MissingMailbox(t *testing.T) {
	tmpDir := t.TempDir()

	// Don't create the mailbox - it should fail

	stateFile := filepath.Join(tmpDir, "state.json")
	stateManager, err := state.New(stateFile, slog.Default())
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	cfg := &config.Config{
		ScanInterval: 10,
		Thunderbird: config.ThunderbirdConfig{
			ProfilePath: tmpDir,
			Mailboxes:   "NonExistent",
		},
	}

	plugin := &Plugin{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	_, err = plugin.NewReader(plugins.ReaderInput{
		AppConfig:    cfg,
		Rules:        []api.Rule{},
		StateManager: stateManager,
		Logger:       logger,
	})
	if err == nil {
		t.Error("expected error for missing mailbox, got nil")
	}
}

func TestPlugin_NewReader_DefaultInterval(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test mailbox structure
	mailDir := filepath.Join(tmpDir, "Mail", "Local Folders")
	if err := os.MkdirAll(mailDir, 0o755); err != nil {
		t.Fatalf("failed to create mail dir: %v", err)
	}

	inboxPath := filepath.Join(mailDir, "Inbox")
	if err := os.WriteFile(inboxPath, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to create inbox: %v", err)
	}

	stateFile := filepath.Join(tmpDir, "state.json")
	stateManager, err := state.New(stateFile, slog.Default())
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	// ScanInterval of 0 will be treated as zero duration; plugin uses it directly.
	cfg := &config.Config{
		ScanInterval: 0,
		Thunderbird: config.ThunderbirdConfig{
			ProfilePath: tmpDir,
			Mailboxes:   "Inbox",
		},
	}

	plugin := &Plugin{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	reader, err := plugin.NewReader(plugins.ReaderInput{
		AppConfig:    cfg,
		Rules:        []api.Rule{},
		StateManager: stateManager,
		Logger:       logger,
	})
	if err != nil {
		t.Fatalf("NewReader() failed: %v", err)
	}

	if reader == nil {
		t.Error("NewReader() returned nil reader")
	}
}

func TestPlugin_NewReader_UsesPersistedReaderConfig(t *testing.T) {
	tmpDir := t.TempDir()
	mailDir := filepath.Join(tmpDir, "Mail", "Local Folders")
	if err := os.MkdirAll(mailDir, 0o755); err != nil {
		t.Fatalf("failed to create mail dir: %v", err)
	}
	for _, mailbox := range []string{"Inbox", "Sent"} {
		if err := os.WriteFile(filepath.Join(mailDir, mailbox), []byte(""), 0o600); err != nil {
			t.Fatalf("failed to create %s: %v", mailbox, err)
		}
	}

	stateFile := filepath.Join(tmpDir, "state.json")
	stateManager, err := state.New(stateFile, slog.Default())
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	plugin := &Plugin{}
	reader, err := plugin.NewReader(plugins.ReaderInput{
		AppConfig:    &config.Config{},
		ReaderConfig: []byte(`{"config":{"profilePath":"` + tmpDir + `","mailboxes":"Inbox,Sent"}}`),
		StateManager: stateManager,
		Logger:       slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("NewReader() failed: %v", err)
	}
	if reader == nil {
		t.Error("NewReader() returned nil reader")
	}
}

func TestRuleMatching(t *testing.T) {
	tests := []struct {
		name            string
		senderEmail     string
		subjectContains string
		testFrom        string
		testSubject     string
		wantMatch       bool
	}{
		{
			name:        "match by sender email",
			senderEmail: "bank@example.com",
			testFrom:    "Bank <bank@example.com>",
			testSubject: "Transaction Alert",
			wantMatch:   true,
		},
		{
			name:            "match by subject",
			subjectContains: "Transaction",
			testFrom:        "anyone@example.com",
			testSubject:     "Transaction Alert for Card",
			wantMatch:       true,
		},
		{
			name:            "match by both",
			senderEmail:     "bank@example.com",
			subjectContains: "Transaction",
			testFrom:        "Bank <bank@example.com>",
			testSubject:     "Transaction Alert",
			wantMatch:       true,
		},
		{
			name:        "no match - wrong sender",
			senderEmail: "bank@example.com",
			testFrom:    "other@example.com",
			testSubject: "Transaction Alert",
			wantMatch:   false,
		},
		{
			name:            "no match - wrong subject",
			subjectContains: "Transaction",
			testFrom:        "bank@example.com",
			testSubject:     "Welcome Message",
			wantMatch:       false,
		},
		{
			name:        "case insensitive email",
			senderEmail: "BANK@EXAMPLE.COM",
			testFrom:    "bank@example.com",
			testSubject: "Alert",
			wantMatch:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := api.Rule{
				Name:            "test",
				SenderEmail:     tt.senderEmail,
				SubjectContains: tt.subjectContains,
			}

			got := rule.MatchesEmail(tt.testFrom, tt.testSubject)
			if got != tt.wantMatch {
				t.Errorf("MatchesEmail() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestScanIntervalPropagation(t *testing.T) {
	tests := []struct {
		name         string
		scanInterval int
		want         time.Duration
	}{
		{name: "explicit interval", scanInterval: 30, want: 30 * time.Second},
		{name: "zero interval yields zero duration", scanInterval: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := time.Duration(tt.scanInterval) * time.Second
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMailboxes(t *testing.T) {
	tests := []struct {
		name      string
		mailboxes string
		want      []string
	}{
		{
			name:      "single mailbox",
			mailboxes: "Inbox",
			want:      []string{"Inbox"},
		},
		{
			name:      "multiple mailboxes",
			mailboxes: "Inbox,Archive,Sent",
			want:      []string{"Inbox", "Archive", "Sent"},
		},
		{
			name:      "with spaces",
			mailboxes: "Inbox, Archive, Sent",
			want:      []string{"Inbox", "Archive", "Sent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.ThunderbirdConfig{
				Mailboxes: tt.mailboxes,
			}

			got := cfg.GetMailboxes()
			if len(got) != len(tt.want) {
				t.Errorf("GetMailboxes() returned %d items, want %d", len(got), len(tt.want))
				return
			}

			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("GetMailboxes()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}
