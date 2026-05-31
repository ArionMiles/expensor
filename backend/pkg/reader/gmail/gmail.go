// Package gmail implements a Reader that extracts transactions from Gmail.
package gmail

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	mailpkg "net/mail"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/extractor"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

// Reader reads transactions from Gmail messages.
type Reader struct {
	client              *gmail.Service
	rules               []api.Rule
	resolver            api.CategoryResolver
	interval            time.Duration
	lookbackDays        int
	lastScanAt          *time.Time      // checkpoint: only scan emails after this time on normal runs
	forceFullScan       bool            // when true, bypass checkpoint and use full lookback window
	onCheckpoint        func(time.Time) // called after each scan iteration to persist the checkpoint
	state               *state.Manager
	diagnosticSink      api.DiagnosticSink
	diagnosticSlots     chan struct{}
	diagnosticSlotsOnce sync.Once
	logger              *slog.Logger
}

// Config holds configuration for the Gmail reader.
type Config struct {
	// Rules defines the email matching rules for transaction extraction.
	Rules []api.Rule
	// Resolver maps merchant info to category and bucket.
	Resolver api.CategoryResolver
	// Interval between rule evaluations. Defaults to 60 seconds.
	Interval time.Duration
	// LookbackDays limits how far back in time to search for emails.
	// Defaults to 180 days (6 months). Set to 0 to use the default.
	LookbackDays int
	// LastScanAt is the timestamp of the last successful scan. When set
	// (and ForceFullScan is false) only emails received after this time
	// are fetched, avoiding a full lookback scan every interval.
	LastScanAt *time.Time
	// ForceFullScan bypasses LastScanAt and fetches the full lookback window.
	// Used for force-rescan and retroactive rule application.
	ForceFullScan bool
	// OnCheckpoint is called with time.Now() after each successful scan iteration.
	// Use it to persist the checkpoint so the next run starts from here.
	OnCheckpoint func(time.Time)
	// State is the state manager for tracking processed messages.
	State *state.Manager
	// DiagnosticSink records best-effort extraction diagnostics.
	DiagnosticSink api.DiagnosticSink
}

// New creates a new Gmail reader.
func New(httpClient *http.Client, cfg Config, logger *slog.Logger) (*Reader, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Use context.Background() for service initialization (one-time setup)
	// The actual API calls later use the context passed to Read()
	client, err := gmail.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("creating gmail service: %w", err)
	}

	interval := cfg.Interval
	if interval == 0 {
		interval = 60 * time.Second
	}

	lookback := cfg.LookbackDays
	if lookback <= 0 {
		lookback = 180
	}

	return &Reader{
		client:          client,
		rules:           cfg.Rules,
		resolver:        cfg.Resolver,
		interval:        interval,
		lookbackDays:    lookback,
		lastScanAt:      cfg.LastScanAt,
		forceFullScan:   cfg.ForceFullScan,
		onCheckpoint:    cfg.OnCheckpoint,
		state:           cfg.State,
		diagnosticSink:  cfg.DiagnosticSink,
		diagnosticSlots: make(chan struct{}, maxConcurrentDiagnostics),
		logger:          logger,
	}, nil
}

// Read continuously evaluates rules and sends extracted transactions to the output channel.
// It runs until the context is canceled.
// Messages are marked as processed only after receiving acknowledgment via ackChan.
func (r *Reader) Read(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
	defer close(out)

	// Start goroutine to update state when transactions are successfully written.
	go r.handleAcknowledgments(ctx, ackChan)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run immediately on start, then checkpoint.
	iterationErr := r.evaluateRules(ctx, out)
	if ctx.Err() != nil {
		iterationErr = errors.Join(iterationErr, ctx.Err())
	}
	r.saveCheckpointAfterIteration(iterationErr)

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("gmail reader stopping", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			iterationErr := r.evaluateRules(ctx, out)
			if ctx.Err() != nil {
				iterationErr = errors.Join(iterationErr, ctx.Err())
			}
			r.saveCheckpointAfterIteration(iterationErr)
		}
	}
}

// effectiveSince returns the date to use in the Gmail `after:` filter.
// Uses the checkpoint when available; falls back to the configured lookback window.
func (r *Reader) effectiveSince() time.Time {
	if r.lastScanAt != nil && !r.forceFullScan {
		return r.lastScanAt.Add(-time.Hour) // 1-hour buffer for clock skew
	}
	return time.Now().AddDate(0, 0, -r.lookbackDays)
}

