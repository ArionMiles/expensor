package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HandleListBanks returns the embedded bank color mappings.
// GET /api/config/banks
// @Summary List bank color mappings
// @Tags Taxonomy
// @Produce json
// @Success 200 {array} DocBankColorResponse
// @Router /config/banks [get]
func (h *Handlers) HandleListBanks(w http.ResponseWriter, r *http.Request) {
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

// HandleGetBaseCurrency handles GET /api/config/base-currency.
// @Summary Get the base currency
// @Tags Config
// @Produce json
// @Success 200 {object} DocBaseCurrencyResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/base-currency [get]
func (h *Handlers) HandleGetBaseCurrency(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_currency": h.getBaseCurrency(r.Context())})
}

// HandleSetBaseCurrency handles PUT /api/config/base-currency.
// Body: {"base_currency": "USD"}
// @Summary Set the base currency
// @Tags Config
// @Accept json
// @Produce json
// @Param request body DocBaseCurrencyRequest true "Base currency payload"
// @Success 200 {object} DocBaseCurrencyResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/base-currency [put]
func (h *Handlers) HandleSetBaseCurrency(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		BaseCurrency string `json:"base_currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	currency := strings.ToUpper(strings.TrimSpace(body.BaseCurrency))
	if len(currency) != 3 {
		writeError(w, http.StatusBadRequest, "base_currency must be a 3-letter ISO 4217 code (e.g. INR, USD)")
		return
	}
	for _, c := range currency {
		if c < 'A' || c > 'Z' {
			writeError(w, http.StatusBadRequest, "base_currency must be a 3-letter ISO 4217 code (e.g. INR, USD)")
			return
		}
	}
	if err := h.store.SetAppConfig(r.Context(), "base_currency", currency); err != nil {
		h.logger.Error("set base currency", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update base currency")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_currency": currency})
}

// HandleGetScanInterval handles GET /api/config/scan-interval.
// @Summary Get the scan interval
// @Tags Config
// @Produce json
// @Success 200 {object} DocScanIntervalResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/scan-interval [get]
func (h *Handlers) HandleGetScanInterval(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	val := strconv.Itoa(h.scanInterval)
	if dbVal, err := h.store.GetAppConfig(r.Context(), "scan_interval"); err == nil && dbVal != "" {
		val = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"scan_interval": val})
}

// HandleSetScanInterval handles PUT /api/config/scan-interval.
// Body: {"scan_interval": "120"}
// @Summary Set the scan interval
// @Tags Config
// @Accept json
// @Produce json
// @Param request body DocScanIntervalRequest true "Scan interval payload"
// @Success 200 {object} DocScanIntervalResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/scan-interval [put]
func (h *Handlers) HandleSetScanInterval(w http.ResponseWriter, r *http.Request) { //nolint:dupl // same shape as SetLookbackDays; different key and bounds
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		ScanInterval string `json:"scan_interval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ScanInterval == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"scan_interval\": \"<seconds>\"}")
		return
	}
	n, err := strconv.Atoi(body.ScanInterval)
	if err != nil || n < 10 || n > 3600 {
		writeError(w, http.StatusBadRequest, "scan_interval must be an integer between 10 and 3600 seconds")
		return
	}
	if err := h.store.SetAppConfig(r.Context(), "scan_interval", body.ScanInterval); err != nil {
		h.logger.Error("set scan interval", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update scan interval")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"scan_interval": body.ScanInterval})
}

// HandleGetLookbackDays handles GET /api/config/lookback-days.
// @Summary Get lookback days
// @Tags Config
// @Produce json
// @Success 200 {object} DocLookbackDaysResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/lookback-days [get]
func (h *Handlers) HandleGetLookbackDays(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	val := strconv.Itoa(h.lookbackDays)
	if dbVal, err := h.store.GetAppConfig(r.Context(), "lookback_days"); err == nil && dbVal != "" {
		val = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"lookback_days": val})
}

