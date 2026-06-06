package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

// ListExtractionDiagnostics handles GET /api/extraction-diagnostics.
// @Summary List extraction diagnostics
// @Tags Extraction Diagnostics
// @Produce json
// @Param status query string false "Diagnostic status filter" Enums(open,resolved,ignored,all) default(open)
// @Param limit query int false "Maximum rows to return" minimum(1) default(20)
// @Success 200 {array} ExtractionDiagnosticResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /extraction-diagnostics [get]
func (h *Handlers) ListExtractionDiagnostics(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeAndValidateQuery[diagnosticListQuery](h, w, r)
	if !ok {
		return
	}
	if query.Status == "" {
		query.Status = store.DiagnosticStatusOpen
	}

	filter := store.DiagnosticFilter{Status: query.Status}
	if query.Limit != nil {
		filter.Limit = *query.Limit
	}

	rows, err := h.diagnosticStore.ListExtractionDiagnostics(r.Context(), filter)
	if err != nil {
		h.logger.Error("list extraction diagnostics", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list extraction diagnostics")
		return
	}
	if rows == nil {
		rows = []store.ExtractionDiagnosticRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

type diagnosticListQuery struct {
	Status string `form:"status" validate:"omitempty,oneof=open resolved ignored all"`
	Limit  *int   `form:"limit" validate:"omitempty,min=1"`
}

// GetExtractionDiagnostic handles GET /api/extraction-diagnostics/{id}.
// @Summary Get an extraction diagnostic
// @Tags Extraction Diagnostics
// @Produce json
// @Param id path string true "Diagnostic ID" format(uuid) example(00000000-0000-0000-0000-00000000c002)
// @Success 200 {object} ExtractionDiagnosticResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /extraction-diagnostics/{id} [get]
func (h *Handlers) GetExtractionDiagnostic(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid extraction diagnostic id")
		return
	}

	row, err := h.diagnosticStore.GetExtractionDiagnostic(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "extraction diagnostic not found")
			return
		}
		h.logger.Error("get extraction diagnostic", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch extraction diagnostic")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// UpdateExtractionDiagnosticStatus handles PATCH /api/extraction-diagnostics/{id}.
// @Summary Update extraction diagnostic status
// @Tags Extraction Diagnostics
// @Accept json
// @Produce json
// @Param id path string true "Diagnostic ID" format(uuid) example(00000000-0000-0000-0000-00000000c002)
// @Param request body ExtractionDiagnosticStatusRequest true "Diagnostic status payload"
// @Success 200 {object} ExtractionDiagnosticResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /extraction-diagnostics/{id} [patch]
func (h *Handlers) UpdateExtractionDiagnosticStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid extraction diagnostic id")
		return
	}

	var body ExtractionDiagnosticStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !h.validateRequest(w, "body", body) {
		return
	}

	row, err := h.diagnosticStore.UpdateExtractionDiagnosticStatus(r.Context(), id, body.Status)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "extraction diagnostic not found")
			return
		}
		if errors.Is(err, store.ErrDiagnosticConflict) {
			writeError(w, http.StatusConflict, "open extraction diagnostic already exists")
			return
		}
		h.logger.Error("update extraction diagnostic status", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update extraction diagnostic")
		return
	}
	writeJSON(w, http.StatusOK, row)
}
