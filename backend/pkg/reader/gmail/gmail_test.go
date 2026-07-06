package gmail

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/ArionMiles/expensor/backend/pkg/api"
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

func b64(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}

func TestEffectiveSince_ForceFullScanIgnoresLastScanAt(t *testing.T) {
	now := time.Now()
	lastScan := now
	reader := &Reader{
		lastScanAt:    &lastScan,
		forceFullScan: true,
		lookbackDays:  14,
	}

	got := reader.effectiveSince()
	wantEarliest := time.Now().AddDate(0, 0, -14).Add(-2 * time.Second)
	wantLatest := time.Now().AddDate(0, 0, -14).Add(2 * time.Second)
	if got.Before(wantEarliest) || got.After(wantLatest) {
		t.Fatalf("effectiveSince() with forceFullScan = %v, want near 14-day lookback", got)
	}
}

func TestEffectiveSince_NormalScanUsesCheckpointBuffer(t *testing.T) {
	lastScan := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	reader := &Reader{
		lastScanAt:    &lastScan,
		forceFullScan: false,
		lookbackDays:  14,
	}

	got := reader.effectiveSince()
	want := lastScan.Add(-time.Hour)
	if !got.Equal(want) {
		t.Fatalf("effectiveSince() = %v, want %v", got, want)
	}
}

func TestSaveCheckpointAfterSuccessfulIterationOnly(t *testing.T) {
	t.Run("successful iteration saves checkpoint", func(t *testing.T) {
		var saved bool
		reader := &Reader{
			onCheckpoint: func(time.Time) { saved = true },
		}

		reader.saveCheckpointAfterIteration(nil)

		if !saved {
			t.Fatal("expected checkpoint to be saved after successful iteration")
		}
	})

	t.Run("failed iteration leaves checkpoint untouched", func(t *testing.T) {
		var saved bool
		reader := &Reader{
			onCheckpoint: func(time.Time) { saved = true },
		}

		reader.saveCheckpointAfterIteration(errors.New("list messages: network unavailable"))

		if saved {
			t.Fatal("checkpoint was saved after a failed iteration")
		}
	})
}

func TestIsAuthErrorDetectsInvalidGrant(t *testing.T) {
	err := errors.New(`oauth2: "invalid_grant" "Token has been expired or revoked."`)
	if !isAuthError(err) {
		t.Fatal("expected invalid_grant token error to be classified as auth error")
	}
}

func TestIsAuthErrorDetectsGoogleAPIUnauthorized(t *testing.T) {
	err := &googleapi.Error{Code: http.StatusUnauthorized}
	if !isAuthError(err) {
		t.Fatal("expected googleapi 401 to be classified as auth error")
	}
}

func TestLogAPIError_InvalidGrantUsesErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	logAPIError(logger, "failed to list messages", errors.New(`oauth2: "invalid_grant" "Token has been expired or revoked."`))

	out := buf.String()
	if !strings.Contains(out, "level=ERROR") {
		t.Fatalf("expected ERROR log level, got %s", out)
	}
	if !strings.Contains(out, "OAuth token invalid") {
		t.Fatalf("expected OAuth invalid guidance, got %s", out)
	}
}

