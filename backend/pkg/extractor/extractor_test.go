package extractor

import (
	"regexp"
	"testing"
	"time"
)

func TestExtractTransactionDetails(t *testing.T) {
	tests := []struct {
		name           string
		emailBody      string
		amountRegex    string
		merchantRegex  string
		expectedAmount float64
		expectedMerch  string
	}{
		{
			name:           "ICICI credit card style transaction",
			emailBody:      "Transaction alert: INR 1,234.56 spent at AMAZON on 2024-01-15.",
			amountRegex:    `INR\s*([\d,]+\.?\d*)`,
			merchantRegex:  `at\s+(\w+)\s+on`,
			expectedAmount: 1234.56,
			expectedMerch:  "AMAZON",
		},
		{
			name:           "HDFC credit card style transaction",
			emailBody:      "Alert: Rs.999.00 spent at SWIGGY on your HDFC Credit Card.",
			amountRegex:    `Rs\.([\d,]+\.?\d*)`,
			merchantRegex:  `at\s+(\w+)\s+on`,
			expectedAmount: 999.00,
			expectedMerch:  "SWIGGY",
		},
		{
			name:           "transaction with large amount",
			emailBody:      "Transaction: INR 1,23,456.78 at FLIPKART on 2024-01-15.",
			amountRegex:    `INR\s*([\d,]+\.?\d*)`,
			merchantRegex:  `at\s+(\w+)\s+on`,
			expectedAmount: 123456.78,
			expectedMerch:  "FLIPKART",
		},
		{
			name:           "simple transaction",
			emailBody:      "You spent Rs. 500 at ZOMATO.",
			amountRegex:    `Rs\.\s*([\d,]+\.?\d*)`,
			merchantRegex:  `at\s+(\w+)`,
			expectedAmount: 500,
			expectedMerch:  "ZOMATO",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Compile regexes
			amountRegex, err := regexp.Compile(tc.amountRegex)
			if err != nil {
				t.Fatalf("failed to compile amount regex: %v", err)
			}

			merchantRegex, err := regexp.Compile(tc.merchantRegex)
			if err != nil {
				t.Fatalf("failed to compile merchant regex: %v", err)
			}

			// Extract transaction details
			receivedTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
			result := ExtractTransactionDetails(tc.emailBody, amountRegex, merchantRegex, receivedTime)

			// Verify amount
			if result.Amount != tc.expectedAmount {
				t.Errorf("amount: got %v, want %v", result.Amount, tc.expectedAmount)
			}

			// Verify merchant
			if result.MerchantInfo != tc.expectedMerch {
				t.Errorf("merchant: got %q, want %q", result.MerchantInfo, tc.expectedMerch)
			}

			// Verify timestamp format (RFC3339)
			expectedTimestamp := "2024-01-15T14:30:45Z"
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
		{
			name:          "amount only - no merchant",
			emailBody:     "You have been charged INR 1,000.00",
			amountRegex:   `INR\s*([\d,]+\.?\d*)`,
			merchantRegex: `at\s+(\w+)`,
			wantAmount:    1000.00,
			wantMerchant:  "",
		},
		{
			name:          "merchant only - no amount",
			emailBody:     "Purchase confirmed at AMAZON",
			amountRegex:   `INR\s*([\d,]+\.?\d*)`,
			merchantRegex: `at\s+(\w+)`,
			wantAmount:    0,
			wantMerchant:  "AMAZON",
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

func TestExtractTransactionDetails_NilRegex(t *testing.T) {
	emailBody := "Transaction of INR 1,000 at AMAZON"
	receivedTime := time.Now()

	// Test with nil amount regex
	result := ExtractTransactionDetails(emailBody, nil, nil, receivedTime)

	if result.Amount != 0 {
		t.Errorf("expected amount 0 with nil regex, got %v", result.Amount)
	}
	if result.MerchantInfo != "" {
		t.Errorf("expected empty merchant with nil regex, got %q", result.MerchantInfo)
	}
	if result.Timestamp == "" {
		t.Error("expected timestamp to be set even with nil regexes")
	}
}
