package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

type testIngestor struct {
	st     *Store
	tenant store.Tenant
}

func newTestIngestor(t *testing.T, overrides store.IngestionConfig) *testIngestor {
	t.Helper()
	ts := newTestStore(t)
	t.Cleanup(ts.cleanup)

	tenant := overrides.Tenant
	if tenant.ID == "" {
		tenant = testTenant(t, ts)
	}
	return &testIngestor{st: ts.Store, tenant: tenant}
}

// TestWrite_SingleTransaction verifies a single transaction is written and acknowledged.
func TestWrite_SingleTransaction(t *testing.T) {
	w := newTestIngestor(t, store.IngestionConfig{BatchSize: 1, FlushInterval: time.Second})

	txn := &api.TransactionDetails{
		MessageID:    fmt.Sprintf("test-msg-%d", time.Now().UnixNano()),
		Amount:       1234.56,
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: "Test Merchant",
		Category:     "Test Category",
		Bucket:       "Wants",
		Source:       api.Source{Label: "Test Source"},
		Description:  "Test transaction",
	}

	assertWrite(t, w, []*api.TransactionDetails{txn}, 5*time.Second)
}

func TestWrite_PersistsStructuredSourceFields(t *testing.T) {
	w := newTestIngestor(t, store.IngestionConfig{BatchSize: 1, FlushInterval: time.Second})
	ctx := context.Background()

	txn := &api.TransactionDetails{
		MessageID:    fmt.Sprintf("structured-source-%d", time.Now().UnixNano()),
		Amount:       999.00,
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: "Swiggy",
		Category:     "Food",
		Bucket:       "Wants",
		Source:       api.Source{Type: "Credit Card", Label: "HDFC Credit Card", Bank: "HDFC"},
	}

	assertWrite(t, w, []*api.TransactionDetails{txn}, 5*time.Second)

	var source, sourceType, sourceLabel, bank string
	err := poolForTest(w.st).QueryRow(ctx, `
		SELECT source, source_type, source_label, bank
		FROM transactions
		WHERE message_id = $1
	`, txn.MessageID).Scan(&source, &sourceType, &sourceLabel, &bank)
	if err != nil {
		t.Fatalf("query transaction source: %v", err)
	}
	if source != "HDFC Credit Card" {
		t.Fatalf("source = %q, want HDFC Credit Card", source)
	}
	if sourceType != "Credit Card" || sourceLabel != "HDFC Credit Card" || bank != "HDFC" {
		t.Fatalf("structured source = (%q, %q, %q)", sourceType, sourceLabel, bank)
	}
}

// TestWrite_MultiCurrency verifies a transaction with currency conversion fields is stored correctly.
func TestWrite_MultiCurrency(t *testing.T) {
	w := newTestIngestor(t, store.IngestionConfig{BatchSize: 1, FlushInterval: time.Second})

	originalAmount := 100.00
	originalCurrency := "USD"
	exchangeRate := 83.50

	txn := &api.TransactionDetails{
		MessageID:        fmt.Sprintf("test-usd-%d", time.Now().UnixNano()),
		Amount:           originalAmount * exchangeRate,
		Currency:         "INR",
		OriginalAmount:   &originalAmount,
		OriginalCurrency: &originalCurrency,
		ExchangeRate:     &exchangeRate,
		Timestamp:        time.Now().Format(time.RFC3339),
		MerchantInfo:     "Amazon.com",
		Category:         "Shopping",
		Bucket:           "Wants",
		Source:           api.Source{Label: "Credit Card - ICICI"},
	}

	assertWrite(t, w, []*api.TransactionDetails{txn}, 5*time.Second)
}

// TestWrite_WithLabels verifies that labels are persisted alongside the transaction.
func TestWrite_WithLabels(t *testing.T) {
	w := newTestIngestor(t, store.IngestionConfig{BatchSize: 1, FlushInterval: time.Second})

	txn := &api.TransactionDetails{
		MessageID:    fmt.Sprintf("test-labels-%d", time.Now().UnixNano()),
		Amount:       500.00,
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: "Starbucks",
		Category:     "Food",
		Bucket:       "Wants",
		Source:       api.Source{Label: "Credit Card - ICICI"},
		Description:  "Coffee with team",
		Labels:       []string{"work", "coffee", "team-expense"},
	}

	assertWrite(t, w, []*api.TransactionDetails{txn}, 5*time.Second)
}

