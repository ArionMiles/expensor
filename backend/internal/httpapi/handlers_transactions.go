package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

// --- transactions ---

// ListTransactions handles GET /api/transactions.
// @Summary List transactions
// @Tags Transactions
// @Produce json
// @Param page query int false "1-based page number; 0 defaults to 1" default(1) minimum(0)
// @Param page_size query int false "Page size" default(20) minimum(1) maximum(100)
// @Param merchant query string false "Merchant filter"
// @Param category query string false "Category filter"
// @Param category_missing query int false "Only transactions without a category when set to 1" Enums(1)
// @Param exclude_categories query string false "Comma-separated categories to exclude"
// @Param currency query string false "Currency filter"
// @Param source query string false "Source filter"
// @Param exclude_sources query string false "Comma-separated sources to exclude"
// @Param source_type query string false "Source type filter"
// @Param exclude_source_types query string false "Comma-separated source types to exclude"
// @Param bank query string false "Bank filter"
// @Param exclude_banks query string false "Comma-separated banks to exclude"
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
// @Param individual_only query int false "Return only individually muted transactions when set to 1" Enums(1)
// @Param weekday query int false "PostgreSQL DOW weekday filter (0=Sunday...6=Saturday)" Enums(0,1,2,3,4,5,6)
// @Param hour_from query int false "Minimum hour filter (0-23)" minimum(0) maximum(23)
// @Param hour_to query int false "Maximum hour filter (0-23)" minimum(0) maximum(23)
// @Param tz query string false "IANA timezone used for weekday/hour filters"
// @Param q query string false "Free-text search over merchant and description"
// @Param sort_by query string false "Sort field" Enums(timestamp)
// @Param sort_dir query string false "Sort direction" Enums(asc,desc)
// @Success 200 {object} TransactionsListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /transactions [get]
func (h *Handlers) ListTransactions(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeAndValidateQuery[transactionListQuery](h, w, r)
	if !ok {
		return
	}

	f := query.listFilter(h.resolveTimezone(r.Context(), query.Timezone))
	q := strings.TrimSpace(query.Query)

	var (
		txns   []store.Transaction
		result store.TransactionListResult
		err    error
	)
	if q == "" {
		txns, result, err = h.transactionStore.ListTransactions(r.Context(), f)
	} else {
		txns, result, err = h.transactionStore.SearchTransactions(r.Context(), q, f)
	}
	if err != nil {
		h.logger.Error("query transactions", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to query transactions")
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

func (query transactionListQuery) listFilter(timezone string) store.ListFilter {
	page := query.Page
	if page == 0 {
		page = 1
	}
	pageSize := 20
	if query.PageSize != nil {
		pageSize = *query.PageSize
	}
	return store.ListFilter{
		Page:               page,
		PageSize:           pageSize,
		Merchant:           query.Merchant,
		Category:           query.Category,
		CategoryMissing:    query.CategoryMissing == "1",
		ExcludeCategories:  queryCSV(query.ExcludeCategories),
		Currency:           query.Currency,
		Source:             query.Source,
		ExcludeSources:     queryCSV(query.ExcludeSources),
		SourceType:         query.SourceType,
		ExcludeSourceTypes: queryCSV(query.ExcludeSourceTypes),
		Bank:               query.Bank,
		ExcludeBanks:       queryCSV(query.ExcludeBanks),
		Label:              query.Label,
		ExcludeLabels:      queryCSV(query.ExcludeLabels),
		Bucket:             query.Bucket,
		BucketMissing:      query.BucketMissing == "1",
		ExcludeBuckets:     queryCSV(query.ExcludeBuckets),
		LabelMissing:       query.LabelMissing == "1",
		ShowMuted:          query.ShowMuted == "1",
		MutedOnly:          query.MutedOnly == "1",
		IndividualOnly:     query.IndividualOnly == "1",
		Weekday:            query.Weekday,
		HourFrom:           query.HourFrom,
		HourTo:             query.HourTo,
		Timezone:           timezone,
		From:               query.DateFrom,
		To:                 query.DateTo,
		SortBy:             query.SortBy,
		SortDir:            query.SortDir,
	}
}

func queryCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
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

// GetTransaction handles GET /api/transactions/{id}.
// @Summary Get a transaction
// @Tags Transactions
// @Produce json
// @Param id path string true "Transaction ID" format(uuid) example(00000000-0000-0000-0000-000000000001)
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /transactions/{id} [get]
func (h *Handlers) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidPathValue(w, r, "id", "transaction")
	if !ok {
		return
	}
	txn, err := h.transactionStore.GetTransaction(r.Context(), id)
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
	cats, err := h.taxonomyStore.ListCategories(r.Context())
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
	bkts, err := h.taxonomyStore.ListBuckets(r.Context())
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

// UpdateTransaction handles PATCH /api/transactions/{id}.
// Body: {"description": "...", "category": "...", "bucket": "...", "muted": true, "mute_reason": "..."}
// All fields are optional; only non-nil fields are written.
// @Summary Update a transaction
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID" format(uuid) example(00000000-0000-0000-0000-000000000001)
// @Param request body TransactionUpdateRequest true "Transaction update payload"
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /transactions/{id} [patch]
func (h *Handlers) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidPathValue(w, r, "id", "transaction")
	if !ok {
		return
	}
	body, ok := decodeAndValidateJSON[TransactionUpdateRequest](h, w, r)
	if !ok {
		return
	}

	if body.Category != nil && *body.Category != "" && !h.validateCategory(w, r, *body.Category) {
		return
	}
	if body.Bucket != nil && *body.Bucket != "" && !h.validateBucket(w, r, *body.Bucket) {
		return
	}

	if !h.patchTransaction(w, r, id, body) {
		return
	}

	tx, err := h.transactionStore.GetTransaction(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch updated transaction")
		return
	}
	writeJSON(w, http.StatusOK, tx)
}

func (h *Handlers) patchTransaction(
	w http.ResponseWriter,
	r *http.Request,
	id string,
	body TransactionUpdateRequest,
) bool {
	if body.Description != nil || body.Category != nil || body.Bucket != nil {
		update := store.TransactionUpdate{
			Description: body.Description,
			Category:    body.Category,
			Bucket:      body.Bucket,
		}
		if err := h.transactionStore.UpdateTransaction(r.Context(), id, update); err != nil {
			return h.writeTransactionPatchError(w, err, "update transaction details")
		}
	}
	if body.Muted != nil {
		reason := ""
		if body.MuteReason != nil {
			reason = *body.MuteReason
		}
		if err := h.muteStore.MuteTransaction(r.Context(), id, *body.Muted, reason); err != nil {
			return h.writeTransactionPatchError(w, err, "update transaction mute state")
		}
	} else if body.MuteReason != nil {
		if err := h.muteStore.UpdateMuteReason(r.Context(), id, *body.MuteReason); err != nil {
			return h.writeTransactionPatchError(w, err, "update transaction mute reason")
		}
	}
	return true
}

func (h *Handlers) writeTransactionPatchError(w http.ResponseWriter, err error, operation string) bool {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "transaction not found")
		return false
	}
	h.logger.Error(operation, "error", err)
	writeError(w, http.StatusInternalServerError, "failed to update transaction")
	return false
}

