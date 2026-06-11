// Package thunderbird implements a Reader that extracts transactions from Thunderbird mailboxes.
package thunderbird

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/mail"
	"os"
	"sync"
	"time"

	"github.com/emersion/go-mbox"
	"go.opentelemetry.io/otel/attribute"

	"github.com/ArionMiles/expensor/backend/internal/extractor"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/observability"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

// Reader reads transactions from Thunderbird MBOX mailboxes.
type Reader struct {
	mailboxPaths        map[string]string // mailbox name -> file path
	rules               []api.Rule
	resolver            api.CategoryResolver
	state               *state.Manager
	interval            time.Duration
	lastScanAt          *time.Time // checkpoint: skip emails older than this on normal scans
	forceFullScan       bool       // bypass checkpoint and scan all emails in the mailbox
	onCheckpoint        func(time.Time)
	diagnosticSink      api.DiagnosticSink
	diagnosticSlots     chan struct{}
	diagnosticSlotsOnce sync.Once
	logger              *slog.Logger
	scope               *observability.Scope
}

const (
	maxConcurrentDiagnostics = 8
	diagnosticRecordTimeout  = 2 * time.Second
)

// Config holds configuration for the Thunderbird reader.
type Config struct {
	// ProfilePath is the Thunderbird profile directory path.
	ProfilePath string
	// Mailboxes is a list of mailbox names to scan (e.g., ["Inbox", "Archive"]).
	Mailboxes []string
	// Rules defines the email matching rules for transaction extraction.
	Rules []api.Rule
	// Resolver maps merchant info to category and bucket.
	Resolver api.CategoryResolver
	// State is the state manager for tracking processed messages.
	State *state.Manager
	// Interval between mailbox scans. Defaults to 60 seconds.
	Interval time.Duration
	// LastScanAt is the timestamp of the last successful scan.
	LastScanAt *time.Time
	// ForceFullScan bypasses LastScanAt and processes all messages.
	ForceFullScan bool
	// OnCheckpoint is called with time.Now() after each successful scan iteration.
	OnCheckpoint func(time.Time)
	// DiagnosticSink records best-effort extraction diagnostics.
	DiagnosticSink api.DiagnosticSink
	// ObservabilityScope records reader telemetry. Defaults to a Thunderbird reader scope.
	ObservabilityScope *observability.Scope
}

// New creates a new Thunderbird reader.
func New(cfg Config, logger *slog.Logger) (*Reader, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Find mailbox paths
	mailboxPaths, err := FindMailboxes(cfg.ProfilePath, cfg.Mailboxes)
	if err != nil {
		return nil, fmt.Errorf("finding mailboxes: %w", err)
	}

	logger.Info("found mailboxes", "count", len(mailboxPaths), "paths", mailboxPaths)

	interval := cfg.Interval
	if interval == 0 {
		interval = 60 * time.Second
	}

	return &Reader{
		mailboxPaths:    mailboxPaths,
		rules:           cfg.Rules,
		resolver:        cfg.Resolver,
		state:           cfg.State,
		interval:        interval,
		lastScanAt:      cfg.LastScanAt,
		forceFullScan:   cfg.ForceFullScan,
		onCheckpoint:    cfg.OnCheckpoint,
		diagnosticSink:  cfg.DiagnosticSink,
		diagnosticSlots: make(chan struct{}, maxConcurrentDiagnostics),
		logger:          logger,
		scope:           readerScope(cfg.ObservabilityScope, logger),
	}, nil
}

func readerScope(scope *observability.Scope, logger *slog.Logger) *observability.Scope {
	if scope != nil {
		return scope
	}
	return observability.NewScope(logger, "github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird")
}

func (r *Reader) observabilityScope() *observability.Scope {
	if r.scope == nil {
		r.scope = readerScope(nil, r.logger)
	}
	return r.scope
}