// TestWrite_PersistsManualLabelProvenanceForPayloadLabels verifies that labels
// supplied in the transaction payload are tracked as manual provenance.
func TestWrite_PersistsManualLabelProvenanceForPayloadLabels(t *testing.T) {
	w := newTestIngestor(t, store.IngestionConfig{BatchSize: 1, FlushInterval: time.Second})
	ctx := context.Background()

	txn := &api.TransactionDetails{
		MessageID:    fmt.Sprintf("payload-labels-%d", time.Now().UnixNano()),
		Amount:       500.00,
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: "Netflix",
		Category:     "Entertainment",
		Bucket:       "Wants",
		Source:       api.Source{Label: "Credit Card - ICICI"},
		Labels:       []string{"subscription"},
	}

	assertWrite(t, w, []*api.TransactionDetails{txn}, 5*time.Second)

	var provenanceCount int
	err := poolForTest(w.st).QueryRow(ctx, `
		SELECT COUNT(*)
		FROM transaction_label_sources tls
		JOIN transactions t ON t.id = tls.transaction_id
		WHERE t.message_id = $1
		  AND tls.label = $2
		  AND tls.source_type = 'manual'
		  AND tls.merchant_pattern = ''
	`, txn.MessageID, "subscription").Scan(&provenanceCount)
	if err != nil {
		t.Fatalf("query label provenance: %v", err)
	}
	if provenanceCount != 1 {
		t.Fatalf("want 1 manual provenance row, got %d", provenanceCount)
	}
}

// TestWrite_Batch verifies that multiple transactions are written and all acknowledged.
func TestWrite_Batch(t *testing.T) {
	w := newTestIngestor(t, store.IngestionConfig{BatchSize: 5, FlushInterval: time.Second})

	txns := make([]*api.TransactionDetails, 10)
	for i := range txns {
		txns[i] = &api.TransactionDetails{
			MessageID:    fmt.Sprintf("test-batch-%d-%d", time.Now().UnixNano(), i),
			Amount:       float64(100 * (i + 1)),
			Currency:     "INR",
			Timestamp:    time.Now().Format(time.RFC3339),
			MerchantInfo: fmt.Sprintf("Merchant %d", i),
			Category:     "Test",
			Bucket:       "Wants",
			Source:       api.Source{Label: "Test Source"},
		}
	}

	assertWrite(t, w, txns, 10*time.Second)
}

// TestWrite_Upsert_PreservesUserEdits verifies that re-processing a transaction
// (same message_id) does not overwrite user-edited description, category, or bucket.
// Extracted fields (amount, merchant_info) must still be updated.
func TestWrite_Upsert_PreservesUserEdits(t *testing.T) {
	w := newTestIngestor(t, store.IngestionConfig{BatchSize: 1, FlushInterval: time.Second})
	ctx := context.Background()

	msgID := fmt.Sprintf("upsert-user-edits-%d", time.Now().UnixNano())

	// Step 1: write the transaction with initial extracted values.
	initial := &api.TransactionDetails{
		MessageID:    msgID,
		Amount:       500.00,
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: "Swiggy",
		Category:     "Food",
		Bucket:       "Wants",
		Source:       api.Source{Label: "Credit Card - HDFC"},
		Description:  "",
	}
	assertWrite(t, w, []*api.TransactionDetails{initial}, 5*time.Second)

	// Step 2: simulate user edits directly in the DB.
	_, err := poolForTest(w.st).Exec(ctx,
		`UPDATE transactions SET description = $1, category = $2, bucket = $3 WHERE message_id = $4`,
		"Anniversary dinner", "Dining Out", "Needs", msgID,
	)
	if err != nil {
		t.Fatalf("failed to simulate user edits: %v", err)
	}

	// Step 3: re-process the same email with updated extracted values (simulates retroactive scan).
	reprocessed := &api.TransactionDetails{
		MessageID:    msgID,
		Amount:       550.00, // amount changed (new regex)
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: "Swiggy Food",
		Category:     "Food & Dining", // extraction now returns a different category
		Bucket:       "Wants",
		Source:       api.Source{Label: "Credit Card - HDFC"},
		Description:  "some extracted description", // extraction should never overwrite description
	}
	assertWrite(t, w, []*api.TransactionDetails{reprocessed}, 5*time.Second)

	// Step 4: verify user-edited fields are preserved; extracted fields are updated.
	var gotDesc, gotCategory, gotBucket string
	var gotAmount float64
	var gotMerchant string
	err = poolForTest(w.st).QueryRow(ctx,
		`SELECT description, category, bucket, amount, merchant_info FROM transactions WHERE message_id = $1`,
		msgID,
	).Scan(&gotDesc, &gotCategory, &gotBucket, &gotAmount, &gotMerchant)
	if err != nil {
		t.Fatalf("failed to query transaction: %v", err)
	}

	// User-edited fields must be preserved.
	if gotDesc != "Anniversary dinner" {
		t.Errorf("description overwritten: got %q, want %q", gotDesc, "Anniversary dinner")
	}
	if gotCategory != "Dining Out" {
		t.Errorf("category overwritten: got %q, want %q", gotCategory, "Dining Out")
	}
	if gotBucket != "Needs" {
		t.Errorf("bucket overwritten: got %q, want %q", gotBucket, "Needs")
	}

	// Extracted fields must be updated from the re-processed values.
	if gotAmount != 550.00 {
		t.Errorf("amount not updated: got %f, want 550.00", gotAmount)
	}
	if gotMerchant != "Swiggy Food" {
		t.Errorf("merchant_info not updated: got %q, want %q", gotMerchant, "Swiggy Food")
	}
}

