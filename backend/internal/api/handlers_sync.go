package api

import "net/http"

// TriggerSync triggers an immediate community content sync.
// POST /api/config/sync
// @Summary Trigger community content sync
// @Tags Config
// @Accept json
// @Produce json
// @Success 200 {object} StatusOnlyResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/sync [post]
func (h *Handlers) TriggerSync(w http.ResponseWriter, r *http.Request) {
	if h.syncFn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync not configured"})
		return
	}
	go h.syncFn()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetSyncStatus returns the last community sync status.
// GET /api/config/sync/status
// @Summary Get community sync status
// @Tags Config
// @Produce json
// @Success 200 {object} SyncStatusResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/sync/status [get]
func (h *Handlers) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.syncStore.GetSyncStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}
