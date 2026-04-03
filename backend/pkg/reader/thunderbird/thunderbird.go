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
	"time"

	"github.com/emersion/go-mbox"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/extractor"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

// Reader reads transactions from Thunderbird MBOX mailboxes.
type Reader struct {
	mailboxPaths map[string]string // mailbox name -> file path
	rules        []api.Rule
	labels       api.Labels
	state        *state.Manager
	interval     time.Duration
	logger       *slog.Logger
}

// Config holds configuration for the Thunderbird reader.
type Config struct {
	// ProfilePath is the Thunderbird profile directory path.
	ProfilePath string
	// Mailboxes is a list of mailbox names to scan (e.g., ["Inbox", "Archive"]).
	Mailboxes []string
	// Rules defines the email matching rules for transaction extraction.
	Rules []api.Rule
	// Labels maps merchants to categories.
	Labels api.Labels
	// State is the state manager for tracking processed messages.
	State *state.Manager
	// Interval between mailbox scans. Defaults to 60 seconds.
	Interval time.Duration
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
		mailboxPaths: mailboxPaths,
		rules:        cfg.Rules,
		labels:       cfg.Labels,
		state:        cfg.State,
		interval:     interval,
		logger:       logger,
	}, nil
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

	// Run immediately on start
	if err := r.scanAllMailboxes(ctx, out); err != nil {
		r.logger.Error("failed to scan mailboxes", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("thunderbird reader stopping", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			if err := r.scanAllMailboxes(ctx, out); err != nil {
				r.logger.Error("failed to scan mailboxes", "error", err)
			}
		}
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
				if err := r.state.MarkProcessed(msgKey); err != nil {
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
	r.logger.Info("starting mailbox scan", "mailbox_count", len(r.mailboxPaths))

	for mailboxName, mailboxPath := range r.mailboxPaths {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
			return ctx.Err()
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

		// Generate message key using state package's GenerateKey function
		messageID := msg.Header.Get("Message-Id")
		dateStr := msg.Header.Get("Date")
		msgKey := state.GenerateKey(mailboxPath, messageID, dateStr)

		// Skip if already processed
		if r.state != nil && r.state.IsProcessed(msgKey) {
			skippedCount++
			continue
		}

		// Check if message matches any rule
		rule, matches := r.matchesRule(msg)
		if !matches {
			skippedCount++
			continue
		}

		// Extract transaction
		transaction, err := r.extractTransaction(msg, rule)
		if err != nil {
			logger.Warn("failed to extract transaction", "error", err, "message_key", msgKey)
			continue
		}

		// Add message key for acknowledgment
		transaction.MessageID = msgKey

		// Send transaction - will be marked as processed only after acknowledgment
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- transaction:
			processedCount++
			logger.Debug("extracted transaction",
				"amount", transaction.Amount,
				"merchant", transaction.MerchantInfo,
				"category", transaction.Category,
			)
		}
	}

	logger.Info("mailbox scan complete", "processed", processedCount, "skipped", skippedCount)
	return nil
}

// matchesRule checks if a message matches any enabled rule.
func (r *Reader) matchesRule(msg *mail.Message) (api.Rule, bool) {
	from := msg.Header.Get("From")
	subject := msg.Header.Get("Subject")

	// Decode RFC 2047 encoded headers
	from = decodeRFC2047(from)
	subject = decodeRFC2047(subject)

	for _, rule := range r.rules {
		if !rule.Enabled {
			continue
		}

		// Use the rule's MatchesEmail method
		if rule.MatchesEmail(from, subject) {
			return rule, true
		}
	}

	return api.Rule{}, false
}

// extractTransaction extracts transaction details from a message.
func (r *Reader) extractTransaction(msg *mail.Message, rule api.Rule) (*api.TransactionDetails, error) {
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
	transaction.Category, transaction.Bucket = r.labels.LabelLookup(transaction.MerchantInfo)
	transaction.Source = rule.Source

	return transaction, nil
}
