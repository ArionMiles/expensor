package thunderbird

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

func TestPlugin_Name(t *testing.T) {
	plugin := &Plugin{}
	if got := plugin.Name(); got != "thunderbird" {
		t.Errorf("Name() = %q, want %q", got, "thunderbird")
	}
}

func TestPlugin_Description(t *testing.T) {
	plugin := &Plugin{}
	desc := plugin.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestPlugin_RequiredScopes(t *testing.T) {
	plugin := &Plugin{}
	scopes := plugin.RequiredScopes()
	if len(scopes) != 0 {
		t.Errorf("RequiredScopes() should be empty for Thunderbird, got %v", scopes)
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
		Thunderbird: config.ThunderbirdConfig{
			ProfilePath: tmpDir,
			Mailboxes:   "Inbox",
			Interval:    10,
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
			Enabled:      true,
			Source:       "test-bank",
		},
	}

	// Create labels
	labels := api.Labels{
		"Amazon": {Category: "Shopping", Bucket: "Want"},
	}

	plugin := &Plugin{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	reader, err := plugin.NewReader(nil, cfg, rules, labels, stateManager, logger)
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
		Thunderbird: config.ThunderbirdConfig{
			ProfilePath: tmpDir,
			Mailboxes:   "NonExistent",
			Interval:    10,
		},
	}

	plugin := &Plugin{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	_, err = plugin.NewReader(nil, cfg, []api.Rule{}, make(api.Labels), stateManager, logger)
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

	// Interval of 0 should default to 60 seconds
	cfg := &config.Config{
		Thunderbird: config.ThunderbirdConfig{
			ProfilePath: tmpDir,
			Mailboxes:   "Inbox",
			Interval:    0,
		},
	}

	plugin := &Plugin{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	reader, err := plugin.NewReader(nil, cfg, []api.Rule{}, make(api.Labels), stateManager, logger)
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
				Enabled:         true,
			}

			got := rule.MatchesEmail(tt.testFrom, tt.testSubject)
			if got != tt.wantMatch {
				t.Errorf("MatchesEmail() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestGetInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval int
		want     time.Duration
	}{
		{
			name:     "explicit interval",
			interval: 30,
			want:     30 * time.Second,
		},
		{
			name:     "default interval",
			interval: 0,
			want:     60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.ThunderbirdConfig{
				Interval: tt.interval,
			}

			got := cfg.GetInterval()
			if got != tt.want {
				t.Errorf("GetInterval() = %v, want %v", got, tt.want)
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
