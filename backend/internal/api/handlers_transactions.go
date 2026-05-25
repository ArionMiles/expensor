package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

// --- transactions ---

// ListTransactions handles GET /api/transactions.
// @Summary List transactions
// @Tags Transactions
// @Produce json
// @Param page query int false "1-based page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param merchant query string false "Merchant filter"
// @Param category query string false "Category filter"
// @Param category_missing query int false "Only transactions without a category when set to 1" Enums(1)
// @Param exclude_categories query string false "Comma-separated categories to exclude"
// @Param currency query string false "Currency filter"
// @Param source query string false "Source filter"
// @Param exclude_sources query string false "Comma-separated sources to exclude"
// @Param label query string false "Label filter"
// @Param label_missing query int false "Only transactions without labels when set to 1" Enums(1)
// @Param exclude_labels query string false "Comma-separated labels to exclude"
// @Param bucket query string false "Bucket filter"
// @Param bucket_missing query int false "Only transactions without a bucket when set to 1" Enums(1)
// @Param exclude_buckets query string false "Comma-separated buckets to exclude"
// @Param date_from query string false "RFC3339 start timestamp"
// @Param date_to query string false "RFC3339 end timestamp"
// @Param show_muted query int false "Include muted transactions when set to 1" Enums(1)
// @Param muted_only query int false "Return only muted transactions when set to 1" Enums(1)
// @Param weekday query int false "PostgreSQL DOW weekday filter (0=Sunday...6=Saturday)" Enums(0,1,2,3,4,5,6)
// @Param hour_from query int false "Minimum hour filter (0-23)"
// @Param hour_to query int false "Maximum hour filter (0-23)"
// @Param tz query string false "IANA timezone used for weekday/hour filters"
// @Param sort_dir query string false "Sort direction" Enums(asc,desc)
// @Success 200 {object} TransactionsListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /transactions [get]
func (h *Handlers) ListTransactions(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	if invalidKey, ok := firstInvalidFilterParam(
		r,
		"merchant",
		"category",
		"currency",
		"source",
		"source_type",
		"bank",
		"label",
		"bucket",
		"exclude_categories",
		"exclude_sources",
		"exclude_source_types",
		"exclude_banks",
		"exclude_labels",
		"exclude_buckets",
	); ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid %s filter", invalidKey))
		return
	}

	f := store.ListFilter{
		Page:               queryInt(r, "page", 1),
		PageSize:           queryInt(r, "page_size", 20),
		Merchant:           r.URL.Query().Get("merchant"),
		Category:           r.URL.Query().Get("category"),
		CategoryMissing:    r.URL.Query().Get("category_missing") == "1",
		ExcludeCategories:  queryCSV(r, "exclude_categories"),
		Currency:           r.URL.Query().Get("currency"),
		Source:             r.URL.Query().Get("source"),
		ExcludeSources:     queryCSV(r, "exclude_sources"),
		SourceType:         r.URL.Query().Get("source_type"),
		ExcludeSourceTypes: queryCSV(r, "exclude_source_types"),
		Bank:               r.URL.Query().Get("bank"),
		ExcludeBanks:       queryCSV(r, "exclude_banks"),
		Label:              r.URL.Query().Get("label"),
		ExcludeLabels:      queryCSV(r, "exclude_labels"),
		Bucket:             r.URL.Query().Get("bucket"),
		BucketMissing:      r.URL.Query().Get("bucket_missing") == "1",
		ExcludeBuckets:     queryCSV(r, "exclude_buckets"),
		LabelMissing:       r.URL.Query().Get("label_missing") == "1",
		ShowMuted:          r.URL.Query().Get("show_muted") == "1",
		MutedOnly:          r.URL.Query().Get("muted_only") == "1",
		IndividualOnly:     r.URL.Query().Get("individual_only") == "1",
		Weekday:            queryWeekday(r, "weekday"),
		HourFrom:           queryHour(r, "hour_from"),
		HourTo:             queryHour(r, "hour_to"),
		Timezone:           h.resolveTimezone(r.Context(), r.URL.Query().Get("tz")),
	}
	if v := r.URL.Query().Get("date_from"); v != "" {
		// JavaScript toISOString() includes milliseconds (RFC3339Nano); try that first.
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			f.From = &t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := r.URL.Query().Get("date_to"); v != "" {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			f.To = &t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}
	f.SortBy = r.URL.Query().Get("sort_by")
	f.SortDir = r.URL.Query().Get("sort_dir")

	txns, result, err := h.store.ListTransactions(r.Context(), f)
	if err != nil {
		h.logger.Error("list transactions", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list transactions")
		return
	}
	if txns == nil {
		txns = []store.Transaction{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"transactions":  txns,
		"total":         result.Total,
		"total_amount":  result.TotalAmount,
		"base_currency": h.currentBaseCurrency(r.Context()),
		"page":          f.Page,
		"page_size":     f.PageSize,
	})
}

func queryCSV(r *http.Request, key string) []string {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func firstInvalidFilterParam(r *http.Request, keys ...string) (string, bool) {
	for _, key := range keys {
		if containsControlChars(r.URL.Query().Get(key)) {
			return key, true
		}
	}
	return "", false
}

func containsControlChars(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// GetTransaction handles GET /api/transactions/{id}.
// @Summary Get a transaction
// @Tags Transactions
// @Produce json
// @Param id path string true "Transaction ID"
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /transactions/{id} [get]
func (h *Handlers) GetTransaction(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}
	txn, err := h.store.GetTransaction(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		h.logger.Error("get transaction", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch transaction")
		return
	}
	writeJSON(w, http.StatusOK, txn)
}

// validateCategory checks that the given category name exists in the store.
// Returns false and writes an error response if validation fails.
func (h *Handlers) validateCategory(w http.ResponseWriter, r *http.Request, name string) bool {
	cats, err := h.store.ListCategories(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate category")
		return false
	}
	for _, c := range cats {
		if c.Name == name {
			return true
		}
	}
	writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("category %q does not exist", name))
	return false
}

// validateBucket checks that the given bucket name exists in the store.
// Returns false and writes an error response if validation fails.
func (h *Handlers) validateBucket(w http.ResponseWriter, r *http.Request, name string) bool {
	bkts, err := h.store.ListBuckets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate bucket")
		return false
	}
	for _, b := range bkts {
		if b.Name == name {
			return true
		}
	}
	writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("bucket %q does not exist", name))
	return false
}

