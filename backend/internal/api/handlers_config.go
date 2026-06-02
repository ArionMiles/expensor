package api

import (
	"context"
	"encoding/json"
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

// GetBaseCurrency handles GET /api/config/base-currency.
// @Summary Get the base currency
// @Tags Config
// @Produce json
// @Success 200 {object} BaseCurrencyResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/base-currency [get]
func (h *Handlers) GetBaseCurrency(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"base_currency": h.currentBaseCurrency(r.Context())})
}

// SetBaseCurrency handles PUT /api/config/base-currency.
// Body: {"base_currency": "USD"}
// @Summary Set the base currency
// @Tags Config
// @Accept json
// @Produce json
// @Param request body BaseCurrencyRequest true "Base currency payload"
// @Success 200 {object} BaseCurrencyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/base-currency [put]
func (h *Handlers) SetBaseCurrency(w http.ResponseWriter, r *http.Request) {
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
	if err := h.settingsStore.SetAppConfig(r.Context(), "base_currency", currency); err != nil {
		h.logger.Error("set base currency", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update base currency")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_currency": currency})
}

// GetScanInterval handles GET /api/config/scan-interval.
// @Summary Get the scan interval
// @Tags Config
// @Produce json
// @Success 200 {object} ScanIntervalResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/scan-interval [get]
func (h *Handlers) GetScanInterval(w http.ResponseWriter, r *http.Request) {
	val := strconv.Itoa(h.scanInterval)
	if dbVal, err := h.settingsStore.GetAppConfig(r.Context(), "scan_interval"); err == nil && dbVal != "" {
		val = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"scan_interval": val})
}

// SetScanInterval handles PUT /api/config/scan-interval.
// Body: {"scan_interval": "120"}
// @Summary Set the scan interval
// @Tags Config
// @Accept json
// @Produce json
// @Param request body ScanIntervalRequest true "Scan interval payload"
// @Success 200 {object} ScanIntervalResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/scan-interval [put]
func (h *Handlers) SetScanInterval(w http.ResponseWriter, r *http.Request) {
	h.setNumericAppConfig(w, r, numericConfigSpec{
		key:         "scan_interval",
		min:         10,
		max:         3600,
		bodyMessage: "body must be {\"scan_interval\": \"<seconds>\"}",
		rangeMsg:    "scan_interval must be an integer between 10 and 3600 seconds",
		logMessage:  "set scan interval",
		errorMsg:    "failed to update scan interval",
	})
}

// GetLookbackDays handles GET /api/config/lookback-days.
// @Summary Get lookback days
// @Tags Config
// @Produce json
// @Success 200 {object} LookbackDaysResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/lookback-days [get]
func (h *Handlers) GetLookbackDays(w http.ResponseWriter, r *http.Request) {
	val := strconv.Itoa(h.lookbackDays)
	if dbVal, err := h.settingsStore.GetAppConfig(r.Context(), "lookback_days"); err == nil && dbVal != "" {
		val = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"lookback_days": val})
}

// SetLookbackDays handles PUT /api/config/lookback-days.
// Body: {"lookback_days": "365"}
// @Summary Set lookback days
// @Tags Config
// @Accept json
// @Produce json
// @Param request body LookbackDaysRequest true "Lookback days payload"
// @Success 200 {object} LookbackDaysResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/lookback-days [put]
func (h *Handlers) SetLookbackDays(w http.ResponseWriter, r *http.Request) {
	h.setNumericAppConfig(w, r, numericConfigSpec{
		key:         "lookback_days",
		min:         1,
		max:         3650,
		bodyMessage: "body must be {\"lookback_days\": \"<days>\"}",
		rangeMsg:    "lookback_days must be an integer between 1 and 3650",
		logMessage:  "set lookback days",
		errorMsg:    "failed to update lookback days",
	})
}

type numericConfigSpec struct {
	key         string
	min         int
	max         int
	bodyMessage string
	rangeMsg    string
	logMessage  string
	errorMsg    string
}

func (h *Handlers) setNumericAppConfig(w http.ResponseWriter, r *http.Request, spec numericConfigSpec) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body[spec.key] == "" {
		writeError(w, http.StatusUnprocessableEntity, spec.bodyMessage)
		return
	}
	value := body[spec.key]
	n, err := strconv.Atoi(value)
	if err != nil || n < spec.min || n > spec.max {
		writeError(w, http.StatusBadRequest, spec.rangeMsg)
		return
	}
	if err := h.settingsStore.SetAppConfig(r.Context(), spec.key, value); err != nil {
		h.logger.Error(spec.logMessage, "error", err)
		writeError(w, http.StatusInternalServerError, spec.errorMsg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{spec.key: value})
}

// validTimeFormats is the set of accepted time_format values.
var validTimeFormats = map[string]bool{
	"HH:mm":     true,
	"HH:mm:ss":  true,
	"h:mm a":    true,
	"h:mm:ss a": true,
}

// GetTimezone handles GET /api/config/timezone.
// @Summary Get the application timezone
// @Tags Config
// @Produce json
// @Success 200 {object} TimezoneResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/timezone [get]
func (h *Handlers) GetTimezone(w http.ResponseWriter, r *http.Request) {
	tz := ""
	if dbVal, err := h.settingsStore.GetAppConfig(r.Context(), "app.timezone"); err == nil && dbVal != "" {
		tz = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"timezone": tz})
}

// SetTimezone handles PUT /api/config/timezone.
// Body: {"timezone": "Asia/Kolkata"}
// @Summary Set the application timezone
// @Tags Config
// @Accept json
// @Produce json
// @Param request body TimezoneRequest true "Timezone payload"
// @Success 200 {object} TimezoneResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/timezone [put]
func (h *Handlers) SetTimezone(w http.ResponseWriter, r *http.Request) {
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
	if err := h.settingsStore.SetAppConfig(r.Context(), "app.timezone", tz); err != nil {
		h.logger.Error("set timezone", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update timezone")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"timezone": tz})
}

// GetTimeFormat handles GET /api/config/time-format.
// @Summary Get the time format
// @Tags Config
// @Produce json
// @Success 200 {object} TimeFormatResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/time-format [get]
func (h *Handlers) GetTimeFormat(w http.ResponseWriter, r *http.Request) {
	tf := "HH:mm"
	if dbVal, err := h.settingsStore.GetAppConfig(r.Context(), "app.time_format"); err == nil && dbVal != "" {
		tf = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"time_format": tf})
}

// SetTimeFormat handles PUT /api/config/time-format.
// Body: {"time_format": "HH:mm"}
// @Summary Set the time format
// @Tags Config
// @Accept json
// @Produce json
// @Param request body TimeFormatRequest true "Time format payload"
// @Success 200 {object} TimeFormatResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /config/time-format [put]
func (h *Handlers) SetTimeFormat(w http.ResponseWriter, r *http.Request) {
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
	if err := h.settingsStore.SetAppConfig(r.Context(), "app.time_format", tf); err != nil {
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