// saveCheckpoint records the current time as the last successful scan timestamp.
// After a checkpoint is saved, subsequent normal scans only fetch emails from this point.
func (r *Reader) saveCheckpoint() {
	now := time.Now()
	r.lastScanAt = &now
	r.forceFullScan = false // clear force flag after the full scan completes
	if r.onCheckpoint != nil {
		r.onCheckpoint(now)
	}
}

func (r *Reader) saveCheckpointAfterIteration(iterationErr error) {
	if iterationErr != nil {
		logger := r.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("scan checkpoint not saved after incomplete scan", "error", iterationErr)
		return
	}
	r.saveCheckpoint()
}

// handleAcknowledgments updates state when transactions are successfully written.
func (r *Reader) handleAcknowledgments(ctx context.Context, ackChan <-chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case msgID, ok := <-ackChan:
			if !ok {
				r.logger.Info("acknowledgment channel closed")
				return
			}
			if r.state != nil {
				if err := r.state.MarkProcessed(ctx, msgID); err != nil {
					r.logger.Warn("failed to mark message as processed in state", "message_id", msgID, "error", err)
				}
			}
		}
	}
}

const (
	maxConcurrentRules       = 5
	maxConcurrentDiagnostics = 8
	diagnosticRecordTimeout  = 2 * time.Second
)

func (r *Reader) evaluateRules(ctx context.Context, out chan<- *api.TransactionDetails) error {
	r.logger.Info("starting rule evaluation", "rule_count", len(r.rules))

	sem := make(chan struct{}, maxConcurrentRules)
	errCh := make(chan error, len(r.rules))
	var wg sync.WaitGroup
	for _, rule := range r.rules {
		wg.Add(1)
		go func(rule api.Rule) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()
			if err := r.processRule(ctx, rule, out); err != nil {
				errCh <- err
			}
		}(rule)
	}
	wg.Wait()
	close(errCh)

	r.logger.Info("rule evaluation complete")

	var ruleErrs []error
	for err := range errCh {
		ruleErrs = append(ruleErrs, err)
	}
	return errors.Join(ruleErrs...)
}

func (r *Reader) processRule(ctx context.Context, rule api.Rule, out chan<- *api.TransactionDetails) error {
	logger := r.logger.With("rule", rule.Name, "source", rule.Source)
	queries := r.buildRuleQueries(rule, logger)

	// Paginate through all matching messages.
	totalFound := 0
	var messageErrs []error
	for _, query := range queries {
		var pageToken string
		for {
			resp, err := r.listMessagesPage(ctx, query, pageToken)
			if err != nil {
				return handleListMessagesError(ctx, logger, rule.Name, err)
			}
			pageProcessed, pageErrs, err := r.processRuleMessages(ctx, rule, out, logger, resp.Messages)
			totalFound += pageProcessed
			messageErrs = append(messageErrs, pageErrs...)

			if err != nil {
				return err
			}
			pageToken = resp.NextPageToken
			if pageToken == "" {
				break
			}
		}
	}

	logger.Info("rule processing complete", "rule", rule.Name, "messages_processed", totalFound)
	return errors.Join(messageErrs...)
}

func (r *Reader) buildRuleQuery(rule api.Rule, logger *slog.Logger) string {
	queries := r.buildRuleQueries(rule, logger)
	if len(queries) == 0 {
		return ""
	}
	return queries[0]
}

func (r *Reader) buildRuleQueries(rule api.Rule, logger *slog.Logger) []string {
	queries := buildGmailQueriesForRule(rule)
	since := r.effectiveSince()
	logger.Debug("executing gmail query", "since", since.Format("2006/01/02"),
		"checkpoint", r.lastScanAt != nil && !r.forceFullScan)
	out := make([]string, 0, len(queries))
	for _, query := range queries {
		if query != "" {
			query += " "
		}
		out = append(out, query+fmt.Sprintf("after:%s", since.Format("2006/01/02")))
	}
	return out
}

func buildGmailQueriesForRule(rule api.Rule) []string {
	senders := normalizedRuleSenders(rule)
	if len(senders) == 0 {
		return []string{buildGmailQueryForSender(rule, "")}
	}
	queries := make([]string, 0, len(senders))
	for _, sender := range senders {
		queries = append(queries, buildGmailQueryForSender(rule, sender))
	}
	return queries
}

func buildGmailQueryForSender(rule api.Rule, sender string) string {
	var parts []string
	if sender != "" {
		parts = append(parts, fmt.Sprintf("from:%s", sender))
	}
	if rule.SubjectContains != "" {
		parts = append(parts, fmt.Sprintf("subject:%q", rule.SubjectContains))
	}
	return strings.Join(parts, " ")
}

