package api

import "net/http"

// HandleTriggerSync triggers an immediate community content sync.
// POST /api/config/sync
// @Summary Trigger community content sync
// @Tags Config
// @Accept json
// @Produce json
// @Success 200 {object} DocStatusOnlyResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/sync [post]
func (h *Handlers) HandleTriggerSync(w http.ResponseWriter, r *http.Request) {
	if h.syncFn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync not configured"})
		return
	}
	go h.syncFn()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleGetSyncStatus returns the last community sync status.
// GET /api/config/sync/status
// @Summary Get community sync status
// @Tags Config
// @Produce json
// @Success 200 {object} DocSyncStatusResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/sync/status [get]
func (h *Handlers) HandleGetSyncStatus(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	status, err := h.store.GetSyncStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}
