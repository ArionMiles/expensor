package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

// ListExtractionDiagnostics handles GET /api/extraction-diagnostics.
// @Summary List extraction diagnostics
// @Tags Extraction Diagnostics
// @Produce json
// @Param status query string false "Diagnostic status filter"
// @Param limit query int false "Maximum rows to return"
// @Success 200 {array} ExtractionDiagnosticResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /extraction-diagnostics [get]
func (h *Handlers) ListExtractionDiagnostics(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = store.DiagnosticStatusOpen
	}
	if err := store.ValidateDiagnosticFilterStatus(status); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid diagnostic status")
		return
	}

	filter := store.DiagnosticFilter{Status: status}
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusUnprocessableEntity, "invalid limit")
			return
		}
		filter.Limit = limit
	}

	rows, err := h.store.ListExtractionDiagnostics(r.Context(), filter)
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

// GetExtractionDiagnostic handles GET /api/extraction-diagnostics/{id}.
// @Summary Get an extraction diagnostic
// @Tags Extraction Diagnostics
// @Produce json
// @Param id path string true "Diagnostic ID"
// @Success 200 {object} ExtractionDiagnosticResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /extraction-diagnostics/{id} [get]
func (h *Handlers) GetExtractionDiagnostic(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid extraction diagnostic id")
		return
	}

	row, err := h.store.GetExtractionDiagnostic(r.Context(), id)
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

// UpdateExtractionDiagnosticStatus handles PUT /api/extraction-diagnostics/{id}/status.
// @Summary Update extraction diagnostic status
// @Tags Extraction Diagnostics
// @Accept json
// @Produce json
// @Param id path string true "Diagnostic ID"
// @Param request body ExtractionDiagnosticStatusRequest true "Diagnostic status payload"
// @Success 200 {object} ExtractionDiagnosticResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /extraction-diagnostics/{id}/status [put]
func (h *Handlers) UpdateExtractionDiagnosticStatus(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid extraction diagnostic id")
		return
	}

	var body ExtractionDiagnosticStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	if !validDiagnosticUpdateStatus(body.Status) {
		writeError(w, http.StatusUnprocessableEntity, "invalid diagnostic status")
		return
	}

	row, err := h.store.UpdateExtractionDiagnosticStatus(r.Context(), id, body.Status)
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

func validDiagnosticUpdateStatus(status string) bool {
	switch status {
	case store.DiagnosticStatusOpen, store.DiagnosticStatusResolved, store.DiagnosticStatusIgnored:
		return true
	default:
		return false
	}
}