func normalizedRuleSenders(rule api.Rule) []string {
	raw := rule.SenderEmails
	if len(raw) == 0 && rule.SenderEmail != "" {
		raw = []string{rule.SenderEmail}
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, sender := range raw {
		normalized := strings.ToLower(strings.TrimSpace(senderEmail(sender)))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func (r *Reader) listMessagesPage(ctx context.Context, query, pageToken string) (*gmail.ListMessagesResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var resp *gmail.ListMessagesResponse
	err := doWithAuthRetry(func() error {
		req := r.client.Users.Messages.List("me").Q(query).Context(ctx)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}
		var callErr error
		resp, callErr = req.Do()
		return callErr
	})
	return resp, err
}

func handleListMessagesError(ctx context.Context, logger *slog.Logger, ruleName string, err error) error {
	if ctx.Err() != nil {
		logger.Info("context canceled, stopping rule processing", "rule", ruleName)
		return ctx.Err()
	}
	logAPIError(logger, "failed to list messages", err)
	return fmt.Errorf("listing messages for rule %q: %w", ruleName, err)
}

func (r *Reader) processRuleMessages(
	ctx context.Context,
	rule api.Rule,
	out chan<- *api.TransactionDetails,
	logger *slog.Logger,
	messages []*gmail.Message,
) (int, []error, error) {
	processed := 0
	var messageErrs []error
	for _, msg := range messages {
		if r.shouldSkipMessage(ctx, msg.Id, logger) {
			continue
		}
		if err := r.processMessage(ctx, msg.Id, rule, out); err != nil {
			if ctx.Err() != nil {
				logger.Info("context canceled, stopping rule processing", "rule", rule.Name)
				return processed, messageErrs, ctx.Err()
			}
			logAPIError(logger, "failed to process message", err)
			messageErrs = append(messageErrs, err)
			continue
		}
		processed++
	}
	return processed, messageErrs, nil
}

func (r *Reader) shouldSkipMessage(ctx context.Context, msgID string, logger *slog.Logger) bool {
	if r.state == nil || !r.state.IsProcessed(ctx, msgID) {
		return false
	}
	logger.Debug("skipping already processed message", "message_id", msgID)
	return true
}

func (r *Reader) processMessage(ctx context.Context, msgID string, rule api.Rule, out chan<- *api.TransactionDetails) error {
	var msg *gmail.Message
	err := doWithAuthRetry(func() error {
		var callErr error
		msg, callErr = r.client.Users.Messages.Get("me", msgID).Context(ctx).Do()
		return callErr
	})
	if err != nil {
		return fmt.Errorf("getting message: %w", err)
	}

	headers := gmailMessageHeaders(msg)
	subject := headers["subject"]

	// Extract body
	body := extractBody(msg)
	if body == "" {
		r.logger.Warn("empty message body", "message_id", msgID, "subject", subject)
		return nil
	}

	receivedTime := time.Unix(msg.InternalDate/1000, 0)
	transaction := extractor.ExtractTransactionDetails(body, rule.Amount, rule.MerchantInfo, rule.Currency, receivedTime)
	if r.resolver != nil {
		transaction.Category, transaction.Bucket = r.resolver(transaction.MerchantInfo)
	}
	transaction.Source = rule.Source
	transaction.MessageID = msgID // Store message ID for later acknowledgment
	r.recordExtractionDiagnostic(ctx, gmailExtractionDiagnostic(gmailDiagnosticContext{
		message:      msg,
		messageID:    msgID,
		rule:         rule,
		body:         body,
		transaction:  transaction,
		headers:      headers,
		receivedTime: receivedTime,
	}))

	r.logger.Debug("extracted transaction",
		"subject", subject,
		"amount", transaction.Amount,
		"merchant", transaction.MerchantInfo,
		"category", transaction.Category,
		"message_id", msgID,
	)

	// Send transaction to output channel
	// Message will be marked as processed only after successful write and acknowledgment
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- transaction:
	}

	return nil
}

func (r *Reader) recordExtractionDiagnostic(ctx context.Context, diagnostic api.ExtractionDiagnostic) {
	if r.diagnosticSink == nil || len(diagnostic.FailureReasons) == 0 {
		return
	}
	release, ok := r.acquireDiagnosticSlot()
	if !ok {
		r.diagnosticLogger().Warn("skipping extraction diagnostic; diagnostic recorder is saturated",
			"reader", diagnostic.Reader, "message_id", diagnostic.MessageID)
		return
	}
	sink := r.diagnosticSink
	logger := r.logger
	go func() {
		defer release()
		diagnosticCtx, cancel := context.WithTimeout(ctx, diagnosticRecordTimeout)
		defer cancel()
		if err := sink.RecordExtractionDiagnostic(diagnosticCtx, diagnostic); err != nil {
			if logger == nil {
				logger = slog.Default()
			}
			logger.Warn("failed to record extraction diagnostic", "reader", diagnostic.Reader, "message_id", diagnostic.MessageID, "error", err)
		}
	}()
}

func (r *Reader) acquireDiagnosticSlot() (func(), bool) {
	r.diagnosticSlotsOnce.Do(func() {
		if r.diagnosticSlots == nil {
			r.diagnosticSlots = make(chan struct{}, maxConcurrentDiagnostics)
		}
	})
	select {
	case r.diagnosticSlots <- struct{}{}:
		return func() { <-r.diagnosticSlots }, true
	default:
		return nil, false
	}
}

func (r *Reader) diagnosticLogger() *slog.Logger {
	if r.logger == nil {
		return slog.Default()
	}
	return r.logger
}

type gmailDiagnosticContext struct {
	message      *gmail.Message
	messageID    string
	rule         api.Rule
	body         string
	transaction  *api.TransactionDetails
	headers      map[string]string
	receivedTime time.Time
}

func gmailExtractionDiagnostic(ctx gmailDiagnosticContext) api.ExtractionDiagnostic {
	snapshot := ctx.rule.DiagnosticSnapshot()
	sender := ctx.headers["from"]
	return api.ExtractionDiagnostic{
		Reader:         "gmail",
		MessageID:      ctx.messageID,
		Source:         ctx.rule.Source.Display(),
		Sender:         sender,
		SenderEmail:    senderEmail(sender),
		Subject:        ctx.headers["subject"],
		EmailBody:      ctx.body,
		ReceivedAt:     &ctx.receivedTime,
		Snippet:        ctx.message.Snippet,
		RuleID:         snapshot.RuleID,
		RuleName:       snapshot.RuleName,
		AmountRegex:    snapshot.AmountRegex,
		MerchantRegex:  snapshot.MerchantRegex,
		CurrencyRegex:  snapshot.CurrencyRegex,
		FailureReasons: api.ExtractionFailureReasons(ctx.transaction),
	}
}

func gmailMessageHeaders(msg *gmail.Message) map[string]string {
	headers := make(map[string]string)
	if msg.Payload == nil {
		return headers
	}
	for _, header := range msg.Payload.Headers {
		headers[strings.ToLower(header.Name)] = header.Value
	}
	return headers
}

func senderEmail(sender string) string {
	address, err := mailpkg.ParseAddress(sender)
	if err != nil {
		return ""
	}
	return address.Address
}

func doWithAuthRetry(call func() error) error {
	err := call()
	if err == nil || !isAuthError(err) {
		return err
	}
	return call()
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) && apiErr.Code == http.StatusUnauthorized {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "invalid_grant") ||
		strings.Contains(s, "Token has been expired or revoked") ||
		strings.Contains(s, "token expired") ||
		strings.Contains(s, "refresh token")
}

