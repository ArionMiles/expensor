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
func ExtractTransactionDetails(emailBody string, amountRegex, merchantRegex *regexp.Regexp, receivedTime time.Time) *api.TransactionDetails {
	transaction := &api.TransactionDetails{
		Timestamp: receivedTime.Format(time.RFC3339),
	}

	if amountRegex != nil {
		if amountMatches := amountRegex.FindStringSubmatch(emailBody); len(amountMatches) > 1 {
			amountStr := strings.ReplaceAll(amountMatches[1], ",", "")
			if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
				transaction.Amount = amount
			}
		}
	}

	if merchantRegex != nil {
		if merchantMatches := merchantRegex.FindStringSubmatch(emailBody); len(merchantMatches) > 1 {
			transaction.MerchantInfo = merchantMatches[1]
		}
	}

	return transaction
}
