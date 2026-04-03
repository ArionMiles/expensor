// Package extractor provides common transaction extraction logic for email readers.
package extractor

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

// ExtractTransactionDetails extracts transaction details from an email body using regex patterns.
//
// Amount extraction: group 1 of amountRegex is treated as the raw amount string (commas stripped).
//
// Merchant extraction: the first non-empty capture group of merchantRegex is used, which
// allows alternation patterns like `at (X) on|Info: (X)\.` where only one branch matches.
//
// Currency extraction: group 1 of currencyRegex is used as the ISO 4217 currency code
// (e.g. "INR", "USD", "EUR"). If currencyRegex is nil or produces no match, Currency is
// left empty and the writer will apply its own default (currently "INR").
func ExtractTransactionDetails(
	emailBody string,
	amountRegex, merchantRegex, currencyRegex *regexp.Regexp,
	receivedTime time.Time,
) *api.TransactionDetails {
	transaction := &api.TransactionDetails{
		Timestamp: receivedTime.Format(time.RFC3339),
	}

	if amountRegex != nil {
		if m := amountRegex.FindStringSubmatch(emailBody); len(m) > 1 {
			amountStr := strings.ReplaceAll(m[1], ",", "")
			if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
				transaction.Amount = amount
			}
		}
	}

	if merchantRegex != nil {
		if m := merchantRegex.FindStringSubmatch(emailBody); len(m) > 1 {
			// Pick the first non-empty capture group to support alternation-based regexes.
			for _, group := range m[1:] {
				if g := strings.TrimSpace(group); g != "" {
					transaction.MerchantInfo = g
					break
				}
			}
		}
	}

	if currencyRegex != nil {
		if m := currencyRegex.FindStringSubmatch(emailBody); len(m) > 1 {
			if code := strings.TrimSpace(m[1]); code != "" {
				transaction.Currency = code
			}
		}
	}

	return transaction
}
