package extractor

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// emailBody extracts the body section from the Subject/Body file format.
// Files look like:
//
//	Subject: <subject>
//
//	Body:
//	<actual body>
//
// If the file has no "Body:" marker it is returned as-is (legacy format).
func emailBody(raw string) string {
	const marker = "Body:\n"
	if idx := strings.Index(raw, marker); idx >= 0 {
		return raw[idx+len(marker):]
	}
	return raw
}

func loadEmailFixture(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "..", "tests", "data", "emails", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	return emailBody(string(data))
}

// ─── Rule-based tests (one per email file) ───────────────────────────────────

// These use the exact regex patterns from rules.json so that extractor tests
// stay in sync with the deployed rules.

func TestExtractFromEmailFiles(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)

	tests := []struct {
		name            string
		fixture         string
		amountPattern   string
		merchantPattern string
		currencyPattern string
		wantAmount      float64
		wantMerchant    string
		wantCurrency    string
	}{
		{
			// tests/data/emails/hdfc_credit_card_01.txt
			name:            "HDFC Credit Card classic (INR, at…on merchant)",
			fixture:         "hdfc_credit_card_01.txt",
			amountPattern:   `Rs\.([\d,]+(?:\.\d+)?)`,
			merchantPattern: `at (.*?) on`,
			currencyPattern: "",
			wantAmount:      999.00,
			wantMerchant:    "SWIGGY",
			wantCurrency:    "",
		},
		{
			// tests/data/emails/hdfc_credit_card_02.txt
			name:            "HDFC Credit Card debit alert (INR, towards…on merchant)",
			fixture:         "hdfc_credit_card_02.txt",
			amountPattern:   `Rs\.([\d,]+(?:\.\d+)?)`,
			merchantPattern: `towards (.*?) on`,
			currencyPattern: "",
			wantAmount:      868.00,
			wantMerchant:    "WWW SWIGGY IN",
			wantCurrency:    "",
		},
		{
			// tests/data/emails/hdfc_upi_01.txt
			name:            "HDFC UPI (INR, @hdfcbank VPA)",
			fixture:         "hdfc_upi_01.txt",
			amountPattern:   `Rs\.([\d,]+(?:\.\d+)?)`,
			merchantPattern: `VPA \S+\s+(.*?)\s+on\s`,
			currencyPattern: "",
			wantAmount:      1200.00,
			wantMerchant:    "TIM HORTONS",
			wantCurrency:    "",
		},
		{
			// tests/data/emails/hdfc_upi_02.txt
			name:            "HDFC UPI (INR, third-party UPI provider)",
			fixture:         "hdfc_upi_02.txt",
			amountPattern:   `Rs\.([\d,]+(?:\.\d+)?)`,
			merchantPattern: `VPA \S+\s+(.*?)\s+on\s`,
			currencyPattern: "",
			wantAmount:      500.00,
			wantMerchant:    "SAMPLE MERCHANT NAME",
			wantCurrency:    "",
		},
		{
			// tests/data/emails/hdfc_upi_03.txt
			name:            "HDFC UPI captures merchant from parentheses",
			fixture:         "hdfc_upi_03.txt",
			amountPattern:   `Rs\.([\d,]+(?:\.\d+)?)`,
			merchantPattern: `VPA \S+\s+(?:\(([^)]*)\)|(.*?))\s+on\s`,
			currencyPattern: "",
			wantAmount:      750.00,
			wantMerchant:    "SAMPLE MERCHANT NAME",
			wantCurrency:    "",
		},
		{
			// tests/data/emails/icici_neft_imobile_01.txt
			name:            "ICICI NEFT iMobile (INR, towards…on merchant)",
			fixture:         "icici_neft_imobile_01.txt",
			amountPattern:   `Rs\.\s*([\d,]+(?:\.\d+)?)`,
			merchantPattern: `towards (.*?) on`,
			currencyPattern: "",
			wantAmount:      60000.00,
			wantMerchant:    "ACME INC",
			wantCurrency:    "",
		},
		{
			// tests/data/emails/icici_debit_card_01.txt
			name:            "ICICI Debit Card (INR, Info merchant)",
			fixture:         "icici_debit_card_01.txt",
			amountPattern:   `Rs\.\s*([\d,]+(?:\.\d+)?)`,
			merchantPattern: `Info:\s*(.*?)\.`,
			currencyPattern: "",
			wantAmount:      92215.00,
			wantMerchant:    "VPS*SHREE DHOLI",
			wantCurrency:    "",
		},
		{
			// tests/data/emails/icici_credit_card_04_inr_bank_in.txt
			name:            "ICICI Credit Card INR sender variant (Info merchant)",
			fixture:         "icici_credit_card_04_inr_bank_in.txt",
			amountPattern:   `(?:INR|USD|EUR) ([\d,]+(?:\.\d+)?)`,
			merchantPattern: `(?:at ([A-Z][A-Z ]*?) on|Info: (.*?)\.)`,
			currencyPattern: `(INR|USD|EUR)`,
			wantAmount:      1999.00,
			wantMerchant:    "OPENAI *CHATGPT SUBSCR",
			wantCurrency:    "INR",
		},
		{
			// tests/data/emails/icici_credit_card_01.txt — INR domestic (at…on merchant)
			name:            "ICICI Credit Card INR domestic (at…on merchant)",
			fixture:         "icici_credit_card_01.txt",
			amountPattern:   `(?:INR|USD|EUR) ([\d,]+(?:\.\d+)?)`,
			merchantPattern: `(?:at ([A-Z][A-Z ]*?) on|Info: (.*?)\.)`,
			currencyPattern: `(INR|USD|EUR)`,
			wantAmount:      1234.56,
			wantMerchant:    "AMAZON",
			wantCurrency:    "INR",
		},
		{
			// tests/data/emails/icici_credit_card_02_usd.txt — USD international (Info: merchant)
			name:            "ICICI Credit Card USD international (Info: merchant)",
			fixture:         "icici_credit_card_02_usd.txt",
			amountPattern:   `(?:INR|USD|EUR) ([\d,]+(?:\.\d+)?)`,
			merchantPattern: `(?:at ([A-Z][A-Z ]*?) on|Info: (.*?)\.)`,
			currencyPattern: `(INR|USD|EUR)`,
			wantAmount:      5.90,
			wantMerchant:    "ANTHROPIC",
			wantCurrency:    "USD",
		},
		{
			// tests/data/emails/icici_credit_card_03_eur.txt — EUR international (Info: merchant)
			name:            "ICICI Credit Card EUR international (Info: merchant)",
			fixture:         "icici_credit_card_03_eur.txt",
			amountPattern:   `(?:INR|USD|EUR) ([\d,]+(?:\.\d+)?)`,
			merchantPattern: `(?:at ([A-Z][A-Z ]*?) on|Info: (.*?)\.)`,
			currencyPattern: `(INR|USD|EUR)`,
			wantAmount:      20.10,
			wantMerchant:    "NETCUP",
			wantCurrency:    "EUR",
		},
		{
			// tests/data/emails/hdfc_credit_card_03_standing_instructions.txt
			name:            "HDFC Credit Card standing instructions (Rs. with space)",
			fixture:         "hdfc_credit_card_03_standing_instructions.txt",
			amountPattern:   `Rs\.\s*([\d,]+(?:\.\d+)?)`,
			merchantPattern: `at (.*?) on`,
			currencyPattern: "",
			wantAmount:      149.0,
			wantMerchant:    "SPOTIFYINDIA",
			wantCurrency:    "",
		},
		{
			// tests/data/emails/hdfc_credit_card_04_payment_made.txt
			name:            "HDFC Credit Card payment made alert (towards…on merchant)",
			fixture:         "hdfc_credit_card_04_payment_made.txt",
			amountPattern:   `Rs\.\s*([\d,]+(?:\.\d+)?)`,
			merchantPattern: `(?:\bat\b (.*?) on|\btowards\b (.*?) on)`,
			currencyPattern: "",
			wantAmount:      1007.00,
			wantMerchant:    "WWW SWIGGY IN",
			wantCurrency:    "",
		},
		{
			// tests/data/emails/axis_credit_card_01.html
			name:            "Axis Bank Credit Card (HTML, INR, Merchant Name block)",
			fixture:         "axis_credit_card_01.html",
			amountPattern:   `Transaction Amount:\s*</div>\s*<div[^>]*>\s*[A-Z]{3}\s*(?:&nbsp;|\s)*([\d,.]+)`,
			merchantPattern: `Merchant Name:\s*</div>\s*<div[^>]*>\s*(.*?)\s*(?:<br\s*/?>|\s*</div>)`,
			currencyPattern: `Transaction Amount:\s*</div>\s*<div[^>]*>\s*([A-Z]{3})`,
			wantAmount:      4999,
			wantMerchant:    "ACME STORE",
			wantCurrency:    "INR",
		},
		{
			// tests/data/emails/icici_imobile_01.txt — INR fund transfer (towards…on merchant)
			name:            "ICICI iMobile fund transfer (INR, towards…on merchant)",
			fixture:         "icici_imobile_01.txt",
			amountPattern:   `Rs ([\d,]+(?:\.\d+)?)`,
			merchantPattern: `towards (.*?) on`,
			currencyPattern: "",
			wantAmount:      38500.00,
			wantMerchant:    "Luke Skywalker",
			wantCurrency:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := loadEmailFixture(t, tc.fixture)
			amountRe := regexp.MustCompile(tc.amountPattern)
			merchantRe := regexp.MustCompile(tc.merchantPattern)
			var currencyRe *regexp.Regexp
			if tc.currencyPattern != "" {
				currencyRe = regexp.MustCompile(tc.currencyPattern)
			}

			result := ExtractTransactionDetails(body, amountRe, merchantRe, currencyRe, fixedTime)

			if result.Amount != tc.wantAmount {
				t.Errorf("amount: got %v, want %v", result.Amount, tc.wantAmount)
			}
			if result.MerchantInfo != tc.wantMerchant {
				t.Errorf("merchant: got %q, want %q", result.MerchantInfo, tc.wantMerchant)
			}
			if result.Currency != tc.wantCurrency {
				t.Errorf("currency: got %q, want %q", result.Currency, tc.wantCurrency)
			}
		})
	}
}

