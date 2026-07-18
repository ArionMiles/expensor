package httpapi

import (
	"net/http"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

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
	if h.community == nil {
		writeError(w, r, errors.E(errors.Unavailable, errors.User("sync not configured")))
		return
	}
	go h.community.Trigger()
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
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// GetCommunitySyncSettings returns community sync settings.
// GET /api/config/sync/settings
// @Summary Get community sync settings
// @Tags Config
// @Produce json
// @Success 200 {object} CommunitySyncSettingsResponse
// @Failure 500 {object} ErrorResponse
// @Router /config/sync/settings [get]
func (h *Handlers) GetCommunitySyncSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.syncStore.GetCommunitySyncSettings(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, communitySyncSettingsResponse(settings))
}

// PatchCommunitySyncSettings updates community sync settings.
// PATCH /api/config/sync/settings
// @Summary Update community sync settings
// @Tags Config
// @Accept json
// @Produce json
// @Param body body CommunitySyncSettingsPatchRequest true "Community sync settings patch"
// @Success 200 {object} CommunitySyncSettingsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /config/sync/settings [patch]
func (h *Handlers) PatchCommunitySyncSettings(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeAndValidateJSON[CommunitySyncSettingsPatchRequest](h, w, r)
	if !ok {
		return
	}
	settings, err := h.syncStore.PatchCommunitySyncSettings(r.Context(), store.CommunitySyncSettingsPatch{
		AutomaticSyncEnabled: body.AutomaticSyncEnabled,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, communitySyncSettingsResponse(settings))
}

func communitySyncSettingsResponse(settings store.CommunitySyncSettings) CommunitySyncSettingsResponse {
	enabled := true
	if settings.AutomaticSyncEnabled != nil {
		enabled = *settings.AutomaticSyncEnabled
	}
	return CommunitySyncSettingsResponse{AutomaticSyncEnabled: enabled}
}