// HandleSetLookbackDays handles PUT /api/config/lookback-days.
// Body: {"lookback_days": "365"}
// @Summary Set lookback days
// @Tags Config
// @Accept json
// @Produce json
// @Param request body DocLookbackDaysRequest true "Lookback days payload"
// @Success 200 {object} DocLookbackDaysResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/lookback-days [put]
func (h *Handlers) HandleSetLookbackDays(w http.ResponseWriter, r *http.Request) { //nolint:dupl // same shape as SetScanInterval; different key and bounds
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		LookbackDays string `json:"lookback_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.LookbackDays == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"lookback_days\": \"<days>\"}")
		return
	}
	n, err := strconv.Atoi(body.LookbackDays)
	if err != nil || n < 1 || n > 3650 {
		writeError(w, http.StatusBadRequest, "lookback_days must be an integer between 1 and 3650")
		return
	}
	if err := h.store.SetAppConfig(r.Context(), "lookback_days", body.LookbackDays); err != nil {
		h.logger.Error("set lookback days", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update lookback days")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"lookback_days": body.LookbackDays})
}

// validTimeFormats is the set of accepted time_format values.
var validTimeFormats = map[string]bool{
	"HH:mm":     true,
	"HH:mm:ss":  true,
	"h:mm a":    true,
	"h:mm:ss a": true,
}

// HandleGetTimezone handles GET /api/config/timezone.
// @Summary Get the application timezone
// @Tags Config
// @Produce json
// @Success 200 {object} DocTimezoneResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/timezone [get]
func (h *Handlers) HandleGetTimezone(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	tz := ""
	if dbVal, err := h.store.GetAppConfig(r.Context(), "app.timezone"); err == nil && dbVal != "" {
		tz = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"timezone": tz})
}

// HandleSetTimezone handles PUT /api/config/timezone.
// Body: {"timezone": "Asia/Kolkata"}
// @Summary Set the application timezone
// @Tags Config
// @Accept json
// @Produce json
// @Param request body DocTimezoneRequest true "Timezone payload"
// @Success 200 {object} DocTimezoneResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/timezone [put]
func (h *Handlers) HandleSetTimezone(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Timezone string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	tz := strings.TrimSpace(body.Timezone)
	if _, err := time.LoadLocation(tz); err != nil {
		writeError(w, http.StatusBadRequest, "invalid IANA timezone string")
		return
	}
	if err := h.store.SetAppConfig(r.Context(), "app.timezone", tz); err != nil {
		h.logger.Error("set timezone", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update timezone")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"timezone": tz})
}

// HandleGetTimeFormat handles GET /api/config/time-format.
// @Summary Get the time format
// @Tags Config
// @Produce json
// @Success 200 {object} DocTimeFormatResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/time-format [get]
func (h *Handlers) HandleGetTimeFormat(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	tf := "HH:mm"
	if dbVal, err := h.store.GetAppConfig(r.Context(), "app.time_format"); err == nil && dbVal != "" {
		tf = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"time_format": tf})
}

// HandleSetTimeFormat handles PUT /api/config/time-format.
// Body: {"time_format": "HH:mm"}
// @Summary Set the time format
// @Tags Config
// @Accept json
// @Produce json
// @Param request body DocTimeFormatRequest true "Time format payload"
// @Success 200 {object} DocTimeFormatResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/time-format [put]
func (h *Handlers) HandleSetTimeFormat(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		TimeFormat string `json:"time_format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	tf := strings.TrimSpace(body.TimeFormat)
	if !validTimeFormats[tf] {
		writeError(w, http.StatusBadRequest, "invalid time_format; accepted: HH:mm, HH:mm:ss, h:mm a, h:mm:ss a")
		return
	}
	if err := h.store.SetAppConfig(r.Context(), "app.time_format", tf); err != nil {
		h.logger.Error("set time format", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update time format")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"time_format": tf})
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
		value, err := h.store.GetAppConfig(ctx, pref.key)
		if err != nil || strings.TrimSpace(value) == "" {
			missing = append(missing, pref.field)
		}
	}
	return missing
}

// HandleGetSetupStatus handles GET /api/config/setup-status.
// @Summary Get first-run setup status
// @Tags Config
// @Produce json
// @Success 200 {object} DocSetupStatusResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/setup-status [get]
func (h *Handlers) HandleGetSetupStatus(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	missing := h.missingSetupPreferences(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"required": len(missing) > 0,
		"missing":  missing,
	})
}

// HandleGetReaderCheckpoint handles GET /api/config/readers/{name}/checkpoint.
// Returns the last scan timestamp for the reader (or null if no checkpoint exists).
// @Summary Get a reader checkpoint
// @Tags Config
// @Produce json
// @Param name path string true "Reader name"
// @Success 200 {object} DocReaderCheckpointResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/readers/{name}/checkpoint [get]
func (h *Handlers) HandleGetReaderCheckpoint(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	val, err := h.store.GetAppConfig(r.Context(), "reader."+name+".last_scan_at")
	if err != nil || val == "" {
		writeJSON(w, http.StatusOK, map[string]any{"last_scan_at": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"last_scan_at": val})
}

// HandleClearReaderCheckpoint handles DELETE /api/config/readers/{name}/checkpoint.
// Clears the checkpoint so the next scan processes the full lookback window.
// If the daemon is currently running, it is restarted so it picks up the
// now-absent checkpoint immediately rather than waiting for the next interval.
// @Summary Clear a reader checkpoint
// @Tags Config
// @Produce json
// @Param name path string true "Reader name"
// @Success 204 "No Content"
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/readers/{name}/checkpoint [delete]
func (h *Handlers) HandleClearReaderCheckpoint(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	if err := h.store.SetAppConfig(r.Context(), "reader."+name+".last_scan_at", ""); err != nil {
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
	if h.store != nil {
		if configured, err := h.store.GetAppConfig(ctx, "app.timezone"); err == nil && configured != "" {
			if _, err := time.LoadLocation(configured); err == nil {
				return configured
			}
		}
	}
	return fallback
}
