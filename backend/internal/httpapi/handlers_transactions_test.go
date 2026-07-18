package httpapi

import (
	"context"
	stderrors "errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestListTransactions_Empty(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?page=1&page_size=10")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["total"] != float64(0) {
		t.Errorf("expected total=0, got %v", resp["total"])
	}
	txns, ok := resp["transactions"].([]any)
	if !ok {
		t.Fatalf("expected transactions array, got %#v", resp["transactions"])
	}
	if len(txns) != 0 {
		t.Fatalf("expected empty transactions array, got %d entries", len(txns))
	}
}

func TestListTransactions_NilSliceReturnsEmptyArray(t *testing.T) {
	st := &mockStore{transactions: nil, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?page=1&page_size=10")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	txns, ok := resp["transactions"].([]any)
	if !ok {
		t.Fatalf("expected transactions array, got %#v", resp["transactions"])
	}
	if len(txns) != 0 {
		t.Fatalf("expected empty transactions array, got %d entries", len(txns))
	}
}

func TestListTransactions_WithResults(t *testing.T) {
	now := time.Now()
	st := &mockStore{
		transactions: []store.Transaction{
			{ID: "abc", Amount: 100, Currency: "INR", MerchantInfo: "Amazon", Timestamp: now, Labels: []string{}},
		},
		listResult: store.TransactionListResult{
			Total:       1,
			TotalAmount: 100,
		},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	txns := resp["transactions"].([]any)
	if len(txns) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txns))
	}
	if resp["total_amount"] != float64(100) {
		t.Fatalf("expected total_amount=100, got %v", resp["total_amount"])
	}
	if resp["base_currency"] != "INR" {
		t.Fatalf("expected base_currency=INR, got %v", resp["base_currency"])
	}
}

func TestListTransactions_StoreError(t *testing.T) {
	st := &mockStore{listErr: stderrors.New("db error")}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestListTransactions_RejectsInvalidQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		field   string
		message string
	}{
		{name: "page overflow", query: "page=99999999999999999999999999999", field: "page", message: "must be an integer"},
		{name: "negative page", query: "page=-1", field: "page", message: "must be at least 0"},
		{name: "page size too large", query: "page_size=101", field: "page_size", message: "must be at most 100"},
		{name: "invalid date", query: "date_from=yesterday", field: "date_from", message: "must be an RFC3339 timestamp"},
		{name: "invalid weekday", query: "weekday=7", field: "weekday", message: "must be at most 6"},
		{name: "invalid hour", query: "hour_from=24", field: "hour_from", message: "must be at most 23"},
		{name: "invalid boolean flag", query: "show_muted=true", field: "show_muted", message: "must be 1 when present"},
		{name: "invalid sort", query: "sort_dir=sideways", field: "sort_dir", message: "must be one of: asc, desc"},
		{name: "invalid timezone", query: "tz=Mars/Olympus", field: "tz", message: "must be a valid IANA timezone"},
		{name: "control character", query: "currency=%00bad", field: "currency", message: "must not contain control characters"},
		{name: "invalid search query", query: "q=%00bad", field: "q", message: "must not contain control characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &mockStore{}
			h := newTestHandlers(t, st, &mockDaemon{})
			rr := get(h.ListTransactions, "/api/transactions?"+tt.query)

			assertValidationError(t, rr, tt.field, "query", tt.message)
			if st.listCalls != 0 || st.searchCalls != 0 {
				t.Fatalf("store calls = list:%d search:%d", st.listCalls, st.searchCalls)
			}
		})
	}
}

func TestListTransactions_AcceptsLargePageAndMaximumPageSize(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.ListTransactions, "/api/transactions?page=10001&page_size=100")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Page != 10001 || st.listFilter.PageSize != 100 {
		t.Fatalf("pagination = page:%d page_size:%d", st.listFilter.Page, st.listFilter.PageSize)
	}
}

func TestListTransactions_RejectsOffsetOverflow(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.ListTransactions, fmt.Sprintf(
		"/api/transactions?page=%d&page_size=100",
		math.MaxInt,
	))

	assertValidationError(t, rr, "page", "query", "is too large for page_size")
	if st.listCalls != 0 || st.searchCalls != 0 {
		t.Fatalf("store calls = list:%d search:%d", st.listCalls, st.searchCalls)
	}
}

