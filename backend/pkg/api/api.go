// Package api defines the core interfaces and data structures for expensor.
package api

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// TransactionDetails holds extracted transaction information.
type TransactionDetails struct {
	Amount       float64 `json:"amount"`
	Timestamp    string  `json:"timestamp"`
	MerchantInfo string  `json:"merchant_info"`
	Category     string  `json:"category"`
	// Bucket classifies the expense as Need/Want/Investment.
	Bucket string `json:"bucket"`
	Source string `json:"source"`
	// MessageID is the email message ID (used for marking as read after successful write).
	MessageID string `json:"-"`

	// Multi-currency support
	Currency         string   `json:"currency,omitempty"`          // e.g., "INR", "USD", "EUR"
	OriginalAmount   *float64 `json:"original_amount,omitempty"`   // If converted
	OriginalCurrency *string  `json:"original_currency,omitempty"` // Original currency if converted
	ExchangeRate     *float64 `json:"exchange_rate,omitempty"`     // Conversion rate if applicable

	// User-added fields
	Description string   `json:"description,omitempty"` // User-added description
	Labels      []string `json:"labels,omitempty"`      // User-added labels
}

// Reader reads transactions from a source and sends them to the provided channel.
// Implementations should close the channel when done or on error.
// The ackChan is used to receive acknowledgments of successfully written transactions.
type Reader interface {
	Read(ctx context.Context, out chan<- *TransactionDetails, ackChan <-chan string) error
}

// Writer consumes transactions from a channel and writes them to a destination.
// Successfully written transaction message IDs are sent to the ackChan.
type Writer interface {
	Write(ctx context.Context, in <-chan *TransactionDetails, ackChan chan<- string) error
}

// Rule defines an email matching rule for transaction extraction.
// Rules are reader-agnostic: each reader uses the fields appropriate to its context.
type Rule struct {
	Name            string
	SenderEmail     string         // Email sender to match (e.g., "alerts@icicibank.com")
	SubjectContains string         // Subject substring to match
	Amount          *regexp.Regexp // Regex to extract amount from email body
	MerchantInfo    *regexp.Regexp // Regex to extract merchant info from email body
	Enabled         bool
	Source          string // Transaction source identifier (e.g., "Credit Card - ICICI")
}

// BuildGmailQuery constructs a Gmail API query string from the rule's fields.
func (r *Rule) BuildGmailQuery() string {
	query := "is:unread"
	if r.SenderEmail != "" {
		query += fmt.Sprintf(" from:%s", r.SenderEmail)
	}
	if r.SubjectContains != "" {
		query += fmt.Sprintf(" subject:%q", r.SubjectContains)
	}
	return query
}

// MatchesEmail checks if an email matches this rule based on sender and subject.
// Used by readers that don't have query-based filtering (e.g., Thunderbird).
// The fromHeader parameter can be the full From header (e.g., "Bank <bank@example.com>").
func (r *Rule) MatchesEmail(fromHeader, subject string) bool {
	if r.SenderEmail != "" {
		// Case-insensitive check if the rule's sender email is contained in the From header
		if !containsIgnoreCase(fromHeader, r.SenderEmail) {
			return false
		}
	}
	if r.SubjectContains != "" {
		// Case-insensitive substring match
		if !containsIgnoreCase(subject, r.SubjectContains) {
			return false
		}
	}
	return true
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// Labels maps merchant names to their category and bucket classification.
type Labels map[string]struct {
	Category string `json:"category"`
	Bucket   string `json:"bucket"`
}

// LabelLookup returns the category and bucket for a merchant.
// Returns empty strings if the merchant is not found.
func (l Labels) LabelLookup(merchant string) (category, bucket string) {
	if val, exists := l[merchant]; exists {
		return val.Category, val.Bucket
	}
	return "", ""
}