func TestDoWithAuthRetryRetriesAuthErrorsOnce(t *testing.T) {
	attempts := 0
	errExpired := errors.New(`oauth2: "invalid_grant" "Token has been expired or revoked."`)
	err := doWithAuthRetry(func() error {
		attempts++
		if attempts == 1 {
			return errExpired
		}
		return nil
	})
	if err != nil {
		t.Fatalf("doWithAuthRetry returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestSearchMessages_ReturnsSubjectMatches(t *testing.T) {
	var listQuery string
	body := b64("INR 42.00 at Coffee")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/gmail/v1/users/me/messages":
			listQuery = r.URL.Query().Get("q")
			if r.URL.Query().Get("maxResults") != "5" {
				t.Fatalf("maxResults = %q, want 5", r.URL.Query().Get("maxResults"))
			}
			if _, err := w.Write([]byte(`{"messages":[{"id":"msg-1"}]}`)); err != nil {
				t.Fatalf("write list response: %v", err)
			}
		case "/gmail/v1/users/me/messages/msg-1":
			if _, err := fmt.Fprintf(w, `{
				"id": "msg-1",
				"snippet": "INR 42.00 at Coffee",
				"internalDate": "1780309800000",
				"payload": {
					"headers": [
						{"name": "Subject", "value": "Card spend approved"},
						{"name": "From", "value": "Bank Alerts <alerts@example.com>"}
					],
					"body": {"data": %q}
				}
			}`, body); err != nil {
				t.Fatalf("write get response: %v", err)
			}
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	svc, err := gmail.NewService(
		context.Background(),
		option.WithHTTPClient(server.Client()),
		option.WithEndpoint(server.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	reader := &Reader{client: svc, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	messages, err := reader.Search(context.Background(), api.EmailSearchQuery{
		SubjectQuery: "spend",
		Limit:        5,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if listQuery != `subject:"spend"` {
		t.Fatalf("query = %q, want subject search", listQuery)
	}
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(messages))
	}
	got := messages[0]
	if got.ID != "msg-1" || got.SenderEmail != "alerts@example.com" || got.Body != "INR 42.00 at Coffee" {
		t.Fatalf("unexpected message: %#v", got)
	}
	if got.ReceivedAt == nil || got.ReceivedAt.UTC().Format(time.RFC3339) != "2026-06-01T10:30:00Z" {
		t.Fatalf("received_at = %v, want 2026-06-01T10:30:00Z", got.ReceivedAt)
	}
}

func TestProcessMessage_RetriesGetAuthErrorOnce(t *testing.T) {
	var getCalls int
	body := b64("Paid Rs.12.34 at Test Merchant on card")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gmail/v1/users/me/messages/msg-1" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		getCalls++
		w.Header().Set("Content-Type", "application/json")
		if getCalls == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte(`{"error":{"code":401,"message":"invalid_grant"}}`)); err != nil {
				t.Fatalf("write response: %v", err)
			}
			return
		}
		if _, err := w.Write([]byte(`{
			"id": "msg-1",
			"internalDate": "1777298400000",
			"payload": {
				"headers": [{"name": "Subject", "value": "Test"}],
				"body": {"data": "` + body + `"}
			}
		}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	svc, err := gmail.NewService(
		context.Background(),
		option.WithHTTPClient(server.Client()),
		option.WithEndpoint(server.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	reader := &Reader{client: svc, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	out := make(chan *api.TransactionDetails, 1)
	err = reader.processMessage(context.Background(), "msg-1", api.Rule{
		Source:       api.Source{Label: "test"},
		Amount:       regexp.MustCompile(`Rs\.([\d.]+)`),
		MerchantInfo: regexp.MustCompile(`at (.*?) on`),
	}, out)
	if err != nil {
		t.Fatalf("processMessage returned error: %v", err)
	}
	if getCalls != 2 {
		t.Fatalf("get calls = %d, want 2", getCalls)
	}
	tx := <-out
	if tx.Amount != 12.34 {
		t.Fatalf("amount = %v, want 12.34", tx.Amount)
	}
}

func TestProcessMessage_RecordsExtractionDiagnostic(t *testing.T) {
	bodyText := "Paid at  on card"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gmail/v1/users/me/messages/msg-diagnostic" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{
			"id": "msg-diagnostic",
			"snippet": "Paid at on card",
			"internalDate": "1777298400000",
			"payload": {
				"headers": [
					{"name": "Subject", "value": "Card alert"},
					{"name": "From", "value": "Bank Alerts <alerts@example.com>"}
				],
				"body": {"data": %q}
			}
		}`, b64(bodyText)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	svc, err := gmail.NewService(
		context.Background(),
		option.WithHTTPClient(server.Client()),
		option.WithEndpoint(server.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	sink := &recordingDiagnosticSink{recorded: make(chan struct{})}
	reader := &Reader{
		client:         svc,
		diagnosticSink: sink,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	rule := api.Rule{
		ID:              "rule-1",
		Name:            "Card Rule",
		Source:          api.Source{Label: "credit-card"},
		SenderEmail:     "alerts@example.com",
		SubjectContains: "Card",
		Amount:          regexp.MustCompile(`Rs\.([\d.]+)`),
		MerchantInfo:    regexp.MustCompile(`at (.*?) on`),
	}
	out := make(chan *api.TransactionDetails, 1)

	if err := reader.processMessage(context.Background(), "msg-diagnostic", rule, out); err != nil {
		t.Fatalf("processMessage returned error: %v", err)
	}

	got := sink.waitForDiagnostic(t)
	if got.Reader != "gmail" || got.MessageID != "msg-diagnostic" || got.Source != "credit-card" {
		t.Fatalf("diagnostic identity = (%q, %q, %q), want gmail/msg-diagnostic/credit-card", got.Reader, got.MessageID, got.Source)
	}
	if got.RuleID != "rule-1" || got.RuleName != "Card Rule" {
		t.Fatalf("diagnostic rule = (%q, %q), want rule-1/Card Rule", got.RuleID, got.RuleName)
	}
	if got.AmountRegex != `Rs\.([\d.]+)` || got.MerchantRegex != `at (.*?) on` {
		t.Fatalf("diagnostic regexes = (%q, %q)", got.AmountRegex, got.MerchantRegex)
	}
	if got.Subject != "Card alert" || got.Sender != "Bank Alerts <alerts@example.com>" || got.SenderEmail != "alerts@example.com" {
		t.Fatalf("diagnostic headers = subject %q sender %q sender email %q", got.Subject, got.Sender, got.SenderEmail)
	}
	if got.EmailBody != bodyText || got.Snippet != "Paid at on card" {
		t.Fatalf("diagnostic body/snippet = (%q, %q)", got.EmailBody, got.Snippet)
	}
	if got.ReceivedAt == nil || !got.ReceivedAt.Equal(time.Unix(1777298400000/1000, 0)) {
		t.Fatalf("diagnostic received at = %v", got.ReceivedAt)
	}
	if !containsReason(got.FailureReasons, api.FailureAmountZero) || !containsReason(got.FailureReasons, api.FailureMerchantEmpty) {
		t.Fatalf("failure reasons = %v, want amount_zero and merchant_empty", got.FailureReasons)
	}
}

func TestProcessMessage_BlockingDiagnosticSinkDoesNotDelayEmission(t *testing.T) {
	bodyText := "Paid at  on card"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{
			"id": "msg-blocking-sink",
			"internalDate": "1777298400000",
			"payload": {
				"headers": [{"name": "Subject", "value": "Card alert"}],
				"body": {"data": %q}
			}
		}`, b64(bodyText)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	svc, err := gmail.NewService(
		context.Background(),
		option.WithHTTPClient(server.Client()),
		option.WithEndpoint(server.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	sink := &blockingDiagnosticSink{started: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(sink.release) })
	reader := &Reader{
		client:         svc,
		diagnosticSink: sink,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	out := make(chan *api.TransactionDetails, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- reader.processMessage(ctx, "msg-blocking-sink", api.Rule{
			Source:       api.Source{Label: "credit-card"},
			Amount:       regexp.MustCompile(`Rs\.([\d.]+)`),
			MerchantInfo: regexp.MustCompile(`at (.*?) on`),
		}, out)
	}()

	select {
	case <-sink.started:
	case <-time.After(500 * time.Millisecond):
		cancel()
		t.Fatal("diagnostic sink was not called")
	}

	select {
	case tx := <-out:
		if tx.MessageID != "msg-blocking-sink" {
			t.Fatalf("transaction message ID = %q, want msg-blocking-sink", tx.MessageID)
		}
	case <-time.After(250 * time.Millisecond):
		cancel()
		t.Fatal("blocking diagnostic sink delayed transaction emission")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("processMessage returned error: %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		cancel()
		t.Fatal("blocking diagnostic sink delayed processMessage return")
	}
}

func TestProcessMessage_FullDiagnosticLimiterSkipsWithoutBlocking(t *testing.T) {
	bodyText := "Paid at  on card"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{
			"id": %q,
			"internalDate": "1777298400000",
			"payload": {
				"headers": [{"name": "Subject", "value": "Card alert"}],
				"body": {"data": %q}
			}
		}`, strings.TrimPrefix(r.URL.Path, "/gmail/v1/users/me/messages/"), b64(bodyText)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	svc, err := gmail.NewService(
		context.Background(),
		option.WithHTTPClient(server.Client()),
		option.WithEndpoint(server.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	sink := &blockingDiagnosticSink{started: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(sink.release) })
	reader := &Reader{
		client:          svc,
		diagnosticSink:  sink,
		diagnosticSlots: make(chan struct{}, 1),
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	rule := api.Rule{
		Source:       api.Source{Label: "credit-card"},
		Amount:       regexp.MustCompile(`Rs\.([\d.]+)`),
		MerchantInfo: regexp.MustCompile(`at (.*?) on`),
	}
	out := make(chan *api.TransactionDetails, 2)

	if err := reader.processMessage(context.Background(), "msg-limiter-1", rule, out); err != nil {
		t.Fatalf("first processMessage returned error: %v", err)
	}
	select {
	case <-sink.started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first diagnostic sink call did not start")
	}
	if got := sink.callCount(); got != 1 {
		t.Fatalf("diagnostic sink calls after first message = %d, want 1", got)
	}

	done := make(chan error, 1)
	go func() {
		done <- reader.processMessage(context.Background(), "msg-limiter-2", rule, out)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second processMessage returned error: %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("full diagnostic limiter blocked message processing")
	}
	if got := sink.callCount(); got != 1 {
		t.Fatalf("diagnostic sink calls with full limiter = %d, want 1", got)
	}
}

func TestProcessMessage_DiagnosticSinkErrorIsBestEffort(t *testing.T) {
	bodyText := "Paid at Store on card"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{
			"id": "msg-sink-error",
			"internalDate": "1777298400000",
			"payload": {
				"headers": [{"name": "Subject", "value": "Card alert"}],
				"body": {"data": %q}
			}
		}`, b64(bodyText)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	svc, err := gmail.NewService(
		context.Background(),
		option.WithHTTPClient(server.Client()),
		option.WithEndpoint(server.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	reader := &Reader{
		client:         svc,
		diagnosticSink: &recordingDiagnosticSink{err: errors.New("store down")},
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	out := make(chan *api.TransactionDetails, 1)
	err = reader.processMessage(context.Background(), "msg-sink-error", api.Rule{
		Source:       api.Source{Label: "credit-card"},
		Amount:       regexp.MustCompile(`Rs\.([\d.]+)`),
		MerchantInfo: regexp.MustCompile(`at (.*?) on`),
	}, out)
	if err != nil {
		t.Fatalf("processMessage returned error: %v", err)
	}
	if tx := <-out; tx.MerchantInfo != "Store" {
		t.Fatalf("transaction merchant = %q, want Store", tx.MerchantInfo)
	}
}

func TestProcessRule_GetAuthFailureLogsReauthorizationGuidance(t *testing.T) {
	var buf bytes.Buffer
	var getCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/gmail/v1/users/me/messages":
			if _, err := w.Write([]byte(`{"messages":[{"id":"msg-1"}]}`)); err != nil {
				t.Fatalf("write response: %v", err)
			}
		case "/gmail/v1/users/me/messages/msg-1":
			getCalls++
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte(`{"error":{"code":401,"message":"invalid_grant"}}`)); err != nil {
				t.Fatalf("write response: %v", err)
			}
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	svc, err := gmail.NewService(
		context.Background(),
		option.WithHTTPClient(server.Client()),
		option.WithEndpoint(server.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	reader := &Reader{
		client:       svc,
		lookbackDays: 14,
		logger:       slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	err = reader.processRule(context.Background(), api.Rule{Name: "Test Rule", Source: api.Source{Label: "test"}}, make(chan *api.TransactionDetails, 1))
	if err == nil {
		t.Fatal("expected processRule to return get-message auth error")
	}
	if getCalls != 2 {
		t.Fatalf("get calls = %d, want 2", getCalls)
	}
	out := buf.String()
	if !strings.Contains(out, "level=ERROR") {
		t.Fatalf("expected ERROR log level, got %s", out)
	}
	if !strings.Contains(out, "OAuth token invalid") {
		t.Fatalf("expected OAuth invalid guidance, got %s", out)
	}
}

func TestProcessRule_QueriesEveryExactSender(t *testing.T) {
	queries := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/gmail/v1/users/me/messages" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		queries = append(queries, r.URL.Query().Get("q"))
		if _, err := w.Write([]byte(`{"messages":[]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	svc, err := gmail.NewService(
		context.Background(),
		option.WithHTTPClient(server.Client()),
		option.WithEndpoint(server.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	reader := &Reader{
		client:       svc,
		lookbackDays: 14,
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	rule := api.Rule{
		Name:            "HDFC Credit Card",
		SenderEmails:    []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"},
		SubjectContains: "HDFC Credit Card",
		Source:          api.Source{Type: "Credit Card", Label: "HDFC Credit Card", Bank: "HDFC"},
	}

	if err := reader.processRule(context.Background(), rule, make(chan *api.TransactionDetails, 1)); err != nil {
		t.Fatalf("processRule: %v", err)
	}

	if len(queries) != 2 {
		t.Fatalf("queries = %#v, want 2 sender-specific queries", queries)
	}
	wantFragments := [][]string{
		{"from:alerts@hdfcbank.net", `subject:"HDFC Credit Card"`},
		{"from:alerts@hdfcbank.bank.in", `subject:"HDFC Credit Card"`},
	}
	for i, fragments := range wantFragments {
		for _, fragment := range fragments {
			if !strings.Contains(queries[i], fragment) {
				t.Fatalf("query[%d] = %q, missing %q", i, queries[i], fragment)
			}
		}
	}
}

func TestBuildGmailQueriesForRule(t *testing.T) {
	rule := api.Rule{
		SenderEmails:    []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"},
		SubjectContains: "HDFC Bank Credit Card",
	}

	got := buildGmailQueriesForRule(rule)
	want := []string{
		`from:alerts@hdfcbank.net subject:"HDFC Bank Credit Card"`,
		`from:alerts@hdfcbank.bank.in subject:"HDFC Bank Credit Card"`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("queries = %#v, want %#v", got, want)
	}
}

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}

func TestRead_CheckpointAfterListMessagesResult(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		body           string
		wantCheckpoint bool
	}{
		{
			name:           "successful empty list saves checkpoint",
			status:         http.StatusOK,
			body:           `{"messages":[]}`,
			wantCheckpoint: true,
		},
		{
			name:           "failed list leaves checkpoint untouched",
			status:         http.StatusInternalServerError,
			body:           `{"error":{"message":"temporary failure"}}`,
			wantCheckpoint: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var listCalls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/gmail/v1/users/me/messages" {
					t.Fatalf("unexpected path %q", r.URL.Path)
				}
				listCalls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if _, err := w.Write([]byte(tc.body)); err != nil {
					t.Fatalf("write response: %v", err)
				}
			}))
			t.Cleanup(server.Close)

			svc, err := gmail.NewService(
				context.Background(),
				option.WithHTTPClient(server.Client()),
				option.WithEndpoint(server.URL+"/"),
			)
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			checkpointSaved := make(chan struct{}, 1)
			reader := &Reader{
				client:       svc,
				rules:        []api.Rule{{Name: "Test Rule", Source: api.Source{Label: "test"}}},
				interval:     time.Hour,
				lookbackDays: 14,
				onCheckpoint: func(time.Time) {
					checkpointSaved <- struct{}{}
					cancel()
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			err = reader.Read(ctx, make(chan *api.TransactionDetails), nil)
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Read error = %v, want context cancellation", err)
			}
			if listCalls != 1 {
				t.Fatalf("list calls = %d, want 1", listCalls)
			}

			select {
			case <-checkpointSaved:
				if !tc.wantCheckpoint {
					t.Fatal("checkpoint was saved after failed list response")
				}
			default:
				if tc.wantCheckpoint {
					t.Fatal("checkpoint was not saved after successful list response")
				}
			}
		})
	}
}

func TestExtractBody(t *testing.T) {
	tests := []struct {
		name string
		msg  *gmail.Message
		want string
	}{
		{
			name: "html part returned",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: b64("<p>Hello</p>")}},
					},
				},
			},
			want: "<p>Hello</p>",
		},
		{
			name: "only body data no parts",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Body: &gmail.MessagePartBody{Data: b64("plain text body")},
				},
			},
			want: "plain text body",
		},
		{
			name: "invalid base64 in html part falls through to body",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: "!!!not-valid-base64!!!"}},
					},
					Body: &gmail.MessagePartBody{Data: b64("fallback body")},
				},
			},
			want: "fallback body",
		},
		{
			name: "invalid base64 in body returns empty",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Body: &gmail.MessagePartBody{Data: "!!!not-valid-base64!!!"},
				},
			},
			want: "",
		},
		{
			name: "no parts and no body data returns empty",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{},
			},
			want: "",
		},
		{
			name: "nil body returns empty",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Body: nil,
				},
			},
			want: "",
		},
		{
			name: "text/plain part skipped, falls through to body",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: b64("plain text")}},
					},
					Body: &gmail.MessagePartBody{Data: b64("body fallback")},
				},
			},
			want: "body fallback",
		},
		{
			name: "multiple parts picks html",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: b64("plain")}},
						{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: b64("<b>rich</b>")}},
					},
				},
			},
			want: "<b>rich</b>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractBody(tc.msg)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
