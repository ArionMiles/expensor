// Package api defines the core interfaces and data structures for expensor.
package api

import (
	"context"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

const (
	// FailureAmountZero indicates extraction produced a zero amount.
	FailureAmountZero = "amount_zero"
	// FailureMerchantEmpty indicates extraction produced no merchant text.
	FailureMerchantEmpty = "merchant_empty"
)

// TransactionDetails holds extracted transaction information.
type TransactionDetails struct {
	Amount       float64 `json:"amount"`
	Timestamp    string  `json:"timestamp"`
	MerchantInfo string  `json:"merchant_info"`
	Category     string  `json:"category"`
	// Bucket classifies the expense as Need/Want/Investment.
	Bucket string `json:"bucket"`
	Source Source `json:"source"`
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

// Source describes where a transaction originated.
type Source struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Bank  string `json:"bank"`
}

// Display returns a compact fallback label for places that still need a string.
func (s Source) Display() string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(s.Bank) != "" {
		parts = append(parts, strings.TrimSpace(s.Bank))
	}
	if strings.TrimSpace(s.Type) != "" {
		parts = append(parts, strings.TrimSpace(s.Type))
	}
	display := strings.Join(parts, " ")
	label := strings.TrimSpace(s.Label)
	if label != "" && !strings.EqualFold(label, display) {
		parts = append(parts, label)
	}
	return strings.Join(parts, " ")
}

// Reader reads transactions from a source and sends them to the provided channel.
// Implementations should close the channel when done or on error.
// The ackChan is used to receive acknowledgments of successfully written transactions.
type Reader interface {
	Read(ctx context.Context, out chan<- *TransactionDetails, ackChan <-chan string) error
}

// EmailSearcher searches emails for rule authoring samples.
type EmailSearcher interface {
	Search(ctx context.Context, query EmailSearchQuery) ([]EmailSearchResult, error)
}

// EmailSearchQuery describes an email search request.
type EmailSearchQuery struct {
	// SubjectQuery is a case-insensitive subject substring to search for.
	SubjectQuery string
	// Limit is validated by the caller. HTTP allows 1..50 and defaults omitted values to 10.
	Limit int
}

// EmailSearchResult is a full email search result suitable for rule authoring.
type EmailSearchResult struct {
	// ID is provider-local and only meaningful within the provider that returned it.
	ID          string
	SenderEmail string
	Subject     string
	Body        string
	ReceivedAt  *time.Time
}

// Rule defines an email matching rule for transaction extraction.
// Rules are reader-agnostic: each reader uses the fields appropriate to its context.
type Rule struct {
	ID              string
	Name            string
	SenderEmail     string         // Email sender to match (e.g., "alerts@icicibank.com")
	SenderEmails    []string       // Exact sender email addresses to match.
	SubjectContains string         // Subject substring to match
	Amount          *regexp.Regexp // Regex to extract amount (group 1 = numeric amount, commas stripped)
	MerchantInfo    *regexp.Regexp // Regex to extract merchant; first non-empty capture group is used
	Currency        *regexp.Regexp // Regex to extract ISO currency code (group 1 = code, e.g. "INR", "USD")
	Source          Source         // Transaction source metadata.
}

// RuleDiagnosticSnapshot captures the diagnostic fields from a rule at extraction time.
type RuleDiagnosticSnapshot struct {
	RuleID        string
	RuleName      string
	AmountRegex   string
	MerchantRegex string
	CurrencyRegex string
}

// ExtractionDiagnostic records context for an extraction attempt that did not produce a usable transaction.
type ExtractionDiagnostic struct {
	Reader         string
	MessageID      string
	Source         string
	Sender         string
	SenderEmail    string
	Subject        string
	EmailBody      string
	ReceivedAt     *time.Time
	Snippet        string
	RuleID         string
	RuleName       string
	AmountRegex    string
	MerchantRegex  string
	CurrencyRegex  string
	FailureReasons []string
}

// DiagnosticSink persists extraction diagnostics for later inspection.
type DiagnosticSink interface {
	RecordExtractionDiagnostic(ctx context.Context, diagnostic ExtractionDiagnostic) error
}

// DiagnosticSnapshot returns a diagnostic-safe copy of a rule's identity and regex strings.
func (r Rule) DiagnosticSnapshot() RuleDiagnosticSnapshot {
	snapshot := RuleDiagnosticSnapshot{
		RuleID:   r.ID,
		RuleName: r.Name,
	}
	if r.Amount != nil {
		snapshot.AmountRegex = r.Amount.String()
	}
	if r.MerchantInfo != nil {
		snapshot.MerchantRegex = r.MerchantInfo.String()
	}
	if r.Currency != nil {
		snapshot.CurrencyRegex = r.Currency.String()
	}
	return snapshot
}

// ExtractionFailureReasons returns diagnostic reason codes for missing required transaction fields.
func ExtractionFailureReasons(transaction *TransactionDetails) []string {
	if transaction == nil {
		return nil
	}
	reasons := make([]string, 0, 2)
	if transaction.Amount == 0 {
		reasons = append(reasons, FailureAmountZero)
	}
	if strings.TrimSpace(transaction.MerchantInfo) == "" {
		reasons = append(reasons, FailureMerchantEmpty)
	}
	return reasons
}

// MatchesEmail checks if an email matches this rule based on sender and subject.
// Used by readers that don't have query-based filtering (e.g., Thunderbird).
// The fromHeader parameter can be the full From header (e.g., "Bank <bank@example.com>").
func (r *Rule) MatchesEmail(fromHeader, subject string) bool {
	senders := r.normalizedSenders()
	if len(senders) > 0 {
		fromEmail := normalizeEmailAddress(fromHeader)
		if fromEmail == "" || !emailInList(fromEmail, senders) {
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

func (r *Rule) normalizedSenders() []string {
	raw := r.SenderEmails
	if len(raw) == 0 && r.SenderEmail != "" {
		raw = []string{r.SenderEmail}
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, sender := range raw {
		normalized := normalizeEmailAddress(sender)
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

func normalizeEmailAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := mail.ParseAddress(value); err == nil {
		value = parsed.Address
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func emailInList(email string, senders []string) bool {
	for _, sender := range senders {
		if strings.EqualFold(email, sender) {
			return true
		}
	}
	return false
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// CategoryResolver returns the category and bucket for a merchant string.
// Implementations perform substring matching against a community-maintained
// fragment list. Returns ("", "") when no fragment matches; callers treat
// this as "Uncategorized".
type CategoryResolver func(merchantInfo string) (category, bucket string)
