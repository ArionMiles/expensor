package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/store"
)

// ListBanks returns the embedded bank color mappings.
// GET /api/config/banks
// @Summary List bank color mappings
// @Tags Taxonomy
// @Produce json
// @Success 200 {array} BankColorResponse
// @Router /config/banks [get]
func (h *Handlers) ListBanks(w http.ResponseWriter, r *http.Request) {
	data := h.banksData
	if len(data) == 0 {
		data = []byte("[]")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.logger.Warn("failed to write banks response", "error", err)
	}
}

// GetPreferences handles GET /api/config/preferences.
// @Summary Get application preferences
// @Tags Config
// @Produce json
// @Success 200 {object} PreferencesResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/preferences [get]
func (h *Handlers) GetPreferences(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.preferences(r.Context(), requestTenant(r)))
}

// PatchPreferences handles PATCH /api/config/preferences.
// @Summary Update application preferences
// @Tags Config
// @Accept json
// @Produce json
// @Param request body PreferencesPatchRequest true "Preferences to update"
// @Success 200 {object} PreferencesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/preferences [patch]
func (h *Handlers) PatchPreferences(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeJSONRequest[PreferencesPatchRequest](w, r)
	if !ok {
		return
	}
	normalizePreferencesPatch(&body)
	if !h.validateRequest(w, "body", body) {
		return
	}
	if err := h.persistPreferences(r.Context(), requestTenant(r), body); err != nil {
		h.logger.Error("update preferences", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update preferences")
		return
	}
	writeJSON(w, http.StatusOK, h.preferences(r.Context(), requestTenant(r)))
}

func (h *Handlers) preferences(ctx context.Context, tenant store.Tenant) PreferencesResponse {
	return PreferencesResponse{
		BaseCurrency: h.currentBaseCurrency(ctx, tenant),
		ScanInterval: h.storedIntPreference(ctx, tenant, "scan_interval", h.scanInterval),
		LookbackDays: h.storedIntPreference(ctx, tenant, "lookback_days", h.lookbackDays),
		Timezone:     h.storedPreference(ctx, tenant, "app.timezone", ""),
		TimeFormat:   h.storedPreference(ctx, tenant, "app.time_format", "HH:mm"),
	}
}

func (h *Handlers) storedPreference(ctx context.Context, tenant store.Tenant, key, fallback string) string {
	value, err := h.settingsStore.GetAppConfig(ctx, tenant, key)
	if err != nil || value == "" {
		return fallback
	}
	return value
}

func (h *Handlers) storedIntPreference(ctx context.Context, tenant store.Tenant, key string, fallback int) int {
	value := h.storedPreference(ctx, tenant, key, "")
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizePreferencesPatch(body *PreferencesPatchRequest) {
	if body.BaseCurrency != nil {
		value := strings.ToUpper(strings.TrimSpace(*body.BaseCurrency))
		body.BaseCurrency = &value
	}
	if body.Timezone != nil {
		value := strings.TrimSpace(*body.Timezone)
		body.Timezone = &value
	}
	if body.TimeFormat != nil {
		value := strings.TrimSpace(*body.TimeFormat)
		body.TimeFormat = &value
	}
}

func (h *Handlers) persistPreferences(ctx context.Context, tenant store.Tenant, body PreferencesPatchRequest) error {
	values := []struct {
		key   string
		value *string
	}{
		{key: "base_currency", value: body.BaseCurrency},
		{key: "scan_interval", value: intString(body.ScanInterval)},
		{key: "lookback_days", value: intString(body.LookbackDays)},
		{key: "app.timezone", value: body.Timezone},
		{key: "app.time_format", value: body.TimeFormat},
	}
	for _, preference := range values {
		if preference.value == nil {
			continue
		}
		if err := h.settingsStore.SetAppConfig(ctx, tenant, preference.key, *preference.value); err != nil {
			return err
		}
	}
	return nil
}

func intString(value *int) *string {
	if value == nil {
		return nil
	}
	result := strconv.Itoa(*value)
	return &result
}

func (h *Handlers) missingSetupPreferences(ctx context.Context, tenant store.Tenant) []string {
	required := []struct {
		key   string
		field string
	}{
		{key: "base_currency", field: "base_currency"},
		{key: "app.timezone", field: "timezone"},
		{key: "app.time_format", field: "time_format"},
	}
	missing := make([]string, 0, len(required))
	for _, pref := range required {
		value, err := h.settingsStore.GetAppConfig(ctx, tenant, pref.key)
		if err != nil || strings.TrimSpace(value) == "" {
			missing = append(missing, pref.field)
		}
	}
	return missing
}

// GetSetupStatus handles GET /api/config/setup-status.
// @Summary Get first-run setup status
// @Tags Config
// @Produce json
// @Success 200 {object} SetupStatusResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/setup-status [get]
func (h *Handlers) GetSetupStatus(w http.ResponseWriter, r *http.Request) {
	missing := h.missingSetupPreferences(r.Context(), requestTenant(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"required": len(missing) > 0,
		"missing":  missing,
	})
}

// GetReaderCheckpoint handles GET /api/config/providers/{name}/checkpoint.
// Returns the last scan timestamp for the reader (or null if no checkpoint exists).
// @Summary Get a reader checkpoint
// @Tags Config
// @Produce json
// @Param name path string true "Reader name" Enums(thunderbird,gmail) example(thunderbird)
// @Success 200 {object} ProviderCheckpointResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/providers/{name}/checkpoint [get]
func (h *Handlers) GetReaderCheckpoint(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	val, err := h.settingsStore.GetAppConfig(r.Context(), requestTenant(r), "reader."+name+".last_scan_at")
	if err != nil || val == "" {
		writeJSON(w, http.StatusOK, map[string]any{"last_scan_at": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"last_scan_at": val})
}

// ClearReaderCheckpoint handles DELETE /api/config/providers/{name}/checkpoint.
// Clears the checkpoint so the next scan processes the full lookback window.
// If the daemon is currently running, it is restarted so it picks up the
// now-absent checkpoint immediately rather than waiting for the next interval.
// @Summary Clear a reader checkpoint
// @Tags Config
// @Param name path string true "Reader name" Enums(thunderbird,gmail) example(thunderbird)
// @Success 204 "No Content"
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/providers/{name}/checkpoint [delete]
func (h *Handlers) ClearReaderCheckpoint(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.settingsStore.SetAppConfig(r.Context(), requestTenant(r), "reader."+name+".last_scan_at", ""); err != nil {
		h.logger.Error("clear reader checkpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to clear checkpoint")
		return
	}
	// Restart the running daemon so it reloads the (now-absent) checkpoint and
	// immediately starts a full-lookback scan rather than continuing from the
	// stale in-memory value.
	if h.daemon != nil && h.daemon.Status().Running {
		h.daemon.Restart(daemon.RunRequest{Tenant: requestTenant(r), Reader: name})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) resolveTimezone(ctx context.Context, tenant store.Tenant, requested string) string {
	const fallback = "UTC"

	if requested != "" {
		if _, err := time.LoadLocation(requested); err == nil {
			return requested
		}
	}
	if configured, err := h.settingsStore.GetAppConfig(ctx, tenant, "app.timezone"); err == nil && configured != "" {
		if _, err := time.LoadLocation(configured); err == nil {
			return configured
		}
	}
	return fallback
}
