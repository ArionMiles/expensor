package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestListLabels_Success(t *testing.T) {
	ms := &mockStore{labels: []store.Label{{Name: "food", Color: "#f59e0b"}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.ListLabels, "/api/config/labels")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []store.Label
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 || resp[0].Name != "food" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestCreateLabel_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := strings.NewReader(`{"name":"groceries","color":"#aabbcc"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/config/labels", body)
	rr := httptest.NewRecorder()
	h.CreateLabel(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["name"] != "groceries" {
		t.Errorf("expected name=groceries, got %q", resp["name"])
	}
}

func TestCreateLabel_RejectsInvalidColor(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/config/labels",
		strings.NewReader(`{"name":"groceries","color":"blue"}`),
	)
	rr := httptest.NewRecorder()
	h.CreateLabel(rr, req)

	assertValidationError(t, rr, "color", "body", "must be a valid hexadecimal color")
}

func TestDeleteLabel_NotFound(t *testing.T) {
	ms := &mockStore{labelsErr: errStoreNotFound}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/labels/missing", nil)
	req.SetPathValue("name", "missing")
	rr := httptest.NewRecorder()
	// DeleteLabel returns the labelsErr directly; since it's NotFound kind the handler
	// logs and returns 500 (DeleteLabel has no NotFound kind branch in the handler).
	// The store just returns the error; handler writes 500. Verify non-204.
	h.DeleteLabel(rr, req)
	if rr.Code == http.StatusNoContent {
		t.Fatalf("expected non-204 on error, got 204")
	}
}

func TestDeleteLabel_RemoveFromTransactionsOption(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})

	body := strings.NewReader(`{"remove_from_transactions":true}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/labels/food", body)
	req.SetPathValue("name", "food")
	rr := httptest.NewRecorder()

	h.DeleteLabel(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteLabelCleanup {
		t.Fatal("expected delete label to request transaction label cleanup")
	}
}

func TestDeleteLabel_RemoveFromTransactionsQueryOption(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodDelete,
		"/api/config/labels/food?remove_from_transactions=true",
		nil,
	)
	req.SetPathValue("name", "food")
	rr := httptest.NewRecorder()

	h.DeleteLabel(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteLabelCleanup {
		t.Fatal("expected delete label query parameter to request transaction label cleanup")
	}
}

func TestDeleteLabel_RejectsInvalidCleanupFlag(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodDelete,
		"/api/config/labels/food?remove_from_transactions=sometimes",
		nil,
	)
	req.SetPathValue("name", "food")
	rr := httptest.NewRecorder()
	h.DeleteLabel(rr, req)

	assertValidationError(t, rr, "remove_from_transactions", "query", "must be a boolean")
}

func TestListCategories_Success(t *testing.T) {
	ms := &mockStore{categories: []store.Category{{Name: "food & dining", IsDefault: true}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.ListCategories, "/api/config/categories")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []store.Category
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 || resp[0].Name != "food & dining" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestDeleteCategory_DefaultRejected(t *testing.T) {
	ms := &mockStore{catsErr: errors.E(
		errors.Conflict,
		errors.User("The default category cannot be deleted."),
		"cannot delete default category \"food\"",
	)}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/categories/food", nil)
	req.SetPathValue("name", "food")
	rr := httptest.NewRecorder()
	h.DeleteCategory(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var response ErrorResponse
	decodeJSON(t, rr.Body.String(), &response)
	if response.Message != "The default category cannot be deleted." {
		t.Fatalf("message = %q", response.Message)
	}
}

func TestGetCategoryMappings_Success(t *testing.T) {
	ms := &mockStore{categoryMappings: map[string][]string{"Food": {"swiggy", "zomato"}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.GetCategoryMappings, "/api/config/categories/mappings")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string][]string
	decodeJSON(t, rr.Body.String(), &resp)
	if !reflect.DeepEqual(resp["Food"], []string{"swiggy", "zomato"}) {
		t.Fatalf("unexpected mappings: %#v", resp)
	}
}

func TestApplyCategoryByMerchant_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPut,
		"/api/config/categories/Food/merchant-mappings/swiggy",
		nil,
	)
	req.SetPathValue("name", "Food")
	req.SetPathValue("pattern", "swiggy")
	rr := httptest.NewRecorder()

	h.ApplyCategoryByMerchant(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]int
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["applied"] != 2 {
		t.Fatalf("expected applied=2, got %#v", resp)
	}
}

func TestDeleteCategory_RemoveFromTransactionsOption(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := strings.NewReader(`{"remove_from_transactions":true}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/categories/Food", body)
	req.SetPathValue("name", "Food")
	rr := httptest.NewRecorder()

	h.DeleteCategory(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteCategoryCleanup {
		t.Fatal("expected delete category to request transaction cleanup")
	}
}

func TestExportCategories_IncludesMerchants(t *testing.T) {
	ms := &mockStore{
		categories:       []store.Category{{Name: "Food"}},
		categoryMappings: map[string][]string{"Food": {"swiggy"}},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.ExportCategories, "/api/config/categories/export")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp[0]["name"] != "Food" {
		t.Fatalf("unexpected export: %#v", resp)
	}
	merchants, ok := resp[0]["merchants"].([]any)
	if !ok || len(merchants) != 1 || merchants[0] != "swiggy" {
		t.Fatalf("expected merchants in export, got %#v", resp)
	}
}

func TestGetBucketMappings_Success(t *testing.T) {
	ms := &mockStore{bucketMappings: map[string][]string{"Needs": {"rent"}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.GetBucketMappings, "/api/config/buckets/mappings")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string][]string
	decodeJSON(t, rr.Body.String(), &resp)
	if !reflect.DeepEqual(resp["Needs"], []string{"rent"}) {
		t.Fatalf("unexpected mappings: %#v", resp)
	}
}

func TestApplyBucketByMerchant_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPut,
		"/api/config/buckets/Needs/merchant-mappings/rent",
		nil,
	)
	req.SetPathValue("name", "Needs")
	req.SetPathValue("pattern", "rent")
	rr := httptest.NewRecorder()

	h.ApplyBucketByMerchant(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]int
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["applied"] != 3 {
		t.Fatalf("expected applied=3, got %#v", resp)
	}
}

func TestDeleteBucket_RemoveFromTransactionsOption(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := strings.NewReader(`{"remove_from_transactions":true}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/buckets/Needs", body)
	req.SetPathValue("name", "Needs")
	rr := httptest.NewRecorder()

	h.DeleteBucket(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteBucketCleanup {
		t.Fatal("expected delete bucket to request transaction cleanup")
	}
}

func TestExportBuckets_IncludesMerchants(t *testing.T) {
	ms := &mockStore{
		buckets:        []store.Bucket{{Name: "Needs"}},
		bucketMappings: map[string][]string{"Needs": {"rent"}},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.ExportBuckets, "/api/config/buckets/export")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp[0]["name"] != "Needs" {
		t.Fatalf("unexpected export: %#v", resp)
	}
	merchants, ok := resp[0]["merchants"].([]any)
	if !ok || len(merchants) != 1 || merchants[0] != "rent" {
		t.Fatalf("expected merchants in export, got %#v", resp)
	}
}
