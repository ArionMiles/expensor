package gmail

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/pkg/api"
)

// TestCase represents a single test case for transaction extraction.
type TestCase struct {
	Name           string
	EmailFile      string
	AmountRegex    string
	MerchantRegex  string
	ExpectedAmount float64
	ExpectedMerch  string
}

func TestExtractTransactionDetails(t *testing.T) {
	tests := []TestCase{
		{
			Name:           "ICICI credit card transaction",
			EmailFile:      "icici_credit_card_01.txt",
			AmountRegex:    `INR\s*([\d,]+\.?\d*)`,
			MerchantRegex:  `at\s+(\w+)\s+on`,
			ExpectedAmount: 1234.56,
			ExpectedMerch:  "AMAZON",
		},
		{
			Name:           "HDFC credit card transaction",
			EmailFile:      "hdfc_credit_card_01.txt",
			AmountRegex:    `Rs\.([\d,]+\.?\d*)`,
			MerchantRegex:  `at\s+(\w+)\s+on`,
			ExpectedAmount: 999.00,
			ExpectedMerch:  "SWIGGY",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			// Load email body from file
			emailBody, err := loadEmailFixture(tc.EmailFile)
			if err != nil {
				t.Fatalf("failed to load email fixture: %v", err)
			}

			// Compile regexes
			amountRegex, err := regexp.Compile(tc.AmountRegex)
			if err != nil {
				t.Fatalf("failed to compile amount regex: %v", err)
			}

			merchantRegex, err := regexp.Compile(tc.MerchantRegex)
			if err != nil {
				t.Fatalf("failed to compile merchant regex: %v", err)
			}

			// Extract transaction details
			receivedTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
			result := ExtractTransactionDetails(emailBody, amountRegex, merchantRegex, receivedTime)

			// Verify amount
			if result.Amount != tc.ExpectedAmount {
				t.Errorf("amount: got %v, want %v", result.Amount, tc.ExpectedAmount)
			}

			// Verify merchant
			if result.MerchantInfo != tc.ExpectedMerch {
				t.Errorf("merchant: got %q, want %q", result.MerchantInfo, tc.ExpectedMerch)
			}

			// Verify timestamp format
			expectedTimestamp := "2024-01-15 14:30:45"
			if result.Timestamp != expectedTimestamp {
				t.Errorf("timestamp: got %q, want %q", result.Timestamp, expectedTimestamp)
			}
		})
	}
}

func TestExtractTransactionDetails_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		emailBody     string
		amountRegex   string
		merchantRegex string
		wantAmount    float64
		wantMerchant  string
	}{
		{
			name:          "no amount match",
			emailBody:     "Hello, this is a test email with no transaction.",
			amountRegex:   `INR\s*([\d,]+\.?\d*)`,
			merchantRegex: `at\s+(\w+)`,
			wantAmount:    0,
			wantMerchant:  "",
		},
		{
			name:          "amount with commas",
			emailBody:     "Transaction of INR 12,34,567.89 at FLIPKART",
			amountRegex:   `INR\s*([\d,]+\.?\d*)`,
			merchantRegex: `at\s+(\w+)`,
			wantAmount:    1234567.89,
			wantMerchant:  "FLIPKART",
		},
		{
			name:          "amount without decimals",
			emailBody:     "Transaction of Rs.500 at ZOMATO completed",
			amountRegex:   `Rs\.([\d,]+\.?\d*)`,
			merchantRegex: `at\s+(\w+)`,
			wantAmount:    500,
			wantMerchant:  "ZOMATO",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			amountRegex := regexp.MustCompile(tc.amountRegex)
			merchantRegex := regexp.MustCompile(tc.merchantRegex)
			receivedTime := time.Now()

			result := ExtractTransactionDetails(tc.emailBody, amountRegex, merchantRegex, receivedTime)

			if result.Amount != tc.wantAmount {
				t.Errorf("amount: got %v, want %v", result.Amount, tc.wantAmount)
			}

			if result.MerchantInfo != tc.wantMerchant {
				t.Errorf("merchant: got %q, want %q", result.MerchantInfo, tc.wantMerchant)
			}
		})
	}
}

func TestLabelsLookup(t *testing.T) {
	labels := api.Labels{
		"AMAZON": {
			Category: "Shopping",
			Bucket:   "Wants",
		},
		"SWIGGY": {
			Category: "Food",
			Bucket:   "Wants",
		},
		"ELECTRICITY": {
			Category: "Utilities",
			Bucket:   "Needs",
		},
	}

	tests := []struct {
		merchant     string
		wantCategory string
		wantBucket   string
	}{
		{"AMAZON", "Shopping", "Wants"},
		{"SWIGGY", "Food", "Wants"},
		{"ELECTRICITY", "Utilities", "Needs"},
		{"UNKNOWN", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.merchant, func(t *testing.T) {
			category, bucket := labels.LabelLookup(tc.merchant)

			if category != tc.wantCategory {
				t.Errorf("category: got %q, want %q", category, tc.wantCategory)
			}

			if bucket != tc.wantBucket {
				t.Errorf("bucket: got %q, want %q", bucket, tc.wantBucket)
			}
		})
	}
}

// loadEmailFixture loads an email body from the tests/data/emails directory.
func loadEmailFixture(filename string) (string, error) {
	// Try relative path from test file location
	paths := []string{
		filepath.Join("..", "..", "..", "tests", "data", "emails", filename),
		filepath.Join("tests", "data", "emails", filename),
		filepath.Join("../../../tests/data/emails", filename),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
	}

	// Last resort: try from working directory
	data, err := os.ReadFile(filepath.Join("tests", "data", "emails", filename))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
