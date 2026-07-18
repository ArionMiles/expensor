package httpapi

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCategorizeMerchant_OK(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"merchant":"Netflix","category":"Entertainment","bucket":"Wants"}`
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CategorizeMerchant(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["updated"] != 3 {
		t.Errorf("want updated=3, got %d", resp["updated"])
	}
}

func TestCategorizeMerchant_EmptyMerchant(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"merchant":"","category":"Entertainment","bucket":"Wants"}`
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CategorizeMerchant(w, r)
	assertValidationError(t, w, "merchant", "body", "is required")
}

func TestCategorizeMerchant_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader("not-json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CategorizeMerchant(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCategorizeMerchant_StoreError(t *testing.T) {
	h := newTestHandlers(t, &mockStore{updateErr: stderrors.New("db down")}, &mockDaemon{})
	body := `{"merchant":"Netflix","category":"Entertainment","bucket":"Wants"}`
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CategorizeMerchant(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestUpdateMerchantReason_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/muted-merchants/not-a-uuid", strings.NewReader(`{"reason":"x"}`))
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.UpdateMerchantReason(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateMerchantReason_Success(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/muted-merchants/"+testTransactionID,
		strings.NewReader(`{"reason":"subscription"}`),
	)
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateMerchantReason(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if st.updateMerchantID != testTransactionID || st.updateMerchantReason != "subscription" {
		t.Fatalf("merchant reason call = id=%q reason=%q", st.updateMerchantID, st.updateMerchantReason)
	}
}

func TestDeleteMutedMerchant_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/muted-merchants/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.DeleteMutedMerchant(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestDeleteMutedMerchant_RejectsInvalidUnmuteFlag(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodDelete,
		"/api/muted-merchants/"+testTransactionID+"?unmute=sometimes",
		nil,
	)
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.DeleteMutedMerchant(rr, req)

	assertValidationError(t, rr, "unmute", "query", "must be a boolean")
}
