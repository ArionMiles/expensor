package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// --- daemon control ---

// HandleStartDaemon handles POST /api/daemon/start.
// Body: {"reader": "gmail"}
// Triggers the background daemon with the given reader if it is not already running.
// @Summary Start the daemon
// @Tags Bootstrap
// @Accept json
// @Produce json
// @Param request body DocDaemonReaderRequest true "Daemon start request"
// @Success 200 {object} DocStatusOnlyResponse "Daemon already running"
// @Success 202 {object} DocStatusOnlyResponse "Daemon starting"
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 501 {object} DocErrorResponse
// @Router /daemon/start [post]
func (h *Handlers) HandleStartDaemon(w http.ResponseWriter, r *http.Request) {
	if h.startFn == nil {
		writeError(w, http.StatusNotImplemented, "daemon start not configured")
		return
	}
	if h.daemon.Status().Running {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already_running"})
		return
	}

	var body struct {
		Reader string `json:"reader"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Reader == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"reader\": \"<name>\"}")
		return
	}
	if _, err := h.registry.GetReader(body.Reader); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("reader %q not found", body.Reader))
		return
	}

	h.logger.Info("daemon start requested", "reader", body.Reader)
	h.startFn(body.Reader)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "starting"})
}

// HandleRescan handles POST /api/daemon/rescan.
// Body: {"reader": "<name>"}
// Stops any running daemon and restarts with forceRescan=true, bypassing the
// checkpoint and state deduplication so the full lookback window is scanned.
// @Summary Trigger a full rescan
// @Tags Bootstrap
// @Accept json
// @Produce json
// @Param request body DocDaemonReaderRequest true "Daemon rescan request"
// @Success 202 {object} DocStatusOnlyResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 501 {object} DocErrorResponse
// @Router /daemon/rescan [post]
func (h *Handlers) HandleRescan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reader string `json:"reader"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Reader == "" {
		writeError(w, http.StatusBadRequest, `body must be {"reader": "<name>"}`)
		return
	}
	if _, err := h.registry.GetReader(body.Reader); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("reader %q not found", body.Reader))
		return
	}

	if h.rescanFn == nil {
		writeError(w, http.StatusNotImplemented, "rescan not configured")
		return
	}
	h.rescanFn(body.Reader)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "rescanning"})
}

// HandleGetActiveReader handles GET /api/config/active-reader.
// Returns the reader name persisted from the last daemon start, or "" if none.
// @Summary Get the active reader
// @Tags Config
// @Produce json
// @Success 200 {object} DocActiveReaderResponse
// @Failure 500 {object} DocErrorResponse
// @Router /config/active-reader [get]
func (h *Handlers) HandleGetActiveReader(w http.ResponseWriter, r *http.Request) {
	reader, err := h.readActiveReader(r.Context())
	if err != nil {
		h.logger.Error("failed to read active reader", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to read active reader")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reader": reader})
}
