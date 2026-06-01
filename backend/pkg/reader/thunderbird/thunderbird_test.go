package thunderbird

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

type recordingDiagnosticSink struct {
	mu          sync.Mutex
	diagnostics []api.ExtractionDiagnostic
	err         error
	recorded    chan struct{}
}

func (s *recordingDiagnosticSink) RecordExtractionDiagnostic(_ context.Context, diagnostic api.ExtractionDiagnostic) error {
	s.mu.Lock()
	s.diagnostics = append(s.diagnostics, diagnostic)
	recorded := s.recorded
	s.mu.Unlock()
	if recorded != nil {
		close(recorded)
	}
	return s.err
}

func (s *recordingDiagnosticSink) waitForDiagnostic(t *testing.T) api.ExtractionDiagnostic {
	t.Helper()
	select {
	case <-s.recorded:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for diagnostic")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.diagnostics) != 1 {
		t.Fatalf("recorded diagnostics = %d, want 1", len(s.diagnostics))
	}
	return s.diagnostics[0]
}

type fakeProcessedMessageStore struct {
	mu        sync.Mutex
	processed map[string]time.Time
}

func newTestStateManager() *state.Manager {
	return state.NewDBManager(&fakeProcessedMessageStore{processed: map[string]time.Time{}}, slog.Default())
}

func (f *fakeProcessedMessageStore) IsMessageProcessed(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.processed[key]
	return ok, nil
}

func (f *fakeProcessedMessageStore) MarkMessageProcessed(_ context.Context, key string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processed[key] = at
	return nil
}

type blockingDiagnosticSink struct {
	started chan struct{}
	release chan struct{}
	mu      sync.Mutex
	calls   int
}

func (s *blockingDiagnosticSink) RecordExtractionDiagnostic(ctx context.Context, _ api.ExtractionDiagnostic) error {
	s.mu.Lock()
	s.calls++
	if s.calls == 1 {
		close(s.started)
	}
	s.mu.Unlock()
	<-s.release
	return ctx.Err()
}

