package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

func TestStoreRepositoriesEmitDebugInstrumentation(t *testing.T) {
	ts := newInstrumentedTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	must(t, "CreateLabel", ts.CreateLabel(ctx, "instrumented", "#38bdf8"))
	must(t, "UpdateLabel", ts.UpdateLabel(ctx, "instrumented", "#f97316"))

	if _, err := ts.ApplyLabelByMerchant(ctx, "instrumented", "coffee"); err != nil {
		t.Fatalf("ApplyLabelByMerchant: %v", err)
	}
	if _, err := ts.GetLabelMappings(ctx); err != nil {
		t.Fatalf("GetLabelMappings: %v", err)
	}
	if _, err := ts.ListLabels(ctx); err != nil {
		t.Fatalf("ListLabels: %v", err)
	}

	must(t, "CreateCategory", ts.CreateCategory(ctx, "Instrumented Category", "covered by repository instrumentation"))
	if _, err := ts.ListCategories(ctx); err != nil {
		t.Fatalf("ListCategories: %v", err)
	}

	must(t, "CreateBucket", ts.CreateBucket(ctx, "Instrumented Bucket", "covered by repository instrumentation"))
	if _, err := ts.ListBuckets(ctx); err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}

	ts.requireOperation(t, "taxonomy.list_labels")
	ts.requireOperation(t, "taxonomy.create_label")
	ts.requireOperation(t, "taxonomy.update_label")
	ts.requireOperation(t, "taxonomy.apply_label_by_merchant")
	ts.requireOperation(t, "taxonomy.get_label_mappings")
	ts.requireOperation(t, "taxonomy.list_categories")
	ts.requireOperation(t, "taxonomy.create_category")
	ts.requireOperation(t, "taxonomy.list_buckets")
	ts.requireOperation(t, "taxonomy.create_bucket")
}

func TestStoreRuntimeRepositoryEmitsDebugInstrumentation(t *testing.T) {
	ts := newInstrumentedTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	must(t, "SetAppConfig", ts.SetAppConfig(ctx, "instrumented_config", "enabled"))
	if _, err := ts.GetAppConfig(ctx, "instrumented_config"); err != nil {
		t.Fatalf("GetAppConfig: %v", err)
	}
	must(t, "SetActiveReader", ts.SetActiveReader(ctx, "gmail"))
	if _, err := ts.GetActiveReader(ctx); err != nil {
		t.Fatalf("GetActiveReader: %v", err)
	}

	must(t, "SetReaderSecret", ts.SetReaderSecret(ctx, "gmail", []byte(`{"installed":{}}`)))
	if _, _, err := ts.GetReaderSecret(ctx, "gmail"); err != nil {
		t.Fatalf("GetReaderSecret: %v", err)
	}
	must(t, "SetReaderToken", ts.SetReaderToken(ctx, "gmail", []byte(`{"access_token":"a"}`)))
	if _, _, err := ts.GetReaderToken(ctx, "gmail"); err != nil {
		t.Fatalf("GetReaderToken: %v", err)
	}
	must(t, "DeleteReaderToken", ts.DeleteReaderToken(ctx, "gmail"))
	must(t, "SetReaderConfig", ts.SetReaderConfig(ctx, "thunderbird", json.RawMessage(`{"mailboxes":"Inbox"}`)))
	if _, _, err := ts.GetReaderConfig(ctx, "thunderbird"); err != nil {
		t.Fatalf("GetReaderConfig: %v", err)
	}
	must(t, "DeleteReaderRuntime", ts.DeleteReaderRuntime(ctx, "thunderbird"))

	must(t, "MarkMessageProcessed", ts.MarkMessageProcessed(ctx, "msg-instrumented", time.Now()))
	if _, err := ts.IsMessageProcessed(ctx, "msg-instrumented"); err != nil {
		t.Fatalf("IsMessageProcessed: %v", err)
	}
	must(t, "SetSyncStatus", ts.SetSyncStatus(ctx, store.SyncStatus{EntriesUpdated: 3}))
	if _, err := ts.GetSyncStatus(ctx); err != nil {
		t.Fatalf("GetSyncStatus: %v", err)
	}
	must(t, "SetCommunityURL", ts.SetCommunityURL(ctx, "https://example.invalid/content.json"))
	if _, err := ts.GetCommunityURL(ctx); err != nil {
		t.Fatalf("GetCommunityURL: %v", err)
	}

	ts.requireOperation(t, "runtime.set_app_config")
	ts.requireOperation(t, "runtime.get_app_config")
	ts.requireOperation(t, "runtime.set_active_reader")
	ts.requireOperation(t, "runtime.get_active_reader")
	ts.requireOperation(t, "runtime.set_reader_secret")
	ts.requireOperation(t, "runtime.get_reader_secret")
	ts.requireOperation(t, "runtime.set_reader_token")
	ts.requireOperation(t, "runtime.get_reader_token")
	ts.requireOperation(t, "runtime.delete_reader_token")
	ts.requireOperation(t, "runtime.set_reader_config")
	ts.requireOperation(t, "runtime.get_reader_config")
	ts.requireOperation(t, "runtime.delete_reader_runtime")
	ts.requireOperation(t, "runtime.mark_message_processed")
	ts.requireOperation(t, "runtime.is_message_processed")
	ts.requireOperation(t, "runtime.set_sync_status")
	ts.requireOperation(t, "runtime.get_sync_status")
	ts.requireOperation(t, "runtime.set_community_url")
	ts.requireOperation(t, "runtime.get_community_url")
}

