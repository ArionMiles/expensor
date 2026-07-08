package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestTenantScopedTransactionsAndReadModels(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	tenantA := store.Tenant{ID: createRuntimeTestUser(t, ts, "tenant-txn-a@example.com").TenantID}
	tenantB := store.Tenant{ID: createRuntimeTestUser(t, ts, "tenant-txn-b@example.com").TenantID}

	idA := seedTransaction(ctx, t, ts.Store, store.InsertParams{
		Tenant:       tenantA,
		MessageID:    "tenant-a-message",
		Amount:       10,
		Currency:     "INR",
		MerchantInfo: "Tenant A Store",
		Category:     "Food",
		Bucket:       "Needs",
		Timestamp:    time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC),
	})
	idB := seedTransaction(ctx, t, ts.Store, store.InsertParams{
		Tenant:       tenantB,
		MessageID:    "tenant-b-message",
		Amount:       99,
		Currency:     "INR",
		MerchantInfo: "Tenant B Store",
		Category:     "Travel",
		Bucket:       "Wants",
		Timestamp:    time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC),
	})

	txns, totals, err := ts.ListTransactions(ctx, tenantA, store.ListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListTransactions tenant A: %v", err)
	}
	if len(txns) != 1 || txns[0].ID != idA {
		t.Fatalf("tenant A transactions = %#v, want only %s", txns, idA)
	}
	if totals.Total != 1 || totals.TotalAmount != 10 {
		t.Fatalf("tenant A totals = %+v, want one INR 10 transaction", totals)
	}

	if _, err := ts.GetTransaction(ctx, tenantA, idB); errors.WhatKind(err) != errors.NotFound {
		t.Fatalf("GetTransaction tenant A on tenant B row error = %v, want NotFound kind", err)
	}
	if err := ts.UpdateTransaction(ctx, tenantA, idB, store.TransactionUpdate{Description: ptrString("cross tenant")}); errors.WhatKind(err) != errors.NotFound {
		t.Fatalf("UpdateTransaction tenant A on tenant B row error = %v, want NotFound kind", err)
	}

	stats, err := ts.GetStats(ctx, tenantA, "INR")
	if err != nil {
		t.Fatalf("GetStats tenant A: %v", err)
	}
	if stats.TotalCount != 1 || stats.TotalBase != 10 {
		t.Fatalf("tenant A stats = %+v, want one INR 10 transaction", stats)
	}

	facets, err := ts.GetFacets(ctx, tenantA)
	if err != nil {
		t.Fatalf("GetFacets tenant A: %v", err)
	}
	if len(facets.Merchants) != 1 || facets.Merchants[0] != "Tenant A Store" {
		t.Fatalf("tenant A merchant facets = %#v", facets.Merchants)
	}
}

func TestTenantScopedTaxonomyAndMutedMerchants(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	tenantA := store.Tenant{ID: createRuntimeTestUser(t, ts, "tenant-taxonomy-a@example.com").TenantID}
	tenantB := store.Tenant{ID: createRuntimeTestUser(t, ts, "tenant-taxonomy-b@example.com").TenantID}

	idA := seedTransaction(ctx, t, ts.Store, store.InsertParams{
		Tenant:       tenantA,
		MessageID:    "tenant-a-taxonomy",
		Amount:       10,
		Currency:     "INR",
		MerchantInfo: "Shared Merchant",
		Timestamp:    time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC),
	})
	idB := seedTransaction(ctx, t, ts.Store, store.InsertParams{
		Tenant:       tenantB,
		MessageID:    "tenant-b-taxonomy",
		Amount:       20,
		Currency:     "INR",
		MerchantInfo: "Shared Merchant",
		Timestamp:    time.Date(2026, 2, 1, 11, 0, 0, 0, time.UTC),
	})

	if err := ts.CreateLabel(ctx, tenantA, "shared", "#111111"); err != nil {
		t.Fatalf("CreateLabel tenant A: %v", err)
	}
	if err := ts.CreateLabel(ctx, tenantB, "shared", "#222222"); err != nil {
		t.Fatalf("CreateLabel tenant B: %v", err)
	}
	labels, err := ts.ListLabels(ctx, tenantA)
	if err != nil {
		t.Fatalf("ListLabels tenant A: %v", err)
	}
	if len(labels) != 1 || labels[0].Name != "shared" || labels[0].Color != "#111111" {
		t.Fatalf("tenant A labels = %#v", labels)
	}

	applied, err := ts.ApplyLabelByMerchant(ctx, tenantA, "shared", "Shared")
	if err != nil {
		t.Fatalf("ApplyLabelByMerchant tenant A: %v", err)
	}
	if applied != 1 {
		t.Fatalf("ApplyLabelByMerchant tenant A affected %d rows, want 1", applied)
	}
	if err := ts.MuteByMerchant(ctx, tenantA, "Shared", "tenant A only"); err != nil {
		t.Fatalf("MuteByMerchant tenant A: %v", err)
	}

	txnA, err := ts.GetTransaction(ctx, tenantA, idA)
	if err != nil {
		t.Fatalf("GetTransaction tenant A: %v", err)
	}
	if len(txnA.Labels) != 1 || txnA.Labels[0] != "shared" || !txnA.Muted {
		t.Fatalf("tenant A transaction after taxonomy/mute = %+v", txnA)
	}
	txnB, err := ts.GetTransaction(ctx, tenantB, idB)
	if err != nil {
		t.Fatalf("GetTransaction tenant B: %v", err)
	}
	if len(txnB.Labels) != 0 || txnB.Muted {
		t.Fatalf("tenant B transaction was affected by tenant A operations: %+v", txnB)
	}
}