// ─── Generic extractor tests ─────────────────────────────────────────────────

func TestExtractTransactionDetails(t *testing.T) {
	tests := []struct {
		name             string
		emailBody        string
		amountRegex      string
		merchantRegex    string
		currencyRegex    string
		expectedAmount   float64
		expectedMerch    string
		expectedCurrency string
	}{
		{
			name:             "ICICI INR transaction",
			emailBody:        "Your ICICI Bank Credit Card has been used for a transaction of INR 1,234.56 at AMAZON on 2024-01-15.",
			amountRegex:      `(?:INR|USD|EUR) ([\d,]+\.?\d*)`,
			merchantRegex:    `(?:at ([A-Z][A-Z ]*?) on|Info: (.*?)\.)`,
			currencyRegex:    `(INR|USD|EUR)`,
			expectedAmount:   1234.56,
			expectedMerch:    "AMAZON",
			expectedCurrency: "INR",
		},
		{
			name:             "ICICI USD transaction with Info merchant",
			emailBody:        "Your ICICI Bank Credit Card has been used for a transaction of USD 5.90 on Apr 02, 2026 at 10:45:06. Info: ANTHROPIC.",
			amountRegex:      `(?:INR|USD|EUR) ([\d,]+\.?\d*)`,
			merchantRegex:    `(?:at ([A-Z][A-Z ]*?) on|Info: (.*?)\.)`,
			currencyRegex:    `(INR|USD|EUR)`,
			expectedAmount:   5.90,
			expectedMerch:    "ANTHROPIC",
			expectedCurrency: "USD",
		},
		{
			name:             "HDFC Rs. transaction",
			emailBody:        "Alert: Rs.999.00 spent at SWIGGY on your HDFC Credit Card.",
			amountRegex:      `Rs\.([\d,]+\.?\d*)`,
			merchantRegex:    `at\s+(\w+)\s+on`,
			currencyRegex:    "",
			expectedAmount:   999.00,
			expectedMerch:    "SWIGGY",
			expectedCurrency: "",
		},
		{
			name:             "large INR amount with commas",
			emailBody:        "Transaction: INR 1,23,456.78 at FLIPKART on 2024-01-15.",
			amountRegex:      `(?:INR|USD|EUR) ([\d,]+\.?\d*)`,
			merchantRegex:    `at\s+(\w+)\s+on`,
			currencyRegex:    `(INR|USD|EUR)`,
			expectedAmount:   123456.78,
			expectedMerch:    "FLIPKART",
			expectedCurrency: "INR",
		},
		{
			name:             "simple Rs. transaction",
			emailBody:        "You spent Rs. 500 at ZOMATO.",
			amountRegex:      `Rs\.\s*([\d,]+\.?\d*)`,
			merchantRegex:    `at\s+(\w+)`,
			currencyRegex:    "",
			expectedAmount:   500,
			expectedMerch:    "ZOMATO",
			expectedCurrency: "",
		},
	}

	fixedTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			amountRe := regexp.MustCompile(tc.amountRegex)
			merchantRe := regexp.MustCompile(tc.merchantRegex)
			var currencyRe *regexp.Regexp
			if tc.currencyRegex != "" {
				currencyRe = regexp.MustCompile(tc.currencyRegex)
			}

			result := ExtractTransactionDetails(tc.emailBody, amountRe, merchantRe, currencyRe, fixedTime)

			if result.Amount != tc.expectedAmount {
				t.Errorf("amount: got %v, want %v", result.Amount, tc.expectedAmount)
			}
			if result.MerchantInfo != tc.expectedMerch {
				t.Errorf("merchant: got %q, want %q", result.MerchantInfo, tc.expectedMerch)
			}
			if result.Currency != tc.expectedCurrency {
				t.Errorf("currency: got %q, want %q", result.Currency, tc.expectedCurrency)
			}

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
		currencyRegex string
		wantAmount    float64
		wantMerchant  string
		wantCurrency  string
	}{
		{
			name:          "no amount match",
			emailBody:     "Hello, this is a test email with no transaction.",
			amountRegex:   `(?:INR|USD|EUR) ([\d,]+\.?\d*)`,
			merchantRegex: `at\s+(\w+)`,
			currencyRegex: `(INR|USD|EUR)`,
			wantAmount:    0,
			wantMerchant:  "",
			wantCurrency:  "",
		},
		{
			name:          "amount with commas",
			emailBody:     "Transaction of INR 12,34,567.89 at FLIPKART",
			amountRegex:   `(?:INR|USD|EUR) ([\d,]+\.?\d*)`,
			merchantRegex: `at\s+(\w+)`,
			currencyRegex: `(INR|USD|EUR)`,
			wantAmount:    1234567.89,
			wantMerchant:  "FLIPKART",
			wantCurrency:  "INR",
		},
		{
			name:          "amount without decimals",
			emailBody:     "Transaction of Rs.500 at ZOMATO completed",
			amountRegex:   `Rs\.([\d,]+\.?\d*)`,
			merchantRegex: `at\s+(\w+)`,
			currencyRegex: "",
			wantAmount:    500,
			wantMerchant:  "ZOMATO",
			wantCurrency:  "",
		},
		{
			name:          "amount only - no merchant",
			emailBody:     "You have been charged INR 1,000.00",
			amountRegex:   `(?:INR|USD|EUR) ([\d,]+\.?\d*)`,
			merchantRegex: `at\s+(\w+)`,
			currencyRegex: `(INR|USD|EUR)`,
			wantAmount:    1000.00,
			wantMerchant:  "",
			wantCurrency:  "INR",
		},
		{
			name:          "merchant only - no amount",
			emailBody:     "Purchase confirmed at AMAZON",
			amountRegex:   `(?:INR|USD|EUR) ([\d,]+\.?\d*)`,
			merchantRegex: `at\s+(\w+)`,
			currencyRegex: `(INR|USD|EUR)`,
			wantAmount:    0,
			wantMerchant:  "AMAZON",
			wantCurrency:  "",
		},
		{
			name:          "alternation merchant - first branch matches",
			emailBody:     "transaction of INR 100.00 at SWIGGY on 01-Jan-2024.",
			amountRegex:   `(?:INR|USD|EUR) ([\d,]+\.?\d*)`,
			merchantRegex: `(?:at ([A-Z][A-Z ]*?) on|Info: (.*?)\.)`,
			currencyRegex: `(INR|USD|EUR)`,
			wantAmount:    100.00,
			wantMerchant:  "SWIGGY",
			wantCurrency:  "INR",
		},
		{
			name:          "alternation merchant - second branch matches",
			emailBody:     "transaction of USD 42.00 on 01-Jan-2024. Info: NETFLIX.",
			amountRegex:   `(?:INR|USD|EUR) ([\d,]+\.?\d*)`,
			merchantRegex: `(?:at ([A-Z][A-Z ]*?) on|Info: (.*?)\.)`,
			currencyRegex: `(INR|USD|EUR)`,
			wantAmount:    42.00,
			wantMerchant:  "NETFLIX",
			wantCurrency:  "USD",
		},
		{
			name:          "hdfc combined regex does not match 'that' as 'at'",
			emailBody:     "We would like to inform you that Rs. 1007.00 has been debited from your HDFC Bank Credit Card ending 1234 towards WWW SWIGGY IN on 16 Apr, 2026 at 10:28:40.",
			amountRegex:   `Rs\.\s*([\d,]+(?:\.\d+)?)`,
			merchantRegex: `(?:\bat\b (.*?) on|\btowards\b (.*?) on)`,
			currencyRegex: "",
			wantAmount:    1007.00,
			wantMerchant:  "WWW SWIGGY IN",
			wantCurrency:  "",
		},
		{
			name:          "merchant html tags are stripped",
			emailBody:     "Rs. 1007.00 has been debited from your HDFC Bank Credit Card ending <b>1234</b> towards <b>WWW SWIGGY IN</b> on 16 Apr, 2026 at 10:28:40.",
			amountRegex:   `Rs\.\s*([\d,]+(?:\.\d+)?)`,
			merchantRegex: `\btowards\b (.*?) on`,
			currencyRegex: "",
			wantAmount:    1007.00,
			wantMerchant:  "WWW SWIGGY IN",
			wantCurrency:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			amountRe := regexp.MustCompile(tc.amountRegex)
			merchantRe := regexp.MustCompile(tc.merchantRegex)
			var currencyRe *regexp.Regexp
			if tc.currencyRegex != "" {
				currencyRe = regexp.MustCompile(tc.currencyRegex)
			}
			receivedTime := time.Now()

			result := ExtractTransactionDetails(tc.emailBody, amountRe, merchantRe, currencyRe, receivedTime)

			if result.Amount != tc.wantAmount {
				t.Errorf("amount: got %v, want %v", result.Amount, tc.wantAmount)
			}
			if result.MerchantInfo != tc.wantMerchant {
				t.Errorf("merchant: got %q, want %q", result.MerchantInfo, tc.wantMerchant)
			}
			if result.Currency != tc.wantCurrency {
				t.Errorf("currency: got %q, want %q", result.Currency, tc.wantCurrency)
			}
		})
	}
}

func TestExtractTransactionDetails_NilRegex(t *testing.T) {
	emailBody := "Transaction of INR 1,000 at AMAZON"
	receivedTime := time.Now()

	result := ExtractTransactionDetails(emailBody, nil, nil, nil, receivedTime)

	if result.Amount != 0 {
		t.Errorf("expected amount 0 with nil regex, got %v", result.Amount)
	}
	if result.MerchantInfo != "" {
		t.Errorf("expected empty merchant with nil regex, got %q", result.MerchantInfo)
	}
	if result.Currency != "" {
		t.Errorf("expected empty currency with nil regex, got %q", result.Currency)
	}
	if result.Timestamp == "" {
		t.Error("expected timestamp to be set even with nil regexes")
	}
}