func TestInitAppConfigLeavesBaseCurrencyUnset(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	if got, err := ts.GetAppConfig(ctx, "base_currency"); err == nil {
		t.Fatalf("base_currency = %q, want missing", got)
	}
}

func TestStoreRulesRepositoryEmitsDebugInstrumentation(t *testing.T) {
	ts := newInstrumentedTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	must(t, "SeedPredefinedRules", ts.SeedPredefinedRules(ctx, []store.RuleRow{{
		Name:              "Instrumented System Rule",
		SenderEmail:       "system@example.com",
		AmountRegex:       `INR ([0-9.]+)`,
		MerchantRegex:     `at (.+)`,
		TransactionSource: "System",
	}}))
	created, err := ts.CreateRule(ctx, store.RuleRow{
		Name:              "Instrumented User Rule",
		SenderEmail:       "user@example.com",
		AmountRegex:       `INR ([0-9.]+)`,
		MerchantRegex:     `at (.+)`,
		TransactionSource: "User",
	})
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if _, err := ts.GetRule(ctx, created.ID); err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	created.SubjectContains = "card"
	if _, err := ts.UpdateRule(ctx, created.ID, *created); err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	if _, err := ts.ListRules(ctx); err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	must(t, "ImportUserRules", ts.ImportUserRules(ctx, []store.RuleRow{{
		Name:              "Instrumented Imported Rule",
		SenderEmail:       "import@example.com",
		AmountRegex:       `INR ([0-9.]+)`,
		MerchantRegex:     `at (.+)`,
		TransactionSource: "Import",
	}}))
	must(t, "DeleteRule", ts.DeleteRule(ctx, created.ID))

	ts.requireOperation(t, "rules.seed_predefined")
	ts.requireOperation(t, "rules.create")
	ts.requireOperation(t, "rules.get")
	ts.requireOperation(t, "rules.update")
	ts.requireOperation(t, "rules.list")
	ts.requireOperation(t, "rules.import_user")
	ts.requireOperation(t, "rules.delete")
}

func TestStoreDiagnosticsRepositoryEmitsDebugInstrumentation(t *testing.T) {
	ts := newInstrumentedTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	receivedAt := time.Now()
	must(t, "RecordExtractionDiagnostic", ts.RecordExtractionDiagnostic(ctx, storeTestDiagnostic(receivedAt)))
	rows, err := ts.ListExtractionDiagnostics(ctx, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen, Limit: 5})
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected diagnostic row")
	}
	if _, err := ts.GetExtractionDiagnostic(ctx, rows[0].ID); err != nil {
		t.Fatalf("GetExtractionDiagnostic: %v", err)
	}
	if _, err := ts.UpdateExtractionDiagnosticStatus(ctx, rows[0].ID, store.DiagnosticStatusResolved); err != nil {
		t.Fatalf("UpdateExtractionDiagnosticStatus: %v", err)
	}

	ts.requireOperation(t, "diagnostics.record_extraction")
	ts.requireOperation(t, "diagnostics.list_extraction")
	ts.requireOperation(t, "diagnostics.get_extraction")
	ts.requireOperation(t, "diagnostics.update_extraction_status")
}