// UpdateTransaction handles PUT /api/transactions/{id}.
// Body: {"description": "...", "category": "...", "bucket": "..."}
// All fields are optional; only non-nil fields are written.
// @Summary Update a transaction
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID"
// @Param request body TransactionUpdateRequest true "Transaction update payload"
// @Success 200 {object} TransactionResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /transactions/{id} [put]
func (h *Handlers) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Description *string `json:"description"`
		Category    *string `json:"category"`
		Bucket      *string `json:"bucket"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	if body.Category != nil && *body.Category != "" && !h.validateCategory(w, r, *body.Category) {
		return
	}
	if body.Bucket != nil && *body.Bucket != "" && !h.validateBucket(w, r, *body.Bucket) {
		return
	}

	u := store.TransactionUpdate{
		Description: body.Description,
		Category:    body.Category,
		Bucket:      body.Bucket,
	}
	if err := h.store.UpdateTransaction(r.Context(), id, u); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		h.logger.Error("update transaction", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update transaction")
		return
	}

	tx, err := h.store.GetTransaction(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch updated transaction")
		return
	}
	writeJSON(w, http.StatusOK, tx)
}

// --- muted transactions ---

// MuteTransaction handles PUT /api/transactions/{id}/mute.
// Body: {"muted": true|false}
// @Summary Mute or unmute a transaction
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID"
// @Param request body MuteTransactionRequest true "Mute payload"
// @Success 200 {object} MuteTransactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /transactions/{id}/mute [put]
func (h *Handlers) MuteTransaction(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Muted  bool   `json:"muted"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.store.MuteTransaction(r.Context(), id, body.Muted, body.Reason); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		h.logger.Error("mute transaction", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update transaction")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"muted": body.Muted, "reason": body.Reason})
}

// UpdateMuteReason handles PUT /api/transactions/{id}/mute-reason.
// Body: {"reason": "optional text"}
//
// @Summary Update a transaction mute reason
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID"
// @Param request body UpdateMuteReasonRequest true "Mute reason payload"
// @Success 200 {object} MuteReasonResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /transactions/{id}/mute-reason [put]
//
//nolint:dupl // structurally similar to UpdateMerchantReason but calls a different store method
func (h *Handlers) UpdateMuteReason(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.store.UpdateMuteReason(r.Context(), id, body.Reason); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found or not muted")
			return
		}
		h.logger.Error("update mute reason", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update reason")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reason": body.Reason})
}