// Read continuously scans mailboxes and sends extracted transactions to the output channel.
// It runs until the context is canceled.
// Messages are marked as processed only after receiving acknowledgment via ackChan.
func (r *Reader) Read(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
	defer close(out)

	// Start goroutine to handle acknowledgments
	go r.handleAcknowledgments(ctx, ackChan)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run immediately on start.
	if err := r.scanAllMailboxes(ctx, out); err != nil {
		r.logger.Error("failed to scan mailboxes", "error", err)
	} else if ctx.Err() == nil {
		// Only checkpoint when the context is still live. A canceled context
		// means this run was interrupted; writing a timestamp here would
		// overwrite whatever the caller set in the DB before the restart.
		r.saveCheckpoint()
	}

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("thunderbird reader stopping", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			if err := r.scanAllMailboxes(ctx, out); err != nil {
				r.logger.Error("failed to scan mailboxes", "error", err)
			} else if ctx.Err() == nil {
				r.saveCheckpoint()
			}
		}
	}
}

// isBeforeCheckpoint returns true if the email with the given Date header should
// be skipped because it predates the current scan checkpoint.
func (r *Reader) isBeforeCheckpoint(dateStr string) bool {
	if r.lastScanAt == nil || r.forceFullScan || dateStr == "" {
		return false
	}
	t, err := mail.ParseDate(dateStr)
	if err != nil {
		return false
	}
	return t.Before(r.lastScanAt.Add(-time.Hour))
}

// saveCheckpoint records the current time and clears the force-full-scan flag.
func (r *Reader) saveCheckpoint() {
	now := time.Now()
	r.lastScanAt = &now
	r.forceFullScan = false
	if r.onCheckpoint != nil {
		r.onCheckpoint(now)
	}
}

// handleAcknowledgments marks messages as processed when they're successfully written.
func (r *Reader) handleAcknowledgments(ctx context.Context, ackChan <-chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case msgKey, ok := <-ackChan:
			if !ok {
				r.logger.Info("acknowledgment channel closed")
				return
			}
			// Mark as processed in state
			if r.state != nil {
				if err := r.state.MarkProcessed(ctx, msgKey); err != nil {
					r.logger.Warn("failed to mark message as processed", "message_key", msgKey, "error", err)
				} else {
					r.logger.Debug("marked message as processed", "message_key", msgKey)
				}
			}
		}
	}
}

// scanAllMailboxes scans all configured mailboxes for new transactions.
func (r *Reader) scanAllMailboxes(ctx context.Context, out chan<- *api.TransactionDetails) error {
	ctx, span := r.observabilityScope().Start(ctx, "thunderbird.scan")
	defer span.End()

	r.logger.Info("starting mailbox scan", "mailbox_count", len(r.mailboxPaths))
	var scanErr error
	defer func() {
		r.observabilityScope().RecordOperation(ctx, observability.Operation{Namespace: "thunderbird", Name: "scan", Err: scanErr})
	}()

	for mailboxName, mailboxPath := range r.mailboxPaths {
		select {
		case <-ctx.Done():
			scanErr = ctx.Err()
			return scanErr
		default:
		}

		if err := r.scanMailbox(ctx, mailboxName, mailboxPath, out); err != nil {
			r.logger.Error("failed to scan mailbox", "mailbox", mailboxName, "error", err)
			continue
		}
	}

	r.logger.Info("mailbox scan complete")
	return nil
}