func TestListTransactions_DefaultsPagination(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if st.listFilter.Page != 1 || st.listFilter.PageSize != 20 {
		t.Fatalf("pagination = page:%d page_size:%d", st.listFilter.Page, st.listFilter.PageSize)
	}
}

func TestListTransactions_ZeroPageDefaultsToFirstPage(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.ListTransactions, "/api/transactions?page=0")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Page != 1 {
		t.Fatalf("page = %d, want 1", st.listFilter.Page)
	}
}

func TestGetTransaction_Found(t *testing.T) {
	txn := &store.Transaction{ID: "11111111-1111-1111-1111-111111111111", Amount: 500, Currency: "INR", Labels: []string{"food"}}
	st := &mockStore{getResult: txn}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/11111111-1111-1111-1111-111111111111", nil)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.GetTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp store.Transaction
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("expected UUID id, got %s", resp.ID)
	}
}

func TestGetTransaction_NotFound(t *testing.T) {
	st := &mockStore{getErr: errors.E(errors.NotFound, errors.User("transaction not found"))}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/22222222-2222-2222-2222-222222222222", nil)
	req.SetPathValue("id", "22222222-2222-2222-2222-222222222222")
	rr := httptest.NewRecorder()
	h.GetTransaction(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	var response ErrorResponse
	decodeJSON(t, rr.Body.String(), &response)
	if response.Message != "transaction not found" {
		t.Fatalf("message = %q", response.Message)
	}
	if response.RequestID == "" {
		t.Fatal("request_id is empty")
	}
}

func TestGetTransaction_InvalidID(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.GetTransaction(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateTransaction_Success(t *testing.T) {
	txn := &store.Transaction{ID: testTransactionID, Description: "Updated", Labels: []string{}}
	st := &mockStore{getResult: txn}
	h := newTestHandlers(t, st, &mockDaemon{})

	body := `{"description":"Updated"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/transactions/11111111-1111-1111-1111-111111111111", strings.NewReader(body))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestUpdateTransaction_MuteStateAndReason(t *testing.T) {
	txn := &store.Transaction{ID: testTransactionID, Muted: true, MuteReason: "duplicate", Labels: []string{}}
	st := &mockStore{getResult: txn}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/transactions/"+testTransactionID,
		strings.NewReader(`{"muted":true,"mute_reason":"duplicate"}`),
	)
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if st.muteTransactionID != testTransactionID || !st.muteTransactionValue || st.muteTransactionReason != "duplicate" {
		t.Fatalf("mute call = id=%q muted=%v reason=%q", st.muteTransactionID, st.muteTransactionValue, st.muteTransactionReason)
	}
}

func TestUpdateTransaction_MuteReasonOnly(t *testing.T) {
	txn := &store.Transaction{ID: testTransactionID, Muted: true, MuteReason: "updated", Labels: []string{}}
	st := &mockStore{getResult: txn}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/transactions/"+testTransactionID,
		strings.NewReader(`{"mute_reason":"updated"}`),
	)
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if st.updateMuteReasonID != testTransactionID || st.updateMuteReasonValue != "updated" {
		t.Fatalf("mute reason call = id=%q reason=%q", st.updateMuteReasonID, st.updateMuteReasonValue)
	}
}

func TestUpdateTransaction_NotFound(t *testing.T) {
	st := &mockStore{updateTxErr: errors.E(errors.NotFound, errors.User("transaction not found"))}
	h := newTestHandlers(t, st, &mockDaemon{})

	body := `{"description":"x"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/transactions/"+testTransactionID, strings.NewReader(body))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	var response ErrorResponse
	decodeJSON(t, rr.Body.String(), &response)
	if response.Message != "transaction not found" {
		t.Fatalf("message = %q", response.Message)
	}
}

func TestUpdateTransaction_FetchUpdatedNotFound(t *testing.T) {
	st := &mockStore{getErr: errStoreNotFound}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/transactions/"+testTransactionID, strings.NewReader(`{}`))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestUpdateTransaction_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/transactions/not-a-uuid", strings.NewReader(`{}`))
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateTransaction_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/transactions/11111111-1111-1111-1111-111111111111", strings.NewReader("not-json"))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateTransaction_ReportsUnknownCategoryAsValidationError(t *testing.T) {
	st := &mockStore{categories: []store.Category{{Name: "Food"}}}
	h := newTestHandlers(t, st, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/transactions/"+testTransactionID,
		strings.NewReader(`{"category":"Unknown"}`),
	)
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()

	h.UpdateTransaction(rr, req)

	assertValidationError(t, rr, "category", "body", "does not exist")
}

func TestAddLabels_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"labels":["food","work"]}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/11111111-1111-1111-1111-111111111111/labels", strings.NewReader(body))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAddLabels_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/11111111-1111-1111-1111-111111111111/labels", strings.NewReader("bad"))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAddLabels_RejectsEmptyLabels(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/transactions/"+testTransactionID+"/labels",
		strings.NewReader(`{"labels":[]}`),
	)
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	assertValidationError(t, rr, "labels", "body", "must be at least 1")
}