// TestWrite_Upsert_PopulatesEmptyCategoryBucket verifies that when a transaction has
// no user-set category or bucket, re-processing updates them from the extracted values.
func TestWrite_Upsert_PopulatesEmptyCategoryBucket(t *testing.T) {
	w := newTestIngestor(t, store.IngestionConfig{BatchSize: 1, FlushInterval: time.Second})

	msgID := fmt.Sprintf("upsert-empty-fields-%d", time.Now().UnixNano())

	// Write with empty category and bucket.
	initial := &api.TransactionDetails{
		MessageID:    msgID,
		Amount:       200.00,
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: "Uber",
		Category:     "",
		Bucket:       "",
		Source:       api.Source{Label: "UPI"},
	}
	assertWrite(t, w, []*api.TransactionDetails{initial}, 5*time.Second)

	// Re-process: extraction now returns category and bucket.
	reprocessed := &api.TransactionDetails{
		MessageID:    msgID,
		Amount:       200.00,
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: "Uber",
		Category:     "Transport",
		Bucket:       "Needs",
		Source:       api.Source{Label: "UPI"},
	}
	assertWrite(t, w, []*api.TransactionDetails{reprocessed}, 5*time.Second)

	var gotCategory, gotBucket string
	err := poolForTest(w.st).QueryRow(context.Background(),
		`SELECT category, bucket FROM transactions WHERE message_id = $1`, msgID,
	).Scan(&gotCategory, &gotBucket)
	if err != nil {
		t.Fatalf("failed to query transaction: %v", err)
	}
	if gotCategory != "Transport" {
		t.Errorf("category not populated from extraction: got %q, want %q", gotCategory, "Transport")
	}
	if gotBucket != "Needs" {
		t.Errorf("bucket not populated from extraction: got %q, want %q", gotBucket, "Needs")
	}
}

// TestWrite_AutoAppliesMerchantLabelToFutureTransactions verifies that a merchant
// label mapping is applied to newly written transactions.
func TestWrite_AutoAppliesMerchantLabelToFutureTransactions(t *testing.T) {
	w := newTestIngestor(t, store.IngestionConfig{BatchSize: 1, FlushInterval: time.Second})
	ctx := context.Background()

	const merchant = "Netflix"
	const label = "subscription"

	_, err := poolForTest(w.st).Exec(ctx, `
		INSERT INTO labels (tenant_id, name, color)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL DO NOTHING
	`, w.tenant.ID, label, "#f59e0b")
	if err != nil {
		t.Fatalf("seed label row: %v", err)
	}

	_, err = poolForTest(w.st).Exec(ctx, `
		INSERT INTO label_merchants (tenant_id, label, merchant_pattern) VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, label, merchant_pattern) WHERE tenant_id IS NOT NULL DO NOTHING
	`, w.tenant.ID, label, merchant)
	if err != nil {
		t.Fatalf("seed label mapping: %v", err)
	}

	txn := &api.TransactionDetails{
		MessageID:    fmt.Sprintf("future-label-%d", time.Now().UnixNano()),
		Amount:       499,
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: merchant,
		Category:     "Entertainment",
		Bucket:       "Wants",
		Source:       api.Source{Label: "Credit Card - HDFC"},
	}
	assertWrite(t, w, []*api.TransactionDetails{txn}, 5*time.Second)

	var labels []string
	err = poolForTest(w.st).QueryRow(ctx, `
		SELECT COALESCE(array_agg(label ORDER BY label), '{}')
		FROM transaction_labels tl
		JOIN transactions t ON t.id = tl.transaction_id
		WHERE t.message_id = $1
	`, txn.MessageID).Scan(&labels)
	if err != nil {
		t.Fatalf("query labels: %v", err)
	}
	if len(labels) != 1 || labels[0] != label {
		t.Fatalf("want labels [%q], got %v", label, labels)
	}
}