// scanMailbox scans a single mailbox for transactions.
func (r *Reader) scanMailbox(ctx context.Context, mailboxName, mailboxPath string, out chan<- *api.TransactionDetails) error {
	ctx, span := r.observabilityScope().Start(ctx, "thunderbird.mailbox.scan")
	defer span.End()

	logger := r.logger.With("mailbox", mailboxName, "path", mailboxPath)

	file, err := os.Open(mailboxPath)
	if err != nil {
		return fmt.Errorf("opening mailbox: %w", err)
	}
	defer file.Close()

	mboxReader := mbox.NewReader(file)
	processedCount := 0
	skippedCount := 0

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			r.observabilityScope().RecordOperation(ctx, observability.Operation{Namespace: "thunderbird", Name: "mailbox.scan", Err: err})
			return err
		default:
		}

		msgReader, err := mboxReader.NextMessage()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.Warn("error reading next message", "error", err)
			continue
		}

		msg, err := mail.ReadMessage(msgReader)
		if err != nil {
			logger.Warn("error parsing message", "error", err)
			continue
		}

		handled, err := r.processMessage(ctx, msg, mailboxPath, out)
		if err != nil {
			return err
		}
		if handled {
			processedCount++
		} else {
			skippedCount++
		}
	}

	logger.Info("mailbox scan complete", "processed", processedCount, "skipped", skippedCount)
	span.SetAttributes(
		attribute.Int("thunderbird.messages_processed", processedCount),
		attribute.Int("thunderbird.messages_skipped", skippedCount),
	)
	r.observabilityScope().RecordOperation(ctx, observability.Operation{Namespace: "thunderbird", Name: "mailbox.scan"})
	return nil
}

// processMessage handles a single MBOX message: dedup, checkpoint, rule match, extract, send.
// Returns (true, nil) when the message was sent to out; (false, nil) when skipped.
func (r *Reader) processMessage(
	ctx context.Context,
	msg *mail.Message,
	mailboxPath string,
	out chan<- *api.TransactionDetails,
) (bool, error) {
	ctx, span := r.observabilityScope().Start(ctx, "thunderbird.messages.process")
	defer span.End()

	messageID := msg.Header.Get("Message-Id")
	dateStr := msg.Header.Get("Date")
	msgKey := state.GenerateKey(mailboxPath, messageID, dateStr)

	if r.state != nil && r.state.IsProcessed(ctx, msgKey) {
		r.observabilityScope().RecordOperation(ctx, observability.Operation{Namespace: "thunderbird", Name: "messages.skipped"})
		return false, nil
	}
	if r.isBeforeCheckpoint(dateStr) {
		r.observabilityScope().RecordOperation(ctx, observability.Operation{Namespace: "thunderbird", Name: "messages.skipped"})
		return false, nil
	}

	rule, matches := r.matchesRule(msg)
	if !matches {
		r.observabilityScope().RecordOperation(ctx, observability.Operation{Namespace: "thunderbird", Name: "messages.skipped"})
		return false, nil
	}

	transaction, err := r.extractTransaction(ctx, msg, rule, msgKey)
	if err != nil {
		r.observabilityScope().RecordOperation(ctx, observability.Operation{Namespace: "thunderbird", Name: "messages.process", Err: err})
		r.logger.Warn("failed to extract transaction", "error", err, "message_key", msgKey)
		return false, nil
	}
	transaction.MessageID = msgKey

	select {
	case <-ctx.Done():
		err := ctx.Err()
		r.observabilityScope().RecordOperation(ctx, observability.Operation{Namespace: "thunderbird", Name: "messages.process", Err: err})
		return false, err
	case out <- transaction:
		r.observabilityScope().RecordOperation(ctx, observability.Operation{Namespace: "thunderbird", Name: "messages.process"})
		r.logger.Debug("extracted transaction",
			"amount", transaction.Amount,
			"merchant", transaction.MerchantInfo,
			"category", transaction.Category,
		)
		return true, nil
	}
}

// matchesRule checks if a message matches any enabled rule.
func (r *Reader) matchesRule(msg *mail.Message) (api.Rule, bool) {
	from := msg.Header.Get("From")
	subject := msg.Header.Get("Subject")

	// Decode RFC 2047 encoded headers
	from = decodeRFC2047(from)
	subject = decodeRFC2047(subject)

	for _, rule := range r.rules {
		// Use the rule's MatchesEmail method
		if rule.MatchesEmail(from, subject) {
			return rule, true
		}
	}

	return api.Rule{}, false
}

