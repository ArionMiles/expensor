// Package extractor provides common transaction extraction logic for email readers.
package extractor

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

// ExtractTransactionDetails extracts transaction details from an email body using regex patterns.
//
// Amount extraction: group 1 of amountRegex is treated as the raw amount string (commas stripped).
//
// Merchant extraction: the first non-empty capture group of merchantRegex is used, which
// allows alternation patterns like `at (X) on|Info: (X)\.` where only one branch matches.
//
// Currency extraction: group 1 of currencyRegex is used as the ISO 4217 currency code
// (e.g. "INR", "USD", "EUR"). If currencyRegex is nil or produces no match, Currency is
// left empty and store ingestion will apply its own default (currently "INR").
func ExtractTransactionDetails(
	emailBody string,
	amountRegex, merchantRegex, currencyRegex *regexp.Regexp,
	receivedTime time.Time,
) *api.TransactionDetails {
	return &api.TransactionDetails{
		Timestamp:    receivedTime.Format(time.RFC3339),
		Amount:       extractAmount(emailBody, amountRegex),
		MerchantInfo: extractMerchant(emailBody, merchantRegex),
		Currency:     extractCurrency(emailBody, currencyRegex),
	}
}

// extractAmount returns the parsed amount from group 1 of amountRegex, or 0.
func extractAmount(body string, re *regexp.Regexp) float64 {
	if re == nil {
		return 0
	}
	m := re.FindStringSubmatch(body)
	if len(m) <= 1 {
		return 0
	}
	amount, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
	if err != nil {
		return 0
	}
	return amount
}

// extractMerchant returns the first non-empty capture group of merchantRegex.
// This supports alternation patterns where only one branch produces a match.
func extractMerchant(body string, re *regexp.Regexp) string {
	if re == nil {
		return ""
	}
	m := re.FindStringSubmatch(body)
	if len(m) <= 1 {
		return ""
	}
	for _, group := range m[1:] {
		if g := strings.TrimSpace(group); g != "" {
			return cleanMerchant(g)
		}
	}
	return ""
}

func cleanMerchant(v string) string {
	withoutTags := htmlTagPattern.ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(withoutTags)), " ")
}

// extractCurrency returns the trimmed currency code from group 1 of currencyRegex, or "".
func extractCurrency(body string, re *regexp.Regexp) string {
	if re == nil {
		return ""
	}
	m := re.FindStringSubmatch(body)
	if len(m) <= 1 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