// ListMutedMerchants handles GET /api/muted-merchants.
//
// @Summary List muted merchant patterns
// @Tags Transactions
// @Produce json
// @Success 200 {array} MutedMerchantResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /muted-merchants [get]
func (h *Handlers) ListMutedMerchants(w http.ResponseWriter, r *http.Request) {
	merchants, err := h.muteStore.GetMutedMerchantsWithCount(r.Context())
	if err != nil {
		h.logger.Error("list muted merchants", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list muted merchants")
		return
	}
	writeJSON(w, http.StatusOK, merchants)
}

// MuteByMerchant handles POST /api/muted-merchants.
// Body: {"pattern": "MERCHANT NAME", "reason": "optional"}
//
// @Summary Mute transactions by merchant pattern
// @Tags Transactions
// @Accept json
// @Produce json
// @Param request body MuteMerchantRequest true "Merchant mute payload"
// @Success 201 {object} MuteMerchantResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /muted-merchants [post]
func (h *Handlers) MuteByMerchant(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeAndValidateJSON[MuteMerchantRequest](h, w, r)
	if !ok {
		return
	}
	if err := h.muteStore.MuteByMerchant(r.Context(), body.Pattern, body.Reason); err != nil {
		h.logger.Error("mute by merchant", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mute merchant")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"pattern": body.Pattern})
}

// UpdateMerchantReason handles PATCH /api/muted-merchants/{id}.
// Body: {"reason": "optional text"}
//
// @Summary Update a muted merchant reason
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Muted merchant ID" format(uuid) example(00000000-0000-0000-0000-00000000c003)
// @Param request body MerchantReasonRequest true "Muted merchant reason payload"
// @Success 200 {object} MerchantReasonResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /muted-merchants/{id} [patch]
func (h *Handlers) UpdateMerchantReason(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidPathValue(w, r, "id", "muted merchant")
	if !ok {
		return
	}
	body, ok := decodeAndValidateJSON[MerchantReasonRequest](h, w, r)
	if !ok {
		return
	}
	if err := h.muteStore.UpdateMerchantReason(r.Context(), id, body.Reason); err != nil {
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
//
// @Summary Delete a muted merchant pattern
// @Tags Transactions
// @Param id path string true "Muted merchant ID" format(uuid) example(00000000-0000-0000-0000-00000000c003)
// @Param unmute query bool false "Unmute existing transactions matching the merchant pattern"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /muted-merchants/{id} [delete]
func (h *Handlers) DeleteMutedMerchant(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidPathValue(w, r, "id", "muted merchant")
	if !ok {
		return
	}
	query, ok := decodeAndValidateQuery[deleteMutedMerchantQuery](h, w, r)
	if !ok {
		return
	}

	var err error
	if query.Unmute {
		err = h.muteStore.DeleteMutedMerchantAndUnmute(r.Context(), id)
	} else {
		err = h.muteStore.DeleteMutedMerchant(r.Context(), id)
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
//
// @Summary Categorize all matching merchant transactions
// @Tags Transactions
// @Accept json
// @Produce json
// @Param request body CategorizeMerchantRequest true "Merchant categorization payload"
// @Success 200 {object} CategorizeMerchantResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /merchants/categorize [post]
func (h *Handlers) CategorizeMerchant(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeAndValidateJSON[CategorizeMerchantRequest](h, w, r)
	if !ok {
		return
	}
	n, err := h.muteStore.CategorizeMerchant(r.Context(), body.Merchant, body.Category, body.Bucket)
	if err != nil {
		h.logger.Error("categorize merchant", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to categorize merchant")
		return
	}
	writeJSON(w, http.StatusOK, CategorizeMerchantResponse{Updated: n})
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
	facets, err := h.transactionStore.GetFacets(r.Context())
	if err != nil {
		h.logger.Error("get facets", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch facets")
		return
	}
	writeJSON(w, http.StatusOK, facets)
}