// extractTransaction extracts transaction details from a message.
func (r *Reader) extractTransaction(ctx context.Context, msg *mail.Message, rule api.Rule, msgKey string) (*api.TransactionDetails, error) {
	// Extract body
	body, err := ExtractBody(msg)
	if err != nil {
		return nil, fmt.Errorf("extracting body: %w", err)
	}

	// Parse date
	dateStr := msg.Header.Get("Date")
	receivedTime, err := mail.ParseDate(dateStr)
	if err != nil {
		r.logger.Warn("failed to parse date, using current time", "date", dateStr, "error", err)
		receivedTime = time.Now()
	}

	// Use extractor package for consistent extraction
	transaction := extractor.ExtractTransactionDetails(body, rule.Amount, rule.MerchantInfo, rule.Currency, receivedTime)
	if r.resolver != nil {
		transaction.Category, transaction.Bucket = r.resolver(transaction.MerchantInfo)
	}
	transaction.Source = rule.Source
	r.recordExtractionDiagnostic(ctx, thunderbirdExtractionDiagnostic(thunderbirdDiagnosticContext{
		message:      msg,
		messageID:    msgKey,
		rule:         rule,
		body:         body,
		transaction:  transaction,
		receivedTime: receivedTime,
	}))

	return transaction, nil
}

func (r *Reader) recordExtractionDiagnostic(ctx context.Context, diagnostic api.ExtractionDiagnostic) {
	if r.diagnosticSink == nil || len(diagnostic.FailureReasons) == 0 {
		r.observabilityScope().RecordOperation(ctx, observability.Operation{Namespace: "diagnostics", Name: "skipped"})
		return
	}
	release, ok := r.acquireDiagnosticSlot()
	if !ok {
		r.observabilityScope().RecordOperation(ctx, observability.Operation{Namespace: "diagnostics", Name: "skipped"})
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
		diagnosticCtx, span := r.observabilityScope().Start(diagnosticCtx, "diagnostics.record")
		err := sink.RecordExtractionDiagnostic(diagnosticCtx, diagnostic)
		r.observabilityScope().RecordOperation(diagnosticCtx, observability.Operation{Namespace: "diagnostics", Name: "record", Err: err})
		span.End()
		if err != nil {
			if logger == nil {
				logger = slog.Default()
			}
			logger.Warn("failed to record extraction diagnostic", "reader", diagnostic.Reader, "error", err)
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

type thunderbirdDiagnosticContext struct {
	message      *mail.Message
	messageID    string
	rule         api.Rule
	body         string
	transaction  *api.TransactionDetails
	receivedTime time.Time
}

func thunderbirdExtractionDiagnostic(ctx thunderbirdDiagnosticContext) api.ExtractionDiagnostic {
	snapshot := ctx.rule.DiagnosticSnapshot()
	sender := decodeRFC2047(ctx.message.Header.Get("From"))
	return api.ExtractionDiagnostic{
		Reader:         "thunderbird",
		MessageID:      ctx.messageID,
		Source:         ctx.rule.Source.Display(),
		Sender:         sender,
		SenderEmail:    senderEmail(sender),
		Subject:        decodeRFC2047(ctx.message.Header.Get("Subject")),
		EmailBody:      ctx.body,
		ReceivedAt:     &ctx.receivedTime,
		RuleID:         snapshot.RuleID,
		RuleName:       snapshot.RuleName,
		AmountRegex:    snapshot.AmountRegex,
		MerchantRegex:  snapshot.MerchantRegex,
		CurrencyRegex:  snapshot.CurrencyRegex,
		FailureReasons: api.ExtractionFailureReasons(ctx.transaction),
	}
}

func senderEmail(sender string) string {
	address, err := mail.ParseAddress(sender)
	if err != nil {
		return ""
	}
	return address.Address
}