// ListMutedMerchants handles GET /api/muted-merchants.
func (h *Handlers) ListMutedMerchants(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	merchants, err := h.store.GetMutedMerchantsWithCount(r.Context())
	if err != nil {
		h.logger.Error("list muted merchants", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list muted merchants")
		return
	}
	writeJSON(w, http.StatusOK, merchants)
}

// MuteByMerchant handles POST /api/muted-merchants.
// Body: {"pattern": "MERCHANT NAME", "reason": "optional"}
func (h *Handlers) MuteByMerchant(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Pattern string `json:"pattern"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Pattern == "" {
		writeError(w, http.StatusBadRequest, "request body must be JSON with a non-empty \"pattern\" field")
		return
	}
	if err := h.store.MuteByMerchant(r.Context(), body.Pattern, body.Reason); err != nil {
		h.logger.Error("mute by merchant", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mute merchant")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"pattern": body.Pattern})
}

// UpdateMerchantReason handles PUT /api/muted-merchants/{id}/reason.
// Body: {"reason": "optional text"}
//
//nolint:dupl // structurally similar to UpdateMuteReason but calls a different store method
func (h *Handlers) UpdateMerchantReason(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.store.UpdateMerchantReason(r.Context(), id, body.Reason); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "muted merchant not found")
			return
		}
		h.logger.Error("update merchant reason", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update reason")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reason": body.Reason})
}

// DeleteMutedMerchant handles DELETE /api/muted-merchants/{id}.
// Optional query param: ?unmute=true — atomically deletes the rule and
// sets muted=false on all existing transactions that matched the pattern.
func (h *Handlers) DeleteMutedMerchant(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")

	var err error
	if r.URL.Query().Get("unmute") == queryValueTrue {
		err = h.store.DeleteMutedMerchantAndUnmute(r.Context(), id)
	} else {
		err = h.store.DeleteMutedMerchant(r.Context(), id)
	}

	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "muted merchant not found")
			return
		}
		h.logger.Error("delete muted merchant", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete muted merchant")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CategorizeMerchant handles POST /api/merchants/categorize.
// Body: {"merchant": "Name", "category": "Cat", "bucket": "Bucket"}
// Response: {"updated": N}
func (h *Handlers) CategorizeMerchant(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Merchant string `json:"merchant"`
		Category string `json:"category"`
		Bucket   string `json:"bucket"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}
	if body.Merchant == "" {
		writeError(w, http.StatusBadRequest, "\"merchant\" must not be empty")
		return
	}
	n, err := h.store.CategorizeMerchant(r.Context(), body.Merchant, body.Category, body.Bucket)
	if err != nil {
		h.logger.Error("categorize merchant", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to categorize merchant")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"updated": n})
}

// SearchTransactions handles GET /api/transactions/search?q=...
// @Summary Search transactions
// @Tags Transactions
// @Produce json
// @Param q query string true "Search query"
// @Param page query int false "1-based page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param show_muted query int false "Include muted transactions when set to 1" Enums(1)
// @Success 200 {object} TransactionsSearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /transactions/search [get]
func (h *Handlers) SearchTransactions(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	q := r.URL.Query().Get("q")
	if containsControlChars(q) {
		writeError(w, http.StatusBadRequest, "invalid q filter")
		return
	}
	f := store.ListFilter{
		Page:           queryInt(r, "page", 1),
		PageSize:       queryInt(r, "page_size", 20),
		ShowMuted:      r.URL.Query().Get("show_muted") == "1",
		MutedOnly:      r.URL.Query().Get("muted_only") == "1",
		IndividualOnly: r.URL.Query().Get("individual_only") == "1",
	}

	txns, result, err := h.store.SearchTransactions(r.Context(), q, f)
	if err != nil {
		h.logger.Error("search transactions", "error", err)
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	if txns == nil {
		txns = []store.Transaction{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"transactions":  txns,
		"total":         result.Total,
		"total_amount":  result.TotalAmount,
		"base_currency": h.currentBaseCurrency(r.Context()),
		"page":          f.Page,
		"page_size":     f.PageSize,
		"query":         q,
	})
}

// GetFacets handles GET /api/transactions/facets.
// Returns distinct values for source, category, currency, and label — used to
// populate filter dropdowns in the UI.
// @Summary Get transaction facets
// @Tags Transactions
// @Produce json
// @Success 200 {object} FacetsResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /transactions/facets [get]
func (h *Handlers) GetFacets(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	facets, err := h.store.GetFacets(r.Context())
	if err != nil {
		h.logger.Error("get facets", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch facets")
		return
	}
	writeJSON(w, http.StatusOK, facets)
}