func (s *blockingDiagnosticSink) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

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
				Resolver:    nil,
				State:       nil,
				Interval:    10 * time.Second,
			},
			wantErr: false,
			setupFunc: func(t *testing.T, cfg *Config, tmpDir string) {
				t.Helper()
				cfg.ProfilePath = tmpDir

				cfg.State = newTestStateManager()

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
				Resolver:    nil,
				State:       nil,
			},
			wantErr: true,
			errMsg:  "mailbox not found",
			setupFunc: func(t *testing.T, cfg *Config, tmpDir string) {
				t.Helper()
				cfg.ProfilePath = tmpDir

				cfg.State = newTestStateManager()
			},
		},
		{
			name: "default interval",
			cfg: Config{
				ProfilePath: "",
				Mailboxes:   []string{"Inbox"},
				Rules:       []api.Rule{},
				Resolver:    nil,
				State:       nil,
				Interval:    0, // Should default to 60s
			},
			wantErr: false,
			setupFunc: func(t *testing.T, cfg *Config, tmpDir string) {
				t.Helper()
				cfg.ProfilePath = tmpDir

				cfg.State = newTestStateManager()

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
				},
			},
			fromHeader: "other@example.com",
			subjHeader: "Transaction Alert",
			wantMatch:  false,
		},
		{
			name: "no match - sender suffix is not exact address",
			rules: []api.Rule{
				{
					Name:         "test rule",
					SenderEmails: []string{"alerts@hdfcbank.net"},
				},
			},
			fromHeader: "alerts@hdfcbank.net.evil.example",
			subjHeader: "Transaction Alert",
			wantMatch:  false,
		},
		{
			name: "no match - wrong subject",
			rules: []api.Rule{
				{
					Name:            "test rule",
					SubjectContains: "spent",
				},
			},
			fromHeader: "anyone@example.com",
			subjHeader: "Welcome",
			wantMatch:  false,
		},
		{
			name: "case insensitive matching",
			rules: []api.Rule{
				{
					Name:        "test rule",
					SenderEmail: "BANK@EXAMPLE.COM",
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

	stateManager := newTestStateManager()

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
				Source:          api.Source{Label: "test-bank"},
			},
		},
		resolver: nil,
		state:    stateManager,
		logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
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

	err := reader.scanMailbox(ctx, "test", mboxPath, out)
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
		if txn.Source.Label != "test-bank" {
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

func TestProcessMessage_RecordsExtractionDiagnostic(t *testing.T) {
	msg, err := parseTestMessage(
		"From: Bank Alerts <alerts@example.com>\r\n" +
			"Subject: Transaction Alert\r\n" +
			"Message-ID: <diag@example.com>\r\n" +
			"Date: Mon, 1 Jan 2024 10:00:00 +0000\r\n\r\n" +
			"You spent at  using card",
	)
	if err != nil {
		t.Fatalf("parse test message: %v", err)
	}

	sink := &recordingDiagnosticSink{recorded: make(chan struct{})}
	reader := &Reader{
		rules: []api.Rule{
			{
				ID:              "rule-1",
				Name:            "Bank Rule",
				SenderEmail:     "alerts@example.com",
				SubjectContains: "Transaction",
				Amount:          regexp.MustCompile(`Rs\.([\d.]+)`),
				MerchantInfo:    regexp.MustCompile(`at (.*?) using`),
				Source:          api.Source{Label: "bank-card"},
			},
		},
		diagnosticSink: sink,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	out := make(chan *api.TransactionDetails, 1)

	handled, err := reader.processMessage(context.Background(), msg, "/tmp/test.mbox", out)
	if err != nil {
		t.Fatalf("processMessage returned error: %v", err)
	}
	if !handled {
		t.Fatal("processMessage handled = false, want true")
	}
	got := sink.waitForDiagnostic(t)
	wantMessageID := state.GenerateKey("/tmp/test.mbox", msg.Header.Get("Message-Id"), msg.Header.Get("Date"))
	if got.Reader != "thunderbird" || got.MessageID != wantMessageID || got.Source != "bank-card" {
		t.Fatalf("diagnostic identity = (%q, %q, %q), want thunderbird/%s/bank-card", got.Reader, got.MessageID, got.Source, wantMessageID)
	}
	if got.RuleID != "rule-1" || got.RuleName != "Bank Rule" {
		t.Fatalf("diagnostic rule = (%q, %q), want rule-1/Bank Rule", got.RuleID, got.RuleName)
	}
	if got.AmountRegex != `Rs\.([\d.]+)` || got.MerchantRegex != `at (.*?) using` {
		t.Fatalf("diagnostic regexes = (%q, %q)", got.AmountRegex, got.MerchantRegex)
	}
	if got.Subject != "Transaction Alert" || got.Sender != "Bank Alerts <alerts@example.com>" || got.SenderEmail != "alerts@example.com" {
		t.Fatalf("diagnostic headers = subject %q sender %q sender email %q", got.Subject, got.Sender, got.SenderEmail)
	}
	if got.EmailBody != "You spent at  using card" {
		t.Fatalf("diagnostic body = %q", got.EmailBody)
	}
	if got.ReceivedAt == nil {
		t.Fatal("diagnostic ReceivedAt is nil")
	}
	if !containsReason(got.FailureReasons, api.FailureAmountZero) || !containsReason(got.FailureReasons, api.FailureMerchantEmpty) {
		t.Fatalf("failure reasons = %v, want amount_zero and merchant_empty", got.FailureReasons)
	}
}

func TestProcessMessage_BlockingDiagnosticSinkDoesNotDelayEmission(t *testing.T) {
	msg, err := parseTestMessage(
		"From: Bank Alerts <alerts@example.com>\r\n" +
			"Subject: Transaction Alert\r\n" +
			"Message-ID: <blocking-sink@example.com>\r\n" +
			"Date: Mon, 1 Jan 2024 10:00:00 +0000\r\n\r\n" +
			"You spent at  using card",
	)
	if err != nil {
		t.Fatalf("parse test message: %v", err)
	}

	sink := &blockingDiagnosticSink{started: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(sink.release) })
	reader := &Reader{
		rules: []api.Rule{
			{
				SenderEmail:  "alerts@example.com",
				Amount:       regexp.MustCompile(`Rs\.([\d.]+)`),
				MerchantInfo: regexp.MustCompile(`at (.*?) using`),
				Source:       api.Source{Label: "bank-card"},
			},
		},
		diagnosticSink: sink,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	out := make(chan *api.TransactionDetails, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct {
		handled bool
		err     error
	}, 1)
	go func() {
		handled, err := reader.processMessage(ctx, msg, "/tmp/test.mbox", out)
		done <- struct {
			handled bool
			err     error
		}{handled: handled, err: err}
	}()

	select {
	case <-sink.started:
	case <-time.After(500 * time.Millisecond):
		cancel()
		t.Fatal("diagnostic sink was not called")
	}

	select {
	case tx := <-out:
		if tx.Source.Label != "bank-card" {
			t.Fatalf("transaction source = %q, want bank-card", tx.Source)
		}
	case <-time.After(250 * time.Millisecond):
		cancel()
		t.Fatal("blocking diagnostic sink delayed transaction emission")
	}

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("processMessage returned error: %v", result.err)
		}
		if !result.handled {
			t.Fatal("processMessage handled = false, want true")
		}
	case <-time.After(250 * time.Millisecond):
		cancel()
		t.Fatal("blocking diagnostic sink delayed processMessage return")
	}
}

func TestProcessMessage_FullDiagnosticLimiterSkipsWithoutBlocking(t *testing.T) {
	firstMsg, err := parseTestMessage(
		"From: Bank Alerts <alerts@example.com>\r\n" +
			"Subject: Transaction Alert\r\n" +
			"Message-ID: <limiter-1@example.com>\r\n" +
			"Date: Mon, 1 Jan 2024 10:00:00 +0000\r\n\r\n" +
			"You spent at  using card",
	)
	if err != nil {
		t.Fatalf("parse first message: %v", err)
	}
	secondMsg, err := parseTestMessage(
		"From: Bank Alerts <alerts@example.com>\r\n" +
			"Subject: Transaction Alert\r\n" +
			"Message-ID: <limiter-2@example.com>\r\n" +
			"Date: Mon, 1 Jan 2024 10:01:00 +0000\r\n\r\n" +
			"You spent at  using card",
	)
	if err != nil {
		t.Fatalf("parse second message: %v", err)
	}

	sink := &blockingDiagnosticSink{started: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(sink.release) })
	reader := &Reader{
		rules: []api.Rule{
			{
				SenderEmail:  "alerts@example.com",
				Amount:       regexp.MustCompile(`Rs\.([\d.]+)`),
				MerchantInfo: regexp.MustCompile(`at (.*?) using`),
				Source:       api.Source{Label: "bank-card"},
			},
		},
		diagnosticSink:  sink,
		diagnosticSlots: make(chan struct{}, 1),
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	out := make(chan *api.TransactionDetails, 2)

	handled, err := reader.processMessage(context.Background(), firstMsg, "/tmp/test.mbox", out)
	if err != nil {
		t.Fatalf("first processMessage returned error: %v", err)
	}
	if !handled {
		t.Fatal("first processMessage handled = false, want true")
	}
	select {
	case <-sink.started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first diagnostic sink call did not start")
	}
	if got := sink.callCount(); got != 1 {
		t.Fatalf("diagnostic sink calls after first message = %d, want 1", got)
	}

	done := make(chan struct {
		handled bool
		err     error
	}, 1)
	go func() {
		handled, err := reader.processMessage(context.Background(), secondMsg, "/tmp/test.mbox", out)
		done <- struct {
			handled bool
			err     error
		}{handled: handled, err: err}
	}()
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("second processMessage returned error: %v", result.err)
		}
		if !result.handled {
			t.Fatal("second processMessage handled = false, want true")
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("full diagnostic limiter blocked message processing")
	}
	if got := sink.callCount(); got != 1 {
		t.Fatalf("diagnostic sink calls with full limiter = %d, want 1", got)
	}
}

