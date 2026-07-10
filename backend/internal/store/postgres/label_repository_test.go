package postgres_test

import (
	"context"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestLabelRepositoryCRUD(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.CreateLabel(ctx, store.Tenant{}, "custom-test", "#38bdf8"); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}
	labels, err := ts.ListLabels(ctx, store.Tenant{})
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	if !containsLabel(labels, "custom-test", "#38bdf8") {
		t.Fatalf("expected custom-test label after create, got %#v", labels)
	}

	if err := ts.UpdateLabel(ctx, store.Tenant{}, "custom-test", "#f97316"); err != nil {
		t.Fatalf("UpdateLabel: %v", err)
	}
	labels, err = ts.ListLabels(ctx, store.Tenant{})
	if err != nil {
		t.Fatalf("ListLabels after update: %v", err)
	}
	if !containsLabel(labels, "custom-test", "#f97316") {
		t.Fatalf("expected custom-test label color update, got %#v", labels)
	}

	if err := ts.DeleteLabel(ctx, store.Tenant{}, "custom-test", false); err != nil {
		t.Fatalf("DeleteLabel: %v", err)
	}
	labels, err = ts.ListLabels(ctx, store.Tenant{})
	if err != nil {
		t.Fatalf("ListLabels after delete: %v", err)
	}
	if containsLabel(labels, "custom-test", "#f97316") {
		t.Fatalf("expected custom-test label to be deleted, got %#v", labels)
	}
}

func TestStoreDoesNotSeedOutOfBoxLabels(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	labels, err := ts.ListLabels(ctx, store.Tenant{})
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("expected no out-of-the-box labels, got %#v", labels)
	}
}

func TestDeleteLabelCanRemoveExistingTransactionLabels(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	preserveID := seedTransaction(ctx, t, ts.Store, store.InsertParams{
		MessageID: "delete-label-preserve", Amount: 300, Currency: "INR", MerchantInfo: "Blinkit", Category: "Food",
	})

	removeID := seedTransaction(ctx, t, ts.Store, store.InsertParams{
		MessageID: "delete-label-remove", Amount: 400, Currency: "INR", MerchantInfo: "Instamart", Category: "Food",
	})

	if err := ts.CreateLabel(ctx, store.Tenant{}, "cleanup-test-preserve", "#6366f1"); err != nil {
		t.Fatalf("CreateLabel preserve: %v", err)
	}
	if err := ts.AddLabel(ctx, store.Tenant{}, preserveID, "cleanup-test-preserve"); err != nil {
		t.Fatalf("AddLabel preserve: %v", err)
	}
	if err := ts.DeleteLabel(ctx, store.Tenant{}, "cleanup-test-preserve", false); err != nil {
		t.Fatalf("DeleteLabel preserve: %v", err)
	}
	preserveTxn, err := ts.GetTransaction(ctx, store.Tenant{}, preserveID)
	if err != nil {
		t.Fatalf("GetTransaction preserve: %v", err)
	}
	if !containsString(preserveTxn.Labels, "cleanup-test-preserve") {
		t.Fatalf("expected transaction label to remain when cleanup is false, got %v", preserveTxn.Labels)
	}

	if err := ts.CreateLabel(ctx, store.Tenant{}, "cleanup-test-remove", "#6366f1"); err != nil {
		t.Fatalf("CreateLabel remove: %v", err)
	}
	if err := ts.AddLabel(ctx, store.Tenant{}, removeID, "cleanup-test-remove"); err != nil {
		t.Fatalf("AddLabel remove: %v", err)
	}
	if err := ts.DeleteLabel(ctx, store.Tenant{}, "cleanup-test-remove", true); err != nil {
		t.Fatalf("DeleteLabel remove: %v", err)
	}
	removeTxn, err := ts.GetTransaction(ctx, store.Tenant{}, removeID)
	if err != nil {
		t.Fatalf("GetTransaction remove: %v", err)
	}
	if containsString(removeTxn.Labels, "cleanup-test-remove") {
		t.Fatalf("expected transaction label to be removed when cleanup is true, got %v", removeTxn.Labels)
	}
}

func TestDeleteLabelRemovesMerchantMappings(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.CreateLabel(ctx, store.Tenant{}, "merchant-cleanup", "#6366f1"); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}
	if _, err := ts.ApplyLabelByMerchant(ctx, store.Tenant{}, "merchant-cleanup", "Netflix"); err != nil {
		t.Fatalf("ApplyLabelByMerchant: %v", err)
	}

	if err := ts.DeleteLabel(ctx, store.Tenant{}, "merchant-cleanup", false); err != nil {
		t.Fatalf("DeleteLabel: %v", err)
	}

	mappings, err := ts.GetLabelMappings(ctx, store.Tenant{})
	if err != nil {
		t.Fatalf("GetLabelMappings: %v", err)
	}
	if got := mappings["merchant-cleanup"]; len(got) != 0 {
		t.Fatalf("deleted label still has merchant mappings: %v", got)
	}
}

func containsLabel(labels []store.Label, name, color string) bool {
	for _, label := range labels {
		if label.Name == name && label.Color == color {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
