package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

// HandleListLabels handles GET /api/config/labels.
// @Summary List labels
// @Tags Taxonomy
// @Produce json
// @Success 200 {array} DocLabelResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels [get]
func (h *Handlers) HandleListLabels(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	labels, err := h.store.ListLabels(r.Context())
	if err != nil {
		h.logger.Error("list labels", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list labels")
		return
	}
	writeJSON(w, http.StatusOK, labels)
}

// HandleCreateLabel handles POST /api/config/labels.
// Body: {"name": "food", "color": "#f59e0b"}
// @Summary Create a label
// @Tags Taxonomy
// @Accept json
// @Produce json
// @Param request body DocCreateLabelRequest true "Label payload"
// @Success 201 {object} DocLabelMutationResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels [post]
func (h *Handlers) HandleCreateLabel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"name\": \"<name>\", \"color\": \"<hex>\"}")
		return
	}
	if body.Color == "" {
		body.Color = "#6366f1"
	}
	if err := h.store.CreateLabel(r.Context(), body.Name, body.Color); err != nil {
		h.logger.Error("create label", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create label")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": body.Name, "color": body.Color})
}

// HandleUpdateLabel handles PUT /api/config/labels/{name}.
// Body: {"color": "#f59e0b"}
// @Summary Update a label
// @Tags Taxonomy
// @Accept json
// @Produce json
// @Param name path string true "Label name"
// @Param request body DocUpdateLabelRequest true "Label color payload"
// @Success 200 {object} DocLabelMutationResponse
// @Failure 404 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels/{name} [put]
func (h *Handlers) HandleUpdateLabel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	var body struct {
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Color == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"color\": \"<hex>\"}")
		return
	}
	if err := h.store.UpdateLabel(r.Context(), name, body.Color); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "label not found")
			return
		}
		h.logger.Error("update label", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update label")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "color": body.Color})
}

