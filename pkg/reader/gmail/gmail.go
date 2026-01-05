// Package gmail implements a Reader that extracts transactions from Gmail.
package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/ArionMiles/expensor/pkg/api"
)

// Reader reads transactions from Gmail messages.
type Reader struct {
	client   *gmail.Service
	rules    []api.Rule
	labels   api.Labels
	interval time.Duration
	logger   *slog.Logger
}

// Config holds configuration for the Gmail reader.
type Config struct {
	// Rules defines the email matching rules for transaction extraction.
	Rules []api.Rule
	// Labels maps merchants to categories.
	Labels api.Labels
	// Interval between rule evaluations. Defaults to 10 seconds.
	Interval time.Duration
}

// New creates a new Gmail reader.
func New(httpClient *http.Client, cfg Config, logger *slog.Logger) (*Reader, error) {
	if logger == nil {
		logger = slog.Default()
	}

	client, err := gmail.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("creating gmail service: %w", err)
	}

	interval := cfg.Interval
	if interval == 0 {
		interval = 10 * time.Second
	}

	return &Reader{
		client:   client,
		rules:    cfg.Rules,
		labels:   cfg.Labels,
		interval: interval,
		logger:   logger,
	}, nil
}

// Read continuously evaluates rules and sends extracted transactions to the output channel.
// It runs until the context is canceled.
// Emails are only marked as read after receiving acknowledgment via ackChan.
func (r *Reader) Read(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
	defer close(out)

	// Start goroutine to mark messages as read when acknowledged
	go r.handleAcknowledgments(ctx, ackChan)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run immediately on start
	r.evaluateRules(ctx, out)

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("gmail reader stopping", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			r.evaluateRules(ctx, out)
		}
	}
}

// handleAcknowledgments marks emails as read when they're successfully written.
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
			r.markAsRead(ctx, msgID)
		}
	}
}

// markAsRead marks a message as read in Gmail.
func (r *Reader) markAsRead(ctx context.Context, msgID string) {
	_, err := r.client.Users.Messages.Modify("me", msgID, &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{"UNREAD"},
	}).Context(ctx).Do()
	if err != nil {
		r.logger.Warn("failed to mark message as read", "message_id", msgID, "error", err)
	} else {
		r.logger.Debug("marked message as read", "message_id", msgID)
	}
}

func (r *Reader) evaluateRules(ctx context.Context, out chan<- *api.TransactionDetails) {
	r.logger.Info("starting rule evaluation", "rule_count", len(r.rules))

	var wg sync.WaitGroup
	for _, rule := range r.rules {
		if !rule.Enabled {
			r.logger.Debug("skipping disabled rule", "rule", rule.Name)
			continue
		}

		wg.Add(1)
		go func(rule api.Rule) {
			defer wg.Done()
			r.processRule(ctx, rule, out)
		}(rule)
	}
	wg.Wait()

	r.logger.Info("rule evaluation complete")
}

func (r *Reader) processRule(ctx context.Context, rule api.Rule, out chan<- *api.TransactionDetails) {
	logger := r.logger.With("rule", rule.Name, "source", rule.Source)

	resp, err := r.client.Users.Messages.List("me").Q(rule.Query).Context(ctx).Do()
	if err != nil {
		logger.Error("failed to list messages", "error", err)
		return
	}

	logger.Info("found messages", "count", len(resp.Messages))

	for _, msg := range resp.Messages {
		if err := r.processMessage(ctx, msg.Id, rule, out); err != nil {
			logger.Error("failed to process message", "message_id", msg.Id, "error", err)
			continue
		}
	}
}

func (r *Reader) processMessage(ctx context.Context, msgID string, rule api.Rule, out chan<- *api.TransactionDetails) error {
	msg, err := r.client.Users.Messages.Get("me", msgID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("getting message: %w", err)
	}

	// Extract subject for logging
	var subject string
	for _, header := range msg.Payload.Headers {
		if header.Name == "Subject" {
			subject = header.Value
			break
		}
	}

	// Extract body
	body := extractBody(msg)
	if body == "" {
		r.logger.Warn("empty message body", "message_id", msgID, "subject", subject)
		return nil
	}

	receivedTime := time.Unix(msg.InternalDate/1000, 0)
	transaction := ExtractTransactionDetails(body, rule.Amount, rule.MerchantInfo, receivedTime)
	transaction.Category, transaction.Bucket = r.labels.LabelLookup(transaction.MerchantInfo)
	transaction.Source = rule.Source
	transaction.MessageID = msgID // Store message ID for later acknowledgment

	r.logger.Debug("extracted transaction",
		"subject", subject,
		"amount", transaction.Amount,
		"merchant", transaction.MerchantInfo,
		"category", transaction.Category,
		"message_id", msgID,
	)

	// Send transaction to output channel
	// Email will be marked as read only after successful write and acknowledgment
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- transaction:
	}

	return nil
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

// ExtractTransactionDetails extracts transaction details from an email body using regex patterns.
func ExtractTransactionDetails(emailBody string, amountRegex, merchantRegex *regexp.Regexp, receivedTime time.Time) *api.TransactionDetails {
	transaction := &api.TransactionDetails{
		Timestamp: receivedTime.Format(time.DateTime),
	}

	if amountMatches := amountRegex.FindStringSubmatch(emailBody); len(amountMatches) > 1 {
		amountStr := strings.ReplaceAll(amountMatches[1], ",", "")
		if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
			transaction.Amount = amount
		}
	}

	if merchantMatches := merchantRegex.FindStringSubmatch(emailBody); len(merchantMatches) > 1 {
		transaction.MerchantInfo = merchantMatches[1]
	}

	return transaction
}
