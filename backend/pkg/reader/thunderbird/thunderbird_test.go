package thunderbird

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

// createTestMbox creates a test MBOX file with sample messages.
func createTestMbox(t *testing.T, path string, messages []string) {
	t.Helper()

	var content strings.Builder
	for _, msg := range messages {
		// Add mbox separator
		content.WriteString("From test@example.com Mon Jan  1 00:00:00 2024\n")
		content.WriteString(msg)
		content.WriteString("\n\n")
	}

	if err := os.WriteFile(path, []byte(content.String()), 0o600); err != nil {
		t.Fatalf("failed to create test mbox: %v", err)
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		wantErr   bool
		errMsg    string
		setupFunc func(t *testing.T, cfg *Config, tmpDir string)
	}{
		{
			name: "valid config",
			cfg: Config{
				ProfilePath: "",
				Mailboxes:   []string{"Inbox"},
				Rules:       []api.Rule{},
				Labels:      make(api.Labels),
				State:       nil,
				Interval:    10 * time.Second,
			},
			wantErr: false,
			setupFunc: func(t *testing.T, cfg *Config, tmpDir string) {
				t.Helper()
				cfg.ProfilePath = tmpDir

				// Create state manager
				stateFile := filepath.Join(tmpDir, "state.json")
				stateManager, err := state.New(stateFile, slog.Default())
				if err != nil {
					t.Fatalf("failed to create state manager: %v", err)
				}
				cfg.State = stateManager

				// Create mock mailbox
				mailDir := filepath.Join(tmpDir, "Mail", "Local Folders")
				if err := os.MkdirAll(mailDir, 0o755); err != nil {
					t.Fatalf("failed to create mail dir: %v", err)
				}
				inboxPath := filepath.Join(mailDir, "Inbox")
				if err := os.WriteFile(inboxPath, []byte(""), 0o600); err != nil {
					t.Fatalf("failed to create inbox: %v", err)
				}
			},
		},
		{
			name: "missing mailbox",
			cfg: Config{
				ProfilePath: "",
				Mailboxes:   []string{"NonExistent"},
				Rules:       []api.Rule{},
				Labels:      make(api.Labels),
				State:       nil,
			},
			wantErr: true,
			errMsg:  "mailbox not found",
			setupFunc: func(t *testing.T, cfg *Config, tmpDir string) {
				t.Helper()
				cfg.ProfilePath = tmpDir

				stateFile := filepath.Join(tmpDir, "state.json")
				stateManager, err := state.New(stateFile, slog.Default())
				if err != nil {
					t.Fatalf("failed to create state manager: %v", err)
				}
				cfg.State = stateManager
			},
		},
		{
			name: "default interval",
			cfg: Config{
				ProfilePath: "",
				Mailboxes:   []string{"Inbox"},
				Rules:       []api.Rule{},
				Labels:      make(api.Labels),
				State:       nil,
				Interval:    0, // Should default to 60s
			},
			wantErr: false,
			setupFunc: func(t *testing.T, cfg *Config, tmpDir string) {
				t.Helper()
				cfg.ProfilePath = tmpDir

				stateFile := filepath.Join(tmpDir, "state.json")
				stateManager, err := state.New(stateFile, slog.Default())
				if err != nil {
					t.Fatalf("failed to create state manager: %v", err)
				}
				cfg.State = stateManager

				mailDir := filepath.Join(tmpDir, "Mail", "Local Folders")
				if err := os.MkdirAll(mailDir, 0o755); err != nil {
					t.Fatalf("failed to create mail dir: %v", err)
				}
				inboxPath := filepath.Join(mailDir, "Inbox")
				if err := os.WriteFile(inboxPath, []byte(""), 0o600); err != nil {
					t.Fatalf("failed to create inbox: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if tt.setupFunc != nil {
				tt.setupFunc(t, &tt.cfg, tmpDir)
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			reader, err := New(tt.cfg, logger)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if reader == nil {
					t.Error("expected reader, got nil")
				}
				if reader != nil && reader.interval == 0 {
					t.Error("reader interval should not be zero")
				}
			}
		})
	}
}

func TestMatchesRule(t *testing.T) {
	tests := []struct {
		name        string
		rules       []api.Rule
		fromHeader  string
		subjHeader  string
		wantMatch   bool
		wantRuleIdx int
	}{
		{
			name: "match by sender",
			rules: []api.Rule{
				{
					Name:        "test rule",
					SenderEmail: "bank@example.com",
					Enabled:     true,
				},
			},
			fromHeader:  "Bank <bank@example.com>",
			subjHeader:  "Transaction Alert",
			wantMatch:   true,
			wantRuleIdx: 0,
		},
		{
			name: "match by subject",
			rules: []api.Rule{
				{
					Name:            "test rule",
					SubjectContains: "spent",
					Enabled:         true,
				},
			},
			fromHeader:  "anyone@example.com",
			subjHeader:  "You spent Rs. 500",
			wantMatch:   true,
			wantRuleIdx: 0,
		},
		{
			name: "match by both sender and subject",
			rules: []api.Rule{
				{
					Name:            "test rule",
					SenderEmail:     "bank@example.com",
					SubjectContains: "spent",
					Enabled:         true,
				},
			},
			fromHeader:  "Bank <bank@example.com>",
			subjHeader:  "You spent Rs. 500",
			wantMatch:   true,
			wantRuleIdx: 0,
		},
		{
			name: "no match - wrong sender",
			rules: []api.Rule{
				{
					Name:        "test rule",
					SenderEmail: "bank@example.com",
					Enabled:     true,
				},
			},
			fromHeader: "other@example.com",
			subjHeader: "Transaction Alert",
			wantMatch:  false,
		},
		{
			name: "no match - wrong subject",
			rules: []api.Rule{
				{
					Name:            "test rule",
					SubjectContains: "spent",
					Enabled:         true,
				},
			},
			fromHeader: "anyone@example.com",
			subjHeader: "Welcome",
			wantMatch:  false,
		},
		{
			name: "skip disabled rule",
			rules: []api.Rule{
				{
					Name:        "disabled rule",
					SenderEmail: "bank@example.com",
					Enabled:     false,
				},
			},
			fromHeader: "Bank <bank@example.com>",
			subjHeader: "Transaction Alert",
			wantMatch:  false,
		},
		{
			name: "case insensitive matching",
			rules: []api.Rule{
				{
					Name:        "test rule",
					SenderEmail: "BANK@EXAMPLE.COM",
					Enabled:     true,
				},
			},
			fromHeader: "bank@example.com",
			subjHeader: "Transaction Alert",
			wantMatch:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &Reader{
				rules:  tt.rules,
				logger: slog.Default(),
			}

			// Create a mock message
			msgStr := fmt.Sprintf("From: %s\r\nSubject: %s\r\n\r\nBody", tt.fromHeader, tt.subjHeader)
			msg, err := parseTestMessage(msgStr)
			if err != nil {
				t.Fatalf("failed to parse test message: %v", err)
			}

			rule, matches := reader.matchesRule(msg)

			if matches != tt.wantMatch {
				t.Errorf("expected match=%v, got %v", tt.wantMatch, matches)
			}

			if matches && tt.wantMatch && rule.Name != tt.rules[tt.wantRuleIdx].Name {
				t.Errorf("expected rule %q, got %q", tt.rules[tt.wantRuleIdx].Name, rule.Name)
			}
		})
	}
}

