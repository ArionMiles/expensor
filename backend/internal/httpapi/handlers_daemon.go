package httpapi

import (
	"fmt"
	"net/http"
)

// --- daemon control ---

// StartDaemon handles POST /api/daemon/start.
// Body: {"reader": "gmail"}
// Triggers the background daemon with the given reader. The daemon coordinator
// no-ops when the requested reader is already active.
// @Summary Start the daemon
// @Tags Bootstrap
// @Accept json
// @Produce json
// @Param request body DaemonReaderRequest true "Daemon start request"
// @Success 202 {object} StatusOnlyResponse "Daemon starting"
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 501 {object} ErrorResponse
// @Router /daemon/start [post]
func (h *Handlers) StartDaemon(w http.ResponseWriter, r *http.Request) {
	if h.startFn == nil {
		writeError(w, http.StatusNotImplemented, "daemon start not configured")
		return
	}

	body, ok := decodeAndValidateJSON[DaemonReaderRequest](h, w, r)
	if !ok {
		return
	}
	if _, err := h.registry.GetProvider(body.Reader); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("reader %q not found", body.Reader))
		return
	}

	h.logger.Info("daemon start requested", "reader", body.Reader)
	h.startFn(DaemonRunRequest{Tenant: requestTenant(r), Reader: body.Reader})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "starting"})
}

// Rescan handles POST /api/daemon/rescan.
// Body: {"reader": "<name>"}
// Stops any running daemon and restarts with forceRescan=true, bypassing the
// checkpoint and state deduplication so the full lookback window is scanned.
// @Summary Trigger a full rescan
// @Tags Bootstrap
// @Accept json
// @Produce json
// @Param request body DaemonReaderRequest true "Daemon rescan request"
// @Success 202 {object} StatusOnlyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 501 {object} ErrorResponse
// @Router /daemon/rescan [post]
func (h *Handlers) Rescan(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeAndValidateJSON[DaemonReaderRequest](h, w, r)
	if !ok {
		return
	}
	if _, err := h.registry.GetProvider(body.Reader); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("reader %q not found", body.Reader))
		return
	}

	if h.rescanFn == nil {
		writeError(w, http.StatusNotImplemented, "rescan not configured")
		return
	}
	h.rescanFn(DaemonRunRequest{Tenant: requestTenant(r), Reader: body.Reader})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "rescanning"})
}