// HandleDeleteLabel handles DELETE /api/config/labels/{name}.
// Body: {"remove_from_transactions": true}
// @Summary Delete a label
// @Tags Taxonomy
// @Produce json
// @Param name path string true "Label name"
// @Success 204 "No Content"
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels/{name} [delete]
func (h *Handlers) HandleDeleteLabel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	removeFromTransactions, ok := taxonomyCleanupFlag(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteLabel(r.Context(), name, removeFromTransactions); err != nil {
		h.logger.Error("delete label", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete label")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleApplyLabel handles POST /api/config/labels/{name}/apply.
// Body: {"merchant_pattern": "swiggy"}
// @Summary Apply a label by merchant pattern
// @Tags Taxonomy
// @Accept json
// @Produce json
// @Param name path string true "Label name"
// @Param request body DocApplyLabelRequest true "Merchant pattern payload"
// @Success 200 {object} DocAppliedCountResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels/{name}/apply [post]
func (h *Handlers) HandleApplyLabel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	var body struct {
		MerchantPattern string `json:"merchant_pattern"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MerchantPattern == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"merchant_pattern\": \"<pattern>\"}")
		return
	}
	affected, err := h.store.ApplyLabelByMerchant(r.Context(), name, body.MerchantPattern)
	if err != nil {
		h.logger.Error("apply label", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to apply label")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applied": affected})
}

// HandleRemoveLabelByMerchant handles DELETE /api/config/labels/{name}/merchant.
// Body: {"merchant_pattern": "swiggy"}
func (h *Handlers) HandleRemoveLabelByMerchant(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	var body struct {
		MerchantPattern string `json:"merchant_pattern"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MerchantPattern == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"merchant_pattern\": \"<pattern>\"}")
		return
	}
	removed, err := h.store.RemoveLabelByMerchant(r.Context(), name, body.MerchantPattern)
	if err != nil {
		h.logger.Error("remove label by merchant", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove label")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

// HandleGetLabelMonthlySpend handles GET /api/stats/labels/monthly.
// Query params:
//   - dimension=labels|categories|buckets (default: labels)
//
// Response: {"labels":["Food","Travel"], "months":["2025-05","2025-06",...], "series":[{"label":"Food","data":[...]}]}
func (h *Handlers) HandleGetLabelMonthlySpend(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	dimension := strings.TrimSpace(r.URL.Query().Get("dimension"))
	if dimension == "" {
		dimension = "labels"
	}
	switch dimension {
	case "labels", "categories", "buckets":
	default:
		writeError(w, http.StatusBadRequest, "invalid dimension")
		return
	}

	data, err := h.store.GetMonthlyBreakdownSpend(r.Context(), dimension, 12)
	if err != nil {
		h.logger.Error("get monthly breakdown spend", "dimension", dimension, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch monthly breakdown spend")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// HandleGetLabelMappings handles GET /api/config/labels/mappings.
// Returns a map of label → persisted merchant patterns.
// @Summary Get label mappings
// @Tags Taxonomy
// @Produce json
// @Success 200 {object} DocLabelMappingsResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels/mappings [get]
func (h *Handlers) HandleGetLabelMappings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	mappings, err := h.store.GetLabelMappings(r.Context())
	if err != nil {
		h.logger.Error("get label mappings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get label mappings")
		return
	}
	writeJSON(w, http.StatusOK, mappings)
}

// HandleExportLabels handles GET /api/config/labels/export.
// Returns labels with their persisted merchant mappings as a downloadable JSON file.
func (h *Handlers) HandleExportLabels(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	labels, err := h.store.ListLabels(r.Context())
	if err != nil {
		h.logger.Error("export labels", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list labels")
		return
	}
	mappings, err := h.store.GetLabelMappings(r.Context())
	if err != nil {
		h.logger.Error("export label mappings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list label mappings")
		return
	}
	type exportRow struct {
		Name      string   `json:"name"`
		Color     string   `json:"color"`
		Merchants []string `json:"merchants,omitempty"`
	}
	export := make([]exportRow, 0, len(labels))
	for _, l := range labels {
		export = append(export, exportRow{
			Name:      l.Name,
			Color:     l.Color,
			Merchants: mappings[l.Name],
		})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="expensor-labels.json"`)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(export)
}

// HandleExportCategories handles GET /api/config/categories/export.
// Returns categories with their persisted merchant mappings as a downloadable JSON file.
func (h *Handlers) HandleExportCategories(w http.ResponseWriter, r *http.Request) {
	handleExportNamedTaxonomy(w, r, taxonomyExportConfig[store.Category]{
		handlers:    h,
		singular:    "category",
		plural:      "categories",
		filename:    "expensor-categories.json",
		list:        func(ctx context.Context) ([]store.Category, error) { return h.store.ListCategories(ctx) },
		getMappings: h.store.GetCategoryMappings,
		nameOf:      func(item store.Category) string { return item.Name },
	})
}

// HandleGetCategoryMappings handles GET /api/config/categories/mappings.
func (h *Handlers) HandleGetCategoryMappings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	mappings, err := h.store.GetCategoryMappings(r.Context())
	if err != nil {
		h.logger.Error("get category mappings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get category mappings")
		return
	}
	writeJSON(w, http.StatusOK, mappings)
}

// HandleListCategories handles GET /api/config/categories.
// @Summary List categories
// @Tags Taxonomy
// @Produce json
// @Success 200 {array} DocCategoryResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/categories [get]
func (h *Handlers) HandleListCategories(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	cats, err := h.store.ListCategories(r.Context())
	if err != nil {
		h.logger.Error("list categories", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list categories")
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

// HandleCreateCategory handles POST /api/config/categories.
// @Summary Create a category
// @Tags Taxonomy
// @Accept json
// @Produce json
// @Param request body DocCreateCategoryRequest true "Category payload"
// @Success 201 {object} DocNameResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/categories [post]
func (h *Handlers) HandleCreateCategory(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must include \"name\"")
		return
	}
	if err := h.store.CreateCategory(r.Context(), body.Name, body.Description); err != nil {
		h.logger.Error("create category", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create category")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": body.Name})
}

// HandleDeleteCategory handles DELETE /api/config/categories/{name}.
// @Summary Delete a category
// @Tags Taxonomy
// @Produce json
// @Param name path string true "Category name"
// @Success 204 "No Content"
// @Failure 404 {object} DocErrorResponse
// @Failure 409 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/categories/{name} [delete]
func (h *Handlers) HandleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	removeFromTransactions, ok := taxonomyCleanupFlag(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteCategory(r.Context(), name, removeFromTransactions); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "category not found")
			return
		}
		// Default categories return a plain error string.
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleApplyCategoryByMerchant handles POST /api/config/categories/{name}/apply.
func (h *Handlers) HandleApplyCategoryByMerchant(w http.ResponseWriter, r *http.Request) {
	h.handleApplyTaxonomyMerchant(w, r, "category", h.store.ApplyCategoryByMerchant)
}

// HandleRemoveCategoryByMerchant handles DELETE /api/config/categories/{name}/merchant.
func (h *Handlers) HandleRemoveCategoryByMerchant(w http.ResponseWriter, r *http.Request) {
	h.handleRemoveTaxonomyMerchant(w, r, "category", h.store.RemoveCategoryByMerchant)
}

// HandleListBuckets handles GET /api/config/buckets.
// @Summary List buckets
// @Tags Taxonomy
// @Produce json
// @Success 200 {array} DocBucketResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/buckets [get]
func (h *Handlers) HandleListBuckets(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	bkts, err := h.store.ListBuckets(r.Context())
	if err != nil {
		h.logger.Error("list buckets", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list buckets")
		return
	}
	writeJSON(w, http.StatusOK, bkts)
}

// HandleCreateBucket handles POST /api/config/buckets.
// @Summary Create a bucket
// @Tags Taxonomy
// @Accept json
// @Produce json
// @Param request body DocCreateBucketRequest true "Bucket payload"
// @Success 201 {object} DocNameResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/buckets [post]
func (h *Handlers) HandleCreateBucket(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must include \"name\"")
		return
	}
	if err := h.store.CreateBucket(r.Context(), body.Name, body.Description); err != nil {
		h.logger.Error("create bucket", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create bucket")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": body.Name})
}

// HandleDeleteBucket handles DELETE /api/config/buckets/{name}.
// @Summary Delete a bucket
// @Tags Taxonomy
// @Produce json
// @Param name path string true "Bucket name"
// @Success 204 "No Content"
// @Failure 404 {object} DocErrorResponse
// @Failure 409 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/buckets/{name} [delete]
func (h *Handlers) HandleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	removeFromTransactions, ok := taxonomyCleanupFlag(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteBucket(r.Context(), name, removeFromTransactions); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "bucket not found")
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleExportBuckets handles GET /api/config/buckets/export.
// Returns buckets with their persisted merchant mappings as a downloadable JSON file.
func (h *Handlers) HandleExportBuckets(w http.ResponseWriter, r *http.Request) {
	handleExportNamedTaxonomy(w, r, taxonomyExportConfig[store.Bucket]{
		handlers:    h,
		singular:    "bucket",
		plural:      "buckets",
		filename:    "expensor-buckets.json",
		list:        func(ctx context.Context) ([]store.Bucket, error) { return h.store.ListBuckets(ctx) },
		getMappings: h.store.GetBucketMappings,
		nameOf:      func(item store.Bucket) string { return item.Name },
	})
}

// HandleGetBucketMappings handles GET /api/config/buckets/mappings.
func (h *Handlers) HandleGetBucketMappings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	mappings, err := h.store.GetBucketMappings(r.Context())
	if err != nil {
		h.logger.Error("get bucket mappings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get bucket mappings")
		return
	}
	writeJSON(w, http.StatusOK, mappings)
}

// HandleApplyBucketByMerchant handles POST /api/config/buckets/{name}/apply.
func (h *Handlers) HandleApplyBucketByMerchant(w http.ResponseWriter, r *http.Request) {
	h.handleApplyTaxonomyMerchant(w, r, "bucket", h.store.ApplyBucketByMerchant)
}

// HandleRemoveBucketByMerchant handles DELETE /api/config/buckets/{name}/merchant.
func (h *Handlers) HandleRemoveBucketByMerchant(w http.ResponseWriter, r *http.Request) {
	h.handleRemoveTaxonomyMerchant(w, r, "bucket", h.store.RemoveBucketByMerchant)
}

func taxonomyCleanupFlag(w http.ResponseWriter, r *http.Request) (bool, bool) {
	var body struct {
		RemoveFromTransactions bool `json:"remove_from_transactions"`
	}
	removeFromTransactions := r.URL.Query().Get("remove_from_transactions") == queryValueTrue
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusUnprocessableEntity, "body must be {\"remove_from_transactions\": <bool>}")
			return false, false
		}
	}
	return removeFromTransactions || body.RemoveFromTransactions, true
}

type taxonomyExportRow struct {
	Name      string   `json:"name"`
	Merchants []string `json:"merchants,omitempty"`
}

type taxonomyExportConfig[T any] struct {
	handlers    *Handlers
	singular    string
	plural      string
	filename    string
	list        func(context.Context) ([]T, error)
	getMappings func(context.Context) (map[string][]string, error)
	nameOf      func(T) string
}

func handleExportNamedTaxonomy[T any](
	w http.ResponseWriter,
	r *http.Request,
	config taxonomyExportConfig[T],
) {
	h := config.handlers
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	items, err := config.list(r.Context())
	if err != nil {
		h.logger.Error("export taxonomy", "kind", config.singular, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list "+config.plural)
		return
	}
	mappings, err := config.getMappings(r.Context())
	if err != nil {
		h.logger.Error("export taxonomy mappings", "kind", config.singular, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list "+config.singular+" mappings")
		return
	}
	export := make([]taxonomyExportRow, 0, len(items))
	for _, item := range items {
		name := config.nameOf(item)
		export = append(export, taxonomyExportRow{Name: name, Merchants: mappings[name]})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", config.filename))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(export)
}

func (h *Handlers) handleApplyTaxonomyMerchant(
	w http.ResponseWriter,
	r *http.Request,
	kind string,
	apply func(context.Context, string, string) (int64, error),
) {
	h.handleTaxonomyMerchant(w, r, taxonomyMerchantAction{
		kind:        kind,
		action:      "apply",
		responseKey: "applied",
		update:      apply,
	})
}

func (h *Handlers) handleRemoveTaxonomyMerchant(
	w http.ResponseWriter,
	r *http.Request,
	kind string,
	remove func(context.Context, string, string) (int64, error),
) {
	h.handleTaxonomyMerchant(w, r, taxonomyMerchantAction{
		kind:        kind,
		action:      "remove",
		responseKey: "removed",
		update:      remove,
	})
}

type taxonomyMerchantAction struct {
	kind        string
	action      string
	responseKey string
	update      func(context.Context, string, string) (int64, error)
}

func (h *Handlers) handleTaxonomyMerchant(
	w http.ResponseWriter,
	r *http.Request,
	action taxonomyMerchantAction,
) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	var body struct {
		MerchantPattern string `json:"merchant_pattern"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MerchantPattern == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"merchant_pattern\": \"<pattern>\"}")
		return
	}
	count, err := action.update(r.Context(), name, body.MerchantPattern)
	if err != nil {
		h.logger.Error(action.action+" taxonomy merchant", "kind", action.kind, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to "+action.action+" merchant")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{action.responseKey: count})
}

// HandleAddLabels handles POST /api/transactions/{id}/labels.
// Body: {"labels": ["food", "recurring"]}
// @Summary Add labels to a transaction
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID"
// @Param request body DocTransactionLabelsRequest true "Labels payload"
// @Success 200 {object} DocStatusOnlyResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /transactions/{id}/labels [post]
func (h *Handlers) HandleAddLabels(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	var body struct {
		Labels []string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	if err := h.store.AddLabels(r.Context(), id, body.Labels); err != nil {
		h.logger.Error("add labels", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to add labels")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

// HandleRemoveLabel handles DELETE /api/transactions/{id}/labels/{label}.
// @Summary Remove a label from a transaction
// @Tags Transactions
// @Produce json
// @Param id path string true "Transaction ID"
// @Param label path string true "Label name"
// @Success 200 {object} DocStatusOnlyResponse
// @Failure 404 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /transactions/{id}/labels/{label} [delete]
func (h *Handlers) HandleRemoveLabel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	label := r.PathValue("label")

	if err := h.store.RemoveLabel(r.Context(), id, label); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "label not found on transaction")
			return
		}
		h.logger.Error("remove label", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove label")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