func TestScanMailbox(t *testing.T) {
	tmpDir := t.TempDir()
	mboxPath := filepath.Join(tmpDir, "test.mbox")

	// Create test messages
	messages := []string{
		"From: bank@example.com\r\nSubject: Transaction Alert\r\nMessage-ID: <msg1@example.com>\r\nDate: Mon, 1 Jan 2024 10:00:00 +0000\r\n\r\nYou spent Rs. 1,234.56 at Amazon",
		"From: bank@example.com\r\nSubject: Transaction Alert\r\nMessage-ID: <msg2@example.com>\r\nDate: Mon, 1 Jan 2024 11:00:00 +0000\r\n\r\nYou spent Rs. 500.00 at Walmart",
		"From: other@example.com\r\nSubject: Newsletter\r\nMessage-ID: <msg3@example.com>\r\nDate: Mon, 1 Jan 2024 12:00:00 +0000\r\n\r\nWelcome to our newsletter",
	}

	createTestMbox(t, mboxPath, messages)

	// Create state manager
	stateFile := filepath.Join(tmpDir, "state.json")
	stateManager, err := state.New(stateFile, slog.Default())
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	// Create reader with rules
	amountRegex := regexp.MustCompile(`Rs\.\s*([\d,]+\.?\d*)`)
	merchantRegex := regexp.MustCompile(`at\s+(\w+)`)

	reader := &Reader{
		mailboxPaths: map[string]string{"test": mboxPath},
		rules: []api.Rule{
			{
				Name:            "bank rule",
				SenderEmail:     "bank@example.com",
				SubjectContains: "Transaction",
				Amount:          amountRegex,
				MerchantInfo:    merchantRegex,
				Enabled:         true,
				Source:          "test-bank",
			},
		},
		labels: make(api.Labels),
		state:  stateManager,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	ctx := context.Background()
	out := make(chan *api.TransactionDetails, 10)
	done := make(chan struct{})

	var transactions []*api.TransactionDetails
	go func() {
		for txn := range out {
			transactions = append(transactions, txn)
		}
		close(done)
	}()

	err = reader.scanMailbox(ctx, "test", mboxPath, out)
	close(out)
	<-done

	if err != nil {
		t.Fatalf("scanMailbox failed: %v", err)
	}

	// Should have extracted 2 transactions (first 2 messages match the rule)
	if len(transactions) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(transactions))
	}

	// Verify first transaction
	if len(transactions) >= 1 {
		txn := transactions[0]
		if txn.Amount != 1234.56 {
			t.Errorf("expected amount 1234.56, got %v", txn.Amount)
		}
		if txn.MerchantInfo != "Amazon" {
			t.Errorf("expected merchant Amazon, got %q", txn.MerchantInfo)
		}
		if txn.Source != "test-bank" {
			t.Errorf("expected source test-bank, got %q", txn.Source)
		}
	}
}