func TestStoreTransactionsRepositoryEmitsDebugInstrumentation(t *testing.T) {
	ts := newInstrumentedTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	id, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID:    "msg-transactions-instrumented",
		Amount:       42.75,
		Currency:     "INR",
		MerchantInfo: "Instrumented Coffee",
		Category:     "Food",
		Bucket:       "Needs",
		Source:       "gmail",
		Description:  "initial",
		Timestamp:    time.Now(),
	})
	if err != nil {
		t.Fatalf("InsertForTest: %v", err)
	}
	must(t, "CreateLabel", ts.CreateLabel(ctx, "transaction-instrumented", "#22c55e"))

	if _, _, err := ts.ListTransactions(ctx, store.ListFilter{}); err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if _, err := ts.GetTransaction(ctx, id); err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	must(t, "UpdateDescription", ts.UpdateDescription(ctx, id, "updated"))
	must(t, "AddLabel", ts.AddLabel(ctx, id, "transaction-instrumented"))
	must(t, "AddLabels", ts.AddLabels(ctx, id, []string{"transaction-instrumented"}))
	must(t, "RemoveLabel", ts.RemoveLabel(ctx, id, "transaction-instrumented"))
	if _, _, err := ts.SearchTransactions(ctx, "coffee", store.ListFilter{}); err != nil {
		t.Fatalf("SearchTransactions: %v", err)
	}
	if _, err := ts.GetFacets(ctx); err != nil {
		t.Fatalf("GetFacets: %v", err)
	}
	category := "Dining"
	bucket := "Wants"
	must(t, "UpdateTransaction", ts.UpdateTransaction(ctx, id, store.TransactionUpdate{
		Category: &category,
		Bucket:   &bucket,
	}))

	must(t, "MuteTransaction", ts.MuteTransaction(ctx, id, true, "duplicate"))
	must(t, "UpdateMuteReason", ts.UpdateMuteReason(ctx, id, "manual duplicate"))
	must(t, "MuteByMerchant", ts.MuteByMerchant(ctx, "Instrumented", "merchant rule"))
	muted, err := ts.GetMutedMerchantsWithCount(ctx)
	if err != nil {
		t.Fatalf("GetMutedMerchantsWithCount: %v", err)
	}
	if len(muted) == 0 {
		t.Fatal("expected muted merchant")
	}
	must(t, "UpdateMerchantReason", ts.UpdateMerchantReason(ctx, muted[0].ID, "updated reason"))
	if _, err := ts.ListMutedMerchants(ctx); err != nil {
		t.Fatalf("ListMutedMerchants: %v", err)
	}
	if _, err := ts.GetMutedMerchantPatterns(ctx); err != nil {
		t.Fatalf("GetMutedMerchantPatterns: %v", err)
	}
	must(t, "DeleteMutedMerchantAndUnmute", ts.DeleteMutedMerchantAndUnmute(ctx, muted[0].ID))

	ts.requireOperation(t, "transactions.list")
	ts.requireOperation(t, "transactions.get")
	ts.requireOperation(t, "transactions.update_description")
	ts.requireOperation(t, "transactions.add_label")
	ts.requireOperation(t, "transactions.add_labels")
	ts.requireOperation(t, "transactions.remove_label")
	ts.requireOperation(t, "transactions.search")
	ts.requireOperation(t, "transactions.get_facets")
	ts.requireOperation(t, "transactions.update")
	ts.requireOperation(t, "transactions.mute")
	ts.requireOperation(t, "transactions.update_mute_reason")
	ts.requireOperation(t, "transactions.mute_by_merchant")
	ts.requireOperation(t, "transactions.get_muted_merchants_with_count")
	ts.requireOperation(t, "transactions.update_merchant_reason")
	ts.requireOperation(t, "transactions.list_muted_merchants")
	ts.requireOperation(t, "transactions.get_muted_merchant_patterns")
	ts.requireOperation(t, "transactions.delete_muted_merchant_and_unmute")
}

func TestStoreCommunityRepositoryEmitsDebugInstrumentation(t *testing.T) {
	ts := newInstrumentedTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	id, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID:    "msg-community-instrumented",
		Amount:       99,
		Currency:     "INR",
		MerchantInfo: "Instrumented Grocery",
		Source:       "gmail",
		Timestamp:    time.Now(),
	})
	if err != nil {
		t.Fatalf("InsertForTest: %v", err)
	}
	must(t, "SeedMCCCodes", ts.SeedMCCCodes(ctx, []store.MCCEntry{{
		Code:        "5411",
		Description: "Grocery Stores",
		Category:    "Groceries",
		Bucket:      "Needs",
	}}))
	mcc := "5411"
	merchantCategory := "Groceries"
	merchantBucket := "Needs"
	if _, err := ts.SeedMerchantCategories(ctx, []store.MerchantCategoryEntry{{
		Fragment: "instrumented grocery",
		MCC:      &mcc,
		Category: &merchantCategory,
		Bucket:   &merchantBucket,
	}}); err != nil {
		t.Fatalf("SeedMerchantCategories: %v", err)
	}
	resolver, err := ts.LoadCategorySnapshot(ctx)
	if err != nil {
		t.Fatalf("LoadCategorySnapshot: %v", err)
	}
	if category, bucket := resolver("Instrumented Grocery"); category != "Groceries" || bucket != "Needs" {
		t.Fatalf("resolver = %q/%q", category, bucket)
	}
	must(t, "SeedMCCCategories", ts.SeedMCCCategories(ctx, []string{"Groceries"}))
	if rows, err := ts.CategorizeMerchant(ctx, "Instrumented Grocery", "Dining", "Wants"); err != nil || rows != 1 {
		t.Fatalf("CategorizeMerchant: rows=%d err=%v", rows, err)
	}
	if txn, err := ts.GetTransaction(ctx, id); err != nil || txn.Category != "Dining" || txn.Bucket != "Wants" {
		t.Fatalf("GetTransaction after categorize: txn=%+v err=%v", txn, err)
	}

	ts.requireOperation(t, "community.seed_mcc_codes")
	ts.requireOperation(t, "community.seed_merchant_categories")
	ts.requireOperation(t, "community.load_category_snapshot")
	ts.requireOperation(t, "community.seed_mcc_categories")
	ts.requireOperation(t, "community.categorize_merchant")
}