// logAPIError logs a Gmail API error at the appropriate level.
// Transient network failures (no connectivity, timeouts) are logged at Warn
// because they resolve on the next poll. Auth errors and unexpected API
// failures are logged at Error because they require user action.
func logAPIError(logger *slog.Logger, msg string, err error) {
	if isNetworkError(err) {
		logger.Warn(msg+" (network unavailable, will retry on next poll)", "error", err)
		return
	}
	if isAuthError(err) {
		logger.Error(msg+" (OAuth token invalid — re-run onboarding to re-authorize)", "error", err)
		return
	}
	logger.Error(msg, "error", err)
}

// isNetworkError returns true for transient connectivity failures.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	// Covers "no such host", "connection refused", "no route to host", etc.
	s := err.Error()
	return strings.Contains(s, "no such host") ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no route to host") ||
		strings.Contains(s, "network is unreachable") ||
		strings.Contains(s, "dial tcp")
}

func extractBody(msg *gmail.Message) string {
	// Try to find HTML body in parts
	for _, part := range msg.Payload.Parts {
		if part.MimeType == "text/html" {
			bodyBytes, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				continue
			}
			return string(bodyBytes)
		}
	}

	// Fallback to direct body data
	if msg.Payload.Body != nil && msg.Payload.Body.Data != "" {
		bodyBytes, err := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		if err == nil {
			return string(bodyBytes)
		}
	}

	return ""
}
