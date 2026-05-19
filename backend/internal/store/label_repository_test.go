package store_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestLabelRepositoryCRUD(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	repo := store.NewLabelRepository(ts.PoolForTest(), slog.Default())

	if err := repo.Create(ctx, "custom-test", "#38bdf8"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	labels, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !containsLabel(labels, "custom-test", "#38bdf8") {
		t.Fatalf("expected custom-test label after create, got %#v", labels)
	}

	if err := repo.Update(ctx, "custom-test", "#f97316"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	labels, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List after update: %v", err)
	}
	if !containsLabel(labels, "custom-test", "#f97316") {
		t.Fatalf("expected custom-test label color update, got %#v", labels)
	}

	if err := repo.Delete(ctx, "custom-test"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	labels, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if containsLabel(labels, "custom-test", "#f97316") {
		t.Fatalf("expected custom-test label to be deleted, got %#v", labels)
	}
}

func TestStoreDoesNotSeedOutOfBoxLabels(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	labels, err := ts.ListLabels(ctx)
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

	preserveID := seedTransaction(t, ctx, ts.Store, "delete-label-preserve", 300, "INR", "Blinkit", "Food")
	removeID := seedTransaction(t, ctx, ts.Store, "delete-label-remove", 400, "INR", "Instamart", "Food")

	if err := ts.CreateLabel(ctx, "cleanup-test-preserve", "#6366f1"); err != nil {
		t.Fatalf("CreateLabel preserve: %v", err)
	}
	if err := ts.AddLabel(ctx, preserveID, "cleanup-test-preserve"); err != nil {
		t.Fatalf("AddLabel preserve: %v", err)
	}
	if err := ts.DeleteLabel(ctx, "cleanup-test-preserve", false); err != nil {
		t.Fatalf("DeleteLabel preserve: %v", err)
	}
	preserveTxn, err := ts.GetTransaction(ctx, preserveID)
	if err != nil {
		t.Fatalf("GetTransaction preserve: %v", err)
	}
	if !containsString(preserveTxn.Labels, "cleanup-test-preserve") {
		t.Fatalf("expected transaction label to remain when cleanup is false, got %v", preserveTxn.Labels)
	}

	if err := ts.CreateLabel(ctx, "cleanup-test-remove", "#6366f1"); err != nil {
		t.Fatalf("CreateLabel remove: %v", err)
	}
	if err := ts.AddLabel(ctx, removeID, "cleanup-test-remove"); err != nil {
		t.Fatalf("AddLabel remove: %v", err)
	}
	if err := ts.DeleteLabel(ctx, "cleanup-test-remove", true); err != nil {
		t.Fatalf("DeleteLabel remove: %v", err)
	}
	removeTxn, err := ts.GetTransaction(ctx, removeID)
	if err != nil {
		t.Fatalf("GetTransaction remove: %v", err)
	}
	if containsString(removeTxn.Labels, "cleanup-test-remove") {
		t.Fatalf("expected transaction label to be removed when cleanup is true, got %v", removeTxn.Labels)
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