func TestProcessMessage_DiagnosticSinkErrorIsBestEffort(t *testing.T) {
	msg, err := parseTestMessage(
		"From: Bank Alerts <alerts@example.com>\r\n" +
			"Subject: Transaction Alert\r\n" +
			"Message-ID: <sink-error@example.com>\r\n" +
			"Date: Mon, 1 Jan 2024 10:00:00 +0000\r\n\r\n" +
			"You spent at Store using card",
	)
	if err != nil {
		t.Fatalf("parse test message: %v", err)
	}

	reader := &Reader{
		rules: []api.Rule{
			{
				SenderEmail:  "alerts@example.com",
				Amount:       regexp.MustCompile(`Rs\.([\d.]+)`),
				MerchantInfo: regexp.MustCompile(`at (.*?) using`),
				Source:       api.Source{Label: "bank-card"},
			},
		},
		diagnosticSink: &recordingDiagnosticSink{err: errors.New("store down")},
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	out := make(chan *api.TransactionDetails, 1)

	handled, err := reader.processMessage(context.Background(), msg, "/tmp/test.mbox", out)
	if err != nil {
		t.Fatalf("processMessage returned error: %v", err)
	}
	if !handled {
		t.Fatal("processMessage handled = false, want true")
	}
	if tx := <-out; tx.MerchantInfo != "Store" {
		t.Fatalf("transaction merchant = %q, want Store", tx.MerchantInfo)
	}
}

// parseTestMessage is a helper to parse a test message string.
func parseTestMessage(msgStr string) (*mail.Message, error) {
	return mail.ReadMessage(strings.NewReader(msgStr))
}

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}

func TestReadWithContext(t *testing.T) {
	tmpDir := t.TempDir()
	mboxPath := filepath.Join(tmpDir, "test.mbox")

	// Create a simple test message
	messages := []string{
		"From: bank@example.com\r\nSubject: Transaction Alert\r\nMessage-ID: <msg1@example.com>\r\nDate: Mon, 1 Jan 2024 10:00:00 +0000\r\n\r\nYou spent Rs. 100 at Store",
	}
	createTestMbox(t, mboxPath, messages)

	stateManager := newTestStateManager()

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
				Source:       api.Source{Label: "test"},
			},
		},
		resolver: nil,
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
	err := <-errChan
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