// TestWrite_AutoAppliesMerchantCategoryBucketToFutureTransactions verifies that
// merchant category rules follow resolver semantics for future transactions.
func TestWrite_AutoAppliesMerchantCategoryBucketToFutureTransactions(t *testing.T) {
	w := newTestIngestor(t, store.IngestionConfig{BatchSize: 1, FlushInterval: time.Second})
	ctx := context.Background()

	_, err := poolForTest(w.st).Exec(ctx, `
		INSERT INTO mcc_codes (code, description, category, bucket)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (code) DO UPDATE
		SET description = EXCLUDED.description,
		    category = EXCLUDED.category,
		    bucket = EXCLUDED.bucket,
		    updated_at = NOW()
	`, "4121", "Taxi", "Transport", "Needs")
	if err != nil {
		t.Fatalf("seed mcc code: %v", err)
	}

	_, err = poolForTest(w.st).Exec(ctx, `
		INSERT INTO merchant_categories (tenant_id, fragment, mcc_code, category, bucket, user_locked)
		VALUES ($1, $2, $3, NULL, NULL, true)
		ON CONFLICT (tenant_id, fragment) WHERE tenant_id IS NOT NULL DO UPDATE
		SET mcc_code = EXCLUDED.mcc_code,
		    category = EXCLUDED.category,
		    bucket = EXCLUDED.bucket,
		    user_locked = true
	`, w.tenant.ID, "Uber", "4121")
	if err != nil {
		t.Fatalf("seed MCC-backed merchant category: %v", err)
	}

	_, err = poolForTest(w.st).Exec(ctx, `
		INSERT INTO merchant_categories (tenant_id, fragment, category, bucket, user_locked)
		VALUES ($1, $2, $3, $4, true)
		ON CONFLICT (tenant_id, fragment) WHERE tenant_id IS NOT NULL DO UPDATE
		SET category = EXCLUDED.category,
		    bucket = EXCLUDED.bucket,
		    user_locked = true
	`, w.tenant.ID, "Uber Eats", "Food Delivery", "Wants")
	if err != nil {
		t.Fatalf("seed overlapping merchant category: %v", err)
	}

	txns := []*api.TransactionDetails{
		{
			MessageID:    fmt.Sprintf("future-category-uber-%d", time.Now().UnixNano()),
			Amount:       240,
			Currency:     "INR",
			Timestamp:    time.Now().Format(time.RFC3339),
			MerchantInfo: "Uber Black",
			Category:     "",
			Bucket:       "",
			Source:       api.Source{Label: "UPI"},
		},
		{
			MessageID:    fmt.Sprintf("future-category-uber-eats-%d", time.Now().UnixNano()),
			Amount:       650,
			Currency:     "INR",
			Timestamp:    time.Now().Format(time.RFC3339),
			MerchantInfo: "Uber Eats Pass",
			Category:     "",
			Bucket:       "",
			Source:       api.Source{Label: "UPI"},
		},
	}
	assertWrite(t, w, txns, 5*time.Second)

	check := func(messageID, wantCategory, wantBucket string) {
		t.Helper()
		var category, bucket string
		err = poolForTest(w.st).QueryRow(ctx, `
			SELECT category, bucket FROM transactions WHERE message_id = $1
		`, messageID).Scan(&category, &bucket)
		if err != nil {
			t.Fatalf("query categorized transaction %s: %v", messageID, err)
		}
		if category != wantCategory || bucket != wantBucket {
			t.Fatalf("want %s/%s, got %q/%q", wantCategory, wantBucket, category, bucket)
		}
	}
	check(txns[0].MessageID, "Transport", "Needs")
	check(txns[1].MessageID, "Food Delivery", "Wants")
}

// assertWrite persists txns through the Postgres batch writer.
func assertWrite(t *testing.T, w *testIngestor, txns []*api.TransactionDetails, timeout time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := w.st.Write(ctx, store.IngestionBatch{Tenant: w.tenant, Transactions: txns}); err != nil {
		t.Errorf("store Write returned error: %v", err)
	}
}