func TestAddLabels_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/not-a-uuid/labels", strings.NewReader(`{"labels":["food"]}`))
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAddLabels_BatchSuccess(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	body := `{"labels":["food","work","recurring"]}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/11111111-1111-1111-1111-111111111111/labels", strings.NewReader(body))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAddLabels_StoreError_Returns500(t *testing.T) {
	h := newTestHandlers(t, &mockStore{addLabelsErr: stderrors.New("db error")}, &mockDaemon{})

	body := `{"labels":["food"]}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/11111111-1111-1111-1111-111111111111/labels", strings.NewReader(body))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestRemoveLabel_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/transactions/11111111-1111-1111-1111-111111111111/labels/food", nil)
	req.SetPathValue("id", testTransactionID)
	req.SetPathValue("label", "food")
	rr := httptest.NewRecorder()
	h.RemoveLabel(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestRemoveLabel_NotFound(t *testing.T) {
	h := newTestHandlers(t, &mockStore{removeLblErr: errStoreNotFound}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/transactions/11111111-1111-1111-1111-111111111111/labels/missing", nil)
	req.SetPathValue("id", testTransactionID)
	req.SetPathValue("label", "missing")
	rr := httptest.NewRecorder()
	h.RemoveLabel(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestRemoveLabel_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/transactions/not-a-uuid/labels/food", nil)
	req.SetPathValue("id", "not-a-uuid")
	req.SetPathValue("label", "food")
	rr := httptest.NewRecorder()
	h.RemoveLabel(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestListTransactions_WithSearchQuery(t *testing.T) {
	st := &mockStore{
		searchResult: []store.Transaction{{ID: "x", MerchantInfo: "Zomato", Labels: []string{}}},
		searchListResult: store.TransactionListResult{
			Total:       1,
			TotalAmount: 245,
		},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?q=zomato")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["total"] != float64(1) {
		t.Errorf("expected total=1, got %v", resp["total"])
	}
	if resp["total_amount"] != float64(245) {
		t.Errorf("expected total_amount=245, got %v", resp["total_amount"])
	}
	if resp["base_currency"] != "INR" {
		t.Errorf("expected base_currency=INR, got %v", resp["base_currency"])
	}
}

func TestListTransactions_SearchEmptyArray(t *testing.T) {
	st := &mockStore{
		searchResult:     []store.Transaction{},
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?q=zomato")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	txns, ok := resp["transactions"].([]any)
	if !ok {
		t.Fatalf("expected transactions array, got %#v", resp["transactions"])
	}
	if len(txns) != 0 {
		t.Fatalf("expected empty transactions array, got %d entries", len(txns))
	}
}

func TestListTransactions_SearchNilSliceReturnsEmptyArray(t *testing.T) {
	st := &mockStore{
		searchResult:     nil,
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?q=zomato")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	txns, ok := resp["transactions"].([]any)
	if !ok {
		t.Fatalf("expected transactions array, got %#v", resp["transactions"])
	}
	if len(txns) != 0 {
		t.Fatalf("expected empty transactions array, got %d entries", len(txns))
	}
}

func TestListTransactions_SearchMutedAndIndividualFlags(t *testing.T) {
	st := &mockStore{
		searchResult:     []store.Transaction{},
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.ListTransactions, "/api/transactions?q=zomato&muted_only=1&individual_only=1")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !st.searchFilter.MutedOnly {
		t.Fatal("expected muted_only=1 to set SearchTransactions filter")
	}
	if !st.searchFilter.IndividualOnly {
		t.Fatal("expected individual_only=1 to set SearchTransactions filter")
	}
}

func TestListTransactions_SearchParsesListFilters(t *testing.T) {
	st := &mockStore{
		searchResult:     []store.Transaction{},
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.ListTransactions,
		"/api/transactions?q=instamart&source_type=Credit%20Card&bank=HDFC"+
			"&date_from=2026-04-30T18:30:00.000Z&date_to=2026-05-31T18:29:59.999Z&sort_dir=asc",
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if st.searchFilter.SourceType != "Credit Card" {
		t.Fatalf("source_type = %q, want Credit Card", st.searchFilter.SourceType)
	}
	if st.searchFilter.Bank != "HDFC" {
		t.Fatalf("bank = %q, want HDFC", st.searchFilter.Bank)
	}
	if st.searchFilter.From == nil || st.searchFilter.From.UTC().Format(time.RFC3339Nano) != "2026-04-30T18:30:00Z" {
		t.Fatalf("date_from = %#v", st.searchFilter.From)
	}
	if st.searchFilter.To == nil || st.searchFilter.To.UTC().Format(time.RFC3339Nano) != "2026-05-31T18:29:59.999Z" {
		t.Fatalf("date_to = %#v", st.searchFilter.To)
	}
	if st.searchFilter.SortDir != "asc" {
		t.Fatalf("sort_dir = %q, want asc", st.searchFilter.SortDir)
	}
}

func TestListTransactions_BucketParam(t *testing.T) {
	now := time.Now()
	st := &mockStore{
		transactions: []store.Transaction{
			{
				ID: "w1", Amount: 100, Currency: "INR", MerchantInfo: "Netflix",
				Bucket: "wants", Timestamp: now, Labels: []string{},
			},
		},
		listResult: store.TransactionListResult{Total: 1},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?bucket=wants")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["total"] != float64(1) {
		t.Errorf("expected total=1, got %v", resp["total"])
	}
}

func TestListTransactions_WeekdayHourTimezoneParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.ListTransactions, "/api/transactions?weekday=5&hour_from=9&hour_to=9&tz=Asia/Kolkata")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Weekday == nil || *st.listFilter.Weekday != 5 {
		t.Fatalf("expected weekday filter 5, got %#v", st.listFilter.Weekday)
	}
	if st.listFilter.HourFrom == nil || *st.listFilter.HourFrom != 9 {
		t.Fatalf("expected hour_from 9, got %#v", st.listFilter.HourFrom)
	}
	if st.listFilter.HourTo == nil || *st.listFilter.HourTo != 9 {
		t.Fatalf("expected hour_to 9, got %#v", st.listFilter.HourTo)
	}
	if st.listFilter.Timezone != "Asia/Kolkata" {
		t.Fatalf("expected timezone Asia/Kolkata, got %q", st.listFilter.Timezone)
	}
}

func TestListTransactions_WeekdayHourTimezoneAndDateParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.ListTransactions,
		"/api/transactions?weekday=0&hour_from=23&hour_to=23&tz=Asia/Calcutta&date_from=2026-04-01T00:00:00.000Z&date_to=2026-04-30T23:59:59.000Z",
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Weekday == nil || *st.listFilter.Weekday != 0 {
		t.Fatalf("expected weekday filter 0, got %#v", st.listFilter.Weekday)
	}
	if st.listFilter.HourFrom == nil || *st.listFilter.HourFrom != 23 {
		t.Fatalf("expected hour_from 23, got %#v", st.listFilter.HourFrom)
	}
	if st.listFilter.HourTo == nil || *st.listFilter.HourTo != 23 {
		t.Fatalf("expected hour_to 23, got %#v", st.listFilter.HourTo)
	}
	if st.listFilter.Timezone != "Asia/Calcutta" {
		t.Fatalf("expected timezone Asia/Calcutta, got %q", st.listFilter.Timezone)
	}
	if st.listFilter.From == nil || st.listFilter.From.Format(time.RFC3339) != "2026-04-01T00:00:00Z" {
		t.Fatalf("expected date_from to parse, got %#v", st.listFilter.From)
	}
	if st.listFilter.To == nil || st.listFilter.To.Format(time.RFC3339) != "2026-04-30T23:59:59Z" {
		t.Fatalf("expected date_to to parse, got %#v", st.listFilter.To)
	}
}

func TestListTransactions_ExcludeParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.ListTransactions,
		"/api/transactions?exclude_categories=Shopping,Food%20%26%20Dining&exclude_labels=Top,Recurring&exclude_buckets=Needs,Wants&exclude_sources=HDFC,Amex",
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if want := []string{"Shopping", "Food & Dining"}; !reflect.DeepEqual(want, st.listFilter.ExcludeCategories) {
		t.Fatalf("expected exclude_categories %v, got %v", want, st.listFilter.ExcludeCategories)
	}
	if want := []string{"Top", "Recurring"}; !reflect.DeepEqual(want, st.listFilter.ExcludeLabels) {
		t.Fatalf("expected exclude_labels %v, got %v", want, st.listFilter.ExcludeLabels)
	}
	if want := []string{"Needs", "Wants"}; !reflect.DeepEqual(want, st.listFilter.ExcludeBuckets) {
		t.Fatalf("expected exclude_buckets %v, got %v", want, st.listFilter.ExcludeBuckets)
	}
	if want := []string{"HDFC", "Amex"}; !reflect.DeepEqual(want, st.listFilter.ExcludeSources) {
		t.Fatalf("expected exclude_sources %v, got %v", want, st.listFilter.ExcludeSources)
	}
}

func TestListTransactions_StructuredSourceParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.ListTransactions,
		"/api/transactions?source_type=Credit%20Card&bank=HDFC&exclude_source_types=UPI,NetBanking&exclude_banks=ICICI,SBI",
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.SourceType != "Credit Card" {
		t.Fatalf("source_type = %q, want Credit Card", st.listFilter.SourceType)
	}
	if st.listFilter.Bank != "HDFC" {
		t.Fatalf("bank = %q, want HDFC", st.listFilter.Bank)
	}
	if want := []string{"UPI", "NetBanking"}; !reflect.DeepEqual(want, st.listFilter.ExcludeSourceTypes) {
		t.Fatalf("exclude_source_types = %#v, want %#v", st.listFilter.ExcludeSourceTypes, want)
	}
	if want := []string{"ICICI", "SBI"}; !reflect.DeepEqual(want, st.listFilter.ExcludeBanks) {
		t.Fatalf("exclude_banks = %#v, want %#v", st.listFilter.ExcludeBanks, want)
	}
}

func TestListTransactions_MissingTaxonomyParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.ListTransactions,
		"/api/transactions?category_missing=1&bucket_missing=1&label_missing=1",
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !st.listFilter.CategoryMissing {
		t.Fatal("expected category_missing=1 to set ListFilter.CategoryMissing")
	}
	if !st.listFilter.BucketMissing {
		t.Fatal("expected bucket_missing=1 to set ListFilter.BucketMissing")
	}
	if !st.listFilter.LabelMissing {
		t.Fatal("expected label_missing=1 to set ListFilter.LabelMissing")
	}
}

func TestListTransactions_MissingTimezoneFallsBackToAppTimezone(t *testing.T) {
	st := &mockStore{
		transactions: []store.Transaction{},
		listResult:   store.TransactionListResult{Total: 0},
		appConfigByTenant: map[string]map[string]string{
			"tenant-a": {"app.timezone": "Asia/Kolkata"},
		},
	}
	h := newTestHandlers(t, st, &mockDaemon{})

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/transactions?weekday=5&hour_from=9&hour_to=9", nil)
	rr := httptest.NewRecorder()
	h.ListTransactions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Timezone != "Asia/Kolkata" {
		t.Fatalf("expected fallback timezone Asia/Kolkata, got %q", st.listFilter.Timezone)
	}
}