func TestGenerateMessageKey(t *testing.T) {
	msgStr := "From: test@example.com\r\nMessage-ID: <unique@example.com>\r\nDate: Mon, 1 Jan 2024 10:00:00 +0000\r\n\r\nBody"
	msg, err := parseTestMessage(msgStr)
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	messageID := msg.Header.Get("Message-Id")
	dateStr := msg.Header.Get("Date")

	key1 := state.GenerateKey("/path/to/mbox", messageID, dateStr)
	key2 := state.GenerateKey("/path/to/mbox", messageID, dateStr)

	// Same message should generate same key
	if key1 != key2 {
		t.Errorf("expected same key for same message, got %q and %q", key1, key2)
	}

	// Different path should generate different key
	key3 := state.GenerateKey("/different/path", messageID, dateStr)
	if key1 == key3 {
		t.Error("expected different key for different mailbox path")
	}

	// Key should be non-empty and look like a hash
	if len(key1) != 64 { // SHA256 produces 64 hex chars
		t.Errorf("expected 64 character hash, got %d characters", len(key1))
	}
}

// parseTestMessage is a helper to parse a test message string.
func parseTestMessage(msgStr string) (*mail.Message, error) {
	return mail.ReadMessage(strings.NewReader(msgStr))
}

func TestReadWithContext(t *testing.T) {
	tmpDir := t.TempDir()
	mboxPath := filepath.Join(tmpDir, "test.mbox")

	// Create a simple test message
	messages := []string{
		"From: bank@example.com\r\nSubject: Transaction Alert\r\nMessage-ID: <msg1@example.com>\r\nDate: Mon, 1 Jan 2024 10:00:00 +0000\r\n\r\nYou spent Rs. 100 at Store",
	}
	createTestMbox(t, mboxPath, messages)

	// Create state manager
	stateFile := filepath.Join(tmpDir, "state.json")
	stateManager, err := state.New(stateFile, slog.Default())
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	amountRegex := regexp.MustCompile(`Rs\.\s*([\d,]+\.?\d*)`)
	merchantRegex := regexp.MustCompile(`at\s+(\w+)`)

	reader := &Reader{
		mailboxPaths: map[string]string{"test": mboxPath},
		rules: []api.Rule{
			{
				Name:         "test rule",
				SenderEmail:  "bank@example.com",
				Amount:       amountRegex,
				MerchantInfo: merchantRegex,
				Enabled:      true,
				Source:       "test",
			},
		},
		labels:   make(api.Labels),
		state:    stateManager,
		interval: 100 * time.Millisecond,
		logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	out := make(chan *api.TransactionDetails, 10)
	ackChan := make(chan string, 10)

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.Read(ctx, out, ackChan)
	}()

	// Wait for context to cancel
	err = <-errChan
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}

	// Drain and check if channel is closed
	drained := false
	for range out {
		drained = true
	}
	// Channel was properly closed if we can iterate (even if empty)
	_ = drained
}