func TestTenantTaxonomyIncludesGlobalCategoryAndBucketDefaults(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	tenant := store.Tenant{ID: createRuntimeTestUser(t, ts, "tenant-default-taxonomy@example.com").TenantID}

	if err := ts.SeedMCCCategories(ctx, []string{"Global Default Category"}); err != nil {
		t.Fatalf("SeedMCCCategories: %v", err)
	}
	if err := ts.CreateCategory(ctx, tenant, "Tenant Category", "tenant-owned"); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if err := ts.CreateBucket(ctx, tenant, "Tenant Bucket", "tenant-owned"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	categories, err := ts.ListCategories(ctx, tenant)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if !containsCategory(categories, "Global Default Category") || !containsCategory(categories, "Tenant Category") {
		t.Fatalf("tenant categories should include global defaults and tenant rows, got %#v", categories)
	}

	buckets, err := ts.ListBuckets(ctx, tenant)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if !containsBucket(buckets, "Needs") || !containsBucket(buckets, "Tenant Bucket") {
		t.Fatalf("tenant buckets should include global defaults and tenant rows, got %#v", buckets)
	}
}

func TestCategorySnapshotDoesNotIncludeOtherTenantOverrides(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	tenant := store.Tenant{ID: createRuntimeTestUser(t, ts, "tenant-snapshot@example.com").TenantID}
	if _, err := ts.ApplyCategoryByMerchant(ctx, tenant, "Tenant Private Category", "PrivateMerchant"); err != nil {
		t.Fatalf("ApplyCategoryByMerchant: %v", err)
	}

	resolver, err := ts.LoadCategorySnapshot(ctx)
	if err != nil {
		t.Fatalf("LoadCategorySnapshot: %v", err)
	}
	category, bucket := resolver("PrivateMerchant")
	if category != "" || bucket != "" {
		t.Fatalf("global category snapshot included tenant override: category=%q bucket=%q", category, bucket)
	}
}

func TestTenantScopedRulesAndDiagnostics(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	tenantA := store.Tenant{ID: createRuntimeTestUser(t, ts, "tenant-rules-a@example.com").TenantID}
	tenantB := store.Tenant{ID: createRuntimeTestUser(t, ts, "tenant-rules-b@example.com").TenantID}

	ruleA, err := ts.CreateRule(ctx, tenantA, store.RuleRow{
		Name:            "Same user rule",
		SenderEmail:     "alerts@example.com",
		SubjectContains: "spent",
		AmountRegex:     `INR\s+([0-9.]+)`,
		MerchantRegex:   `at\s+(.+)`,
		SourceLabel:     "Tenant A",
	})
	if err != nil {
		t.Fatalf("CreateRule tenant A: %v", err)
	}
	if _, err := ts.CreateRule(ctx, tenantB, store.RuleRow{
		Name:            "Same user rule",
		SenderEmail:     "alerts@example.com",
		SubjectContains: "spent",
		AmountRegex:     `INR\s+([0-9.]+)`,
		MerchantRegex:   `at\s+(.+)`,
		SourceLabel:     "Tenant B",
	}); err != nil {
		t.Fatalf("CreateRule tenant B with same name: %v", err)
	}
	if _, err := ts.GetRule(ctx, tenantB, ruleA.ID); errors.WhatKind(err) != errors.NotFound {
		t.Fatalf("GetRule tenant B on tenant A rule error = %v, want NotFound kind", err)
	}

	diagnostic := tenantScopeDiagnostic("diag-message", "Same user rule")
	if err := ts.RecordTenantExtractionDiagnostic(ctx, tenantA, diagnostic); err != nil {
		t.Fatalf("RecordExtractionDiagnostic tenant A: %v", err)
	}
	if err := ts.RecordTenantExtractionDiagnostic(ctx, tenantB, diagnostic); err != nil {
		t.Fatalf("RecordExtractionDiagnostic tenant B with same message/rule: %v", err)
	}
	rows, err := ts.ListExtractionDiagnostics(ctx, tenantA, store.DiagnosticFilter{Status: store.DiagnosticStatusAll})
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics tenant A: %v", err)
	}
	if len(rows) != 1 || rows[0].MessageID != "diag-message" {
		t.Fatalf("tenant A diagnostics = %#v", rows)
	}
	if _, err := ts.GetExtractionDiagnostic(ctx, tenantB, rows[0].ID); errors.WhatKind(err) != errors.NotFound {
		t.Fatalf("GetExtractionDiagnostic tenant B on tenant A diagnostic error = %v, want NotFound kind", err)
	}
}

func ptrString(v string) *string {
	return &v
}

func containsCategory(categories []store.Category, name string) bool {
	for _, category := range categories {
		if category.Name == name {
			return true
		}
	}
	return false
}

func containsBucket(buckets []store.Bucket, name string) bool {
	for _, bucket := range buckets {
		if bucket.Name == name {
			return true
		}
	}
	return false
}

func tenantScopeDiagnostic(messageID, ruleName string) api.ExtractionDiagnostic {
	return api.ExtractionDiagnostic{
		Reader:         "gmail",
		MessageID:      messageID,
		Source:         "Tenant Source",
		Sender:         "Alerts",
		SenderEmail:    "alerts@example.com",
		Subject:        "spent",
		EmailBody:      "Spent INR 123 at Example",
		Snippet:        "Spent INR",
		RuleName:       ruleName,
		AmountRegex:    `INR\s+([0-9.]+)`,
		MerchantRegex:  `at\s+(.+)`,
		FailureReasons: []string{"no match"},
	}
}