func TestStoreReadModelRepositoryEmitsDebugInstrumentation(t *testing.T) {
	ts := newInstrumentedTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	must(t, "SetAppConfig timezone", ts.SetAppConfig(ctx, "app.timezone", "Asia/Kolkata"))
	must(t, "SetAppConfig base currency", ts.SetAppConfig(ctx, "base_currency", "INR"))
	_, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID:    "msg-read-model-instrumented",
		Amount:       120,
		Currency:     "INR",
		MerchantInfo: "Instrumented Dashboard",
		Category:     "Dining",
		Bucket:       "Wants",
		Source:       "gmail",
		Timestamp:    time.Now(),
	})
	if err != nil {
		t.Fatalf("InsertForTest: %v", err)
	}

	if _, err := ts.GetStats(ctx, "INR"); err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if _, err := ts.GetChartData(ctx); err != nil {
		t.Fatalf("GetChartData: %v", err)
	}
	if _, err := ts.GetDashboardData(ctx); err != nil {
		t.Fatalf("GetDashboardData: %v", err)
	}
	if _, err := ts.GetSpendingHeatmap(ctx, nil, nil); err != nil {
		t.Fatalf("GetSpendingHeatmap: %v", err)
	}
	if _, err := ts.GetAnnualSpend(ctx, time.Now().Year()); err != nil {
		t.Fatalf("GetAnnualSpend: %v", err)
	}
	if _, err := ts.GetMonthlyBreakdownSpend(ctx, "categories", 3); err != nil {
		t.Fatalf("GetMonthlyBreakdownSpend: %v", err)
	}

	ts.requireOperation(t, "read_model.get_stats")
	ts.requireOperation(t, "read_model.get_chart_data")
	ts.requireOperation(t, "read_model.get_dashboard_data")
	ts.requireOperation(t, "read_model.get_spending_heatmap")
	ts.requireOperation(t, "read_model.get_annual_spend")
	ts.requireOperation(t, "read_model.get_monthly_breakdown_spend")
}

type instrumentedTestStore struct {
	*testStore
	logs *bytes.Buffer
}

func newInstrumentedTestStore(t *testing.T) *instrumentedTestStore {
	t.Helper()
	logs := new(bytes.Buffer)
	ts := newTestStoreWithLogger(t, logs)
	return &instrumentedTestStore{testStore: ts, logs: logs}
}

func (ts *instrumentedTestStore) requireOperation(t *testing.T, operation string) {
	t.Helper()
	got := ts.logs.String()
	if !strings.Contains(got, "level=DEBUG") {
		t.Fatalf("expected debug logs, got %q", got)
	}
	if !strings.Contains(got, operation) {
		t.Fatalf("expected operation %q in logs, got %q", operation, got)
	}
}

func must(t *testing.T, name string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func storeTestDiagnostic(receivedAt time.Time) api.ExtractionDiagnostic {
	return api.ExtractionDiagnostic{
		Reader:         "gmail",
		MessageID:      "msg-diagnostics-instrumented",
		Source:         "gmail",
		Sender:         "Bank",
		SenderEmail:    "bank@example.com",
		Subject:        "Debit alert",
		EmailBody:      "spent INR 42 at coffee",
		ReceivedAt:     &receivedAt,
		Snippet:        "spent INR 42",
		RuleName:       "Instrumented diagnostic rule",
		AmountRegex:    `INR ([0-9.]+)`,
		MerchantRegex:  `at (.+)`,
		CurrencyRegex:  `INR`,
		FailureReasons: []string{"amount not found"},
	}
}
