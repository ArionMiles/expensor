package httpapi

import (
	"net/http"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

// ListExtractionDiagnostics handles GET /api/extraction-diagnostics.
// @Summary List extraction diagnostics
// @Tags Extraction Diagnostics
// @Produce json
// @Param status query string false "Diagnostic status filter" Enums(open,resolved,ignored,all) default(open)
// @Param limit query int false "Maximum rows to return" minimum(1) default(20)
// @Success 200 {array} ExtractionDiagnosticResponse
// @Failure 422 {object} ErrorResponse
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

	rows, err := h.diagnosticStore.ListExtractionDiagnostics(r.Context(), requestTenant(r), filter)
	if err != nil {
		writeError(w, r, err)
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
// @Param id path string true "Diagnostic ID" format(uuid) example(00000000-0000-0000-0000-00000000c002)
// @Success 200 {object} ExtractionDiagnosticResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /extraction-diagnostics/{id} [get]
func (h *Handlers) GetExtractionDiagnostic(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidPathValue(w, r, "id", "extraction diagnostic")
	if !ok {
		return
	}

	row, err := h.diagnosticStore.GetExtractionDiagnostic(r.Context(), requestTenant(r), id)
	if err != nil {
		writeError(w, r, err)
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
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /extraction-diagnostics/{id} [patch]
func (h *Handlers) UpdateExtractionDiagnosticStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidPathValue(w, r, "id", "extraction diagnostic")
	if !ok {
		return
	}

	body, ok := decodeAndValidateJSON[ExtractionDiagnosticStatusRequest](h, w, r)
	if !ok {
		return
	}

	row, err := h.diagnosticStore.UpdateExtractionDiagnosticStatus(r.Context(), requestTenant(r), id, body.Status)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}
