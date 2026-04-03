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
	client       *gmail.Service
	rules        []api.Rule
	labels       api.Labels
	interval     time.Duration
	lookbackDays int
	state        *state.Manager
	logger       *slog.Logger
}

// Config holds configuration for the Gmail reader.
type Config struct {
	// Rules defines the email matching rules for transaction extraction.
	Rules []api.Rule
	// Labels maps merchants to categories.
	Labels api.Labels
	// Interval between rule evaluations. Defaults to 60 seconds.
	Interval time.Duration
	// LookbackDays limits how far back in time to search for emails.
	// Defaults to 180 days (6 months). Set to 0 to use the default.
	LookbackDays int
	// State is the state manager for tracking processed messages.
	State *state.Manager
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
		client:       client,
		rules:        cfg.Rules,
		labels:       cfg.Labels,
		interval:     interval,
		lookbackDays: lookback,
		state:        cfg.State,
		logger:       logger,
	}, nil
}

// Read continuously evaluates rules and sends extracted transactions to the output channel.
// It runs until the context is canceled.
// Messages are marked as processed only after receiving acknowledgment via ackChan.
func (r *Reader) Read(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
	defer close(out)

	// Start goroutine to handle acknowledgments (mark as read + update state)
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

// handleAcknowledgments marks emails as read and updates state when they're successfully written.
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
			// Mark as read in Gmail
			r.markAsRead(ctx, msgID)
			// Mark as processed in state
			if r.state != nil {
				if err := r.state.MarkProcessed(msgID); err != nil {
					r.logger.Warn("failed to mark message as processed in state", "message_id", msgID, "error", err)
				}
			}
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

	// Build Gmail query: rule fields + lookback date filter.
	query := rule.BuildGmailQuery()
	since := time.Now().AddDate(0, 0, -r.lookbackDays)
	if query != "" {
		query += " "
	}
	query += fmt.Sprintf("after:%s", since.Format("2006/01/02"))
	logger.Debug("executing gmail query", "query", query, "lookback_days", r.lookbackDays)

	// Paginate through all matching messages.
	var pageToken string
	totalFound := 0
	for {
		select {
		case <-ctx.Done():
			logger.Info("context cancelled, stopping rule processing", "rule", rule.Name)
			return
		default:
		}

		req := r.client.Users.Messages.List("me").Q(query).Context(ctx)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		resp, err := req.Do()
		if err != nil {
			logAPIError(logger, "failed to list messages", err)
			return
		}

		for _, msg := range resp.Messages {
			if r.state != nil && r.state.IsProcessed(msg.Id) {
				logger.Debug("skipping already processed message", "message_id", msg.Id)
				continue
			}
			if err := r.processMessage(ctx, msg.Id, rule, out); err != nil {
				logger.Error("failed to process message", "message_id", msg.Id, "error", err)
				continue
			}
			totalFound++
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	logger.Info("rule processing complete", "rule", rule.Name, "messages_processed", totalFound)
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
	transaction := extractor.ExtractTransactionDetails(body, rule.Amount, rule.MerchantInfo, rule.Currency, receivedTime)
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
	// Message will be marked as processed only after successful write and acknowledgment
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- transaction:
	}

	return nil
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
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) && apiErr.Code == http.StatusUnauthorized {
		logger.Error(msg+" (unauthorized — re-run onboarding to refresh credentials)", "error", err)
		return
	}
	// oauth2 token errors (expired with no refresh token)
	if strings.Contains(err.Error(), "token expired") || strings.Contains(err.Error(), "refresh token") {
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
