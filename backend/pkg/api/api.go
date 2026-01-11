// Package api defines the core interfaces and data structures for expensor.
package api

import (
	"context"
	"regexp"
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
type Rule struct {
	Name         string
	Query        string
	Amount       *regexp.Regexp
	MerchantInfo *regexp.Regexp
	Enabled      bool
	Source       string
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
