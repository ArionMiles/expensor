package httpapi

import (
	"log/slog"
	"net/http"
)

const (
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
)

// GetAdminLoggingSettings handles GET /api/admin/logging/settings.
// @Summary Get runtime logging settings
// @Tags Admin
// @Produce json
// @Success 200 {object} AdminLoggingSettingsResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /admin/logging/settings [get]
func (h *Handlers) GetAdminLoggingSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, h.adminLoggingSettingsResponse())
}

// PatchAdminLoggingSettings handles PATCH /api/admin/logging/settings.
// @Summary Update runtime logging settings
// @Tags Admin
// @Accept json
// @Produce json
// @Param request body AdminLoggingSettingsPatchRequest true "Runtime logging settings patch"
// @Success 200 {object} AdminLoggingSettingsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Router /admin/logging/settings [patch]
func (h *Handlers) PatchAdminLoggingSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	body, ok := decodeAndValidateJSON[AdminLoggingSettingsPatchRequest](h, w, r)
	if !ok {
		return
	}
	level, ok := parseAdminLogLevel(body.Level)
	if !ok {
		writeValidationErrors(w, []ValidationErrorDetail{{
			Field:    "level",
			Location: "body",
			Message:  "must be one of: debug, info, warn, error",
		}})
		return
	}
	if h.logLevel != nil {
		h.logLevel.Set(level)
	}
	writeJSON(w, http.StatusOK, h.adminLoggingSettingsResponse())
}

func (h *Handlers) adminLoggingSettingsResponse() AdminLoggingSettingsResponse {
	if h.logLevel == nil {
		return AdminLoggingSettingsResponse{Level: logLevelName(slog.LevelInfo)}
	}
	return AdminLoggingSettingsResponse{Level: logLevelName(h.logLevel.Level())}
}

func parseAdminLogLevel(level string) (slog.Level, bool) {
	switch level {
	case logLevelDebug:
		return slog.LevelDebug, true
	case logLevelInfo:
		return slog.LevelInfo, true
	case logLevelWarn:
		return slog.LevelWarn, true
	case logLevelError:
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}

func logLevelName(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return logLevelDebug
	case slog.LevelInfo:
		return logLevelInfo
	case slog.LevelWarn:
		return logLevelWarn
	case slog.LevelError:
		return logLevelError
	default:
		return level.String()
	}
}
