package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
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

// validTimeFormats is the set of accepted time_format values.
var validTimeFormats = map[string]bool{
	"HH:mm":     true,
	"HH:mm:ss":  true,
	"h:mm a":    true,
	"h:mm:ss a": true,
}

// GetPreferences handles GET /api/config/preferences.
// @Summary Get application preferences
// @Tags Config
// @Produce json
// @Success 200 {object} PreferencesResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/preferences [get]
func (h *Handlers) GetPreferences(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.preferences(r.Context()))
}

// PatchPreferences handles PATCH /api/config/preferences.
// @Summary Update application preferences
// @Tags Config
// @Accept json
// @Produce json
// @Param request body PreferencesPatchRequest true "Preferences to update"
// @Success 200 {object} PreferencesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/preferences [patch]
func (h *Handlers) PatchPreferences(w http.ResponseWriter, r *http.Request) {
	var body PreferencesPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	if err := normalizePreferencesPatch(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.persistPreferences(r.Context(), body); err != nil {
		h.logger.Error("update preferences", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update preferences")
		return
	}
	writeJSON(w, http.StatusOK, h.preferences(r.Context()))
}

func (h *Handlers) preferences(ctx context.Context) PreferencesResponse {
	return PreferencesResponse{
		BaseCurrency: h.currentBaseCurrency(ctx),
		ScanInterval: h.storedIntPreference(ctx, "scan_interval", h.scanInterval),
		LookbackDays: h.storedIntPreference(ctx, "lookback_days", h.lookbackDays),
		Timezone:     h.storedPreference(ctx, "app.timezone", ""),
		TimeFormat:   h.storedPreference(ctx, "app.time_format", "HH:mm"),
	}
}

func (h *Handlers) storedPreference(ctx context.Context, key, fallback string) string {
	value, err := h.settingsStore.GetAppConfig(ctx, key)
	if err != nil || value == "" {
		return fallback
	}
	return value
}

func (h *Handlers) storedIntPreference(ctx context.Context, key string, fallback int) int {
	value := h.storedPreference(ctx, key, "")
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizePreferencesPatch(body *PreferencesPatchRequest) error {
	if body.BaseCurrency != nil {
		value := strings.ToUpper(strings.TrimSpace(*body.BaseCurrency))
		if !isCurrencyCode(value) {
			return errors.New("base_currency must be a 3-letter ISO 4217 code (e.g. INR, USD)")
		}
		body.BaseCurrency = &value
	}
	if body.ScanInterval != nil && (*body.ScanInterval < 10 || *body.ScanInterval > 3600) {
		return errors.New("scan_interval must be an integer between 10 and 3600 seconds")
	}
	if body.LookbackDays != nil && (*body.LookbackDays < 1 || *body.LookbackDays > 3650) {
		return errors.New("lookback_days must be an integer between 1 and 3650")
	}
	if body.Timezone != nil {
		value := strings.TrimSpace(*body.Timezone)
		if _, err := time.LoadLocation(value); err != nil {
			return errors.New("invalid IANA timezone string")
		}
		body.Timezone = &value
	}
	if body.TimeFormat != nil {
		value := strings.TrimSpace(*body.TimeFormat)
		if !validTimeFormats[value] {
			return errors.New("invalid time_format; accepted: HH:mm, HH:mm:ss, h:mm a, h:mm:ss a")
		}
		body.TimeFormat = &value
	}
	return nil
}

func isCurrencyCode(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func (h *Handlers) persistPreferences(ctx context.Context, body PreferencesPatchRequest) error {
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
		if err := h.settingsStore.SetAppConfig(ctx, preference.key, *preference.value); err != nil {
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

func (h *Handlers) missingSetupPreferences(ctx context.Context) []string {
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
		value, err := h.settingsStore.GetAppConfig(ctx, pref.key)
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
	missing := h.missingSetupPreferences(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"required": len(missing) > 0,
		"missing":  missing,
	})
}

// GetReaderCheckpoint handles GET /api/config/readers/{name}/checkpoint.
// Returns the last scan timestamp for the reader (or null if no checkpoint exists).
// @Summary Get a reader checkpoint
// @Tags Config
// @Produce json
// @Param name path string true "Reader name" Enums(thunderbird,gmail) example(thunderbird)
// @Success 200 {object} ReaderCheckpointResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/readers/{name}/checkpoint [get]
func (h *Handlers) GetReaderCheckpoint(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	val, err := h.settingsStore.GetAppConfig(r.Context(), "reader."+name+".last_scan_at")
	if err != nil || val == "" {
		writeJSON(w, http.StatusOK, map[string]any{"last_scan_at": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"last_scan_at": val})
}

// ClearReaderCheckpoint handles DELETE /api/config/readers/{name}/checkpoint.
// Clears the checkpoint so the next scan processes the full lookback window.
// If the daemon is currently running, it is restarted so it picks up the
// now-absent checkpoint immediately rather than waiting for the next interval.
// @Summary Clear a reader checkpoint
// @Tags Config
// @Param name path string true "Reader name" Enums(thunderbird,gmail) example(thunderbird)
// @Success 204 "No Content"
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/readers/{name}/checkpoint [delete]
func (h *Handlers) ClearReaderCheckpoint(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.settingsStore.SetAppConfig(r.Context(), "reader."+name+".last_scan_at", ""); err != nil {
		h.logger.Error("clear reader checkpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to clear checkpoint")
		return
	}
	// Restart the running daemon so it reloads the (now-absent) checkpoint and
	// immediately starts a full-lookback scan rather than continuing from the
	// stale in-memory value.
	if h.daemon.Status().Running && h.restartFn != nil {
		h.restartFn(name)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) resolveTimezone(ctx context.Context, requested string) string {
	const fallback = "UTC"

	if requested != "" {
		if _, err := time.LoadLocation(requested); err == nil {
			return requested
		}
	}
	if configured, err := h.settingsStore.GetAppConfig(ctx, "app.timezone"); err == nil && configured != "" {
		if _, err := time.LoadLocation(configured); err == nil {
			return configured
		}
	}
	return fallback
}
