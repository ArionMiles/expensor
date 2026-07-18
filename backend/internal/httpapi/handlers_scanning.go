package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// GetScanningSettings handles GET /api/scanning/settings.
// @Summary Get tenant scanning settings
// @Tags Scanning
// @Produce json
// @Success 200 {object} ScanningSettingsResponse
// @Failure 500 {object} ErrorResponse
// @Router /scanning/settings [get]
func (h *Handlers) GetScanningSettings(w http.ResponseWriter, r *http.Request) {
	state, err := h.scanningStore.GetScanningState(r.Context(), requestTenant(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ScanningSettingsResponse{
		ActiveReader: state.ActiveReader,
		Enabled:      state.Enabled,
	})
}

// PatchScanningSettings handles PATCH /api/scanning/settings.
// @Summary Update tenant scanning settings
// @Tags Scanning
// @Accept json
// @Produce json
// @Param request body ScanningSettingsPatchRequest true "Scanning settings patch"
// @Success 200 {object} ScanningSettingsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /scanning/settings [patch]
func (h *Handlers) PatchScanningSettings(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeAndValidateJSON[ScanningSettingsPatchRequest](h, w, r)
	if !ok {
		return
	}
	tenant := requestTenant(r)
	if body.ActiveReader != nil {
		reader := strings.TrimSpace(*body.ActiveReader)
		if reader != "" {
			if _, err := h.registry.GetProvider(reader); err != nil {
				writeError(w, r, errors.E(errors.InvalidArgument, errors.User(fmt.Sprintf("reader %q not found", reader)), err))
				return
			}
			if err := h.scanningStore.SetActiveScanningReader(r.Context(), tenant, reader); err != nil {
				writeError(w, r, err)
				return
			}
		} else if err := h.scanningStore.ClearActiveScanningReader(r.Context(), tenant); err != nil {
			writeError(w, r, err)
			return
		}
	}
	if body.Enabled != nil {
		if err := h.applyScanningEnabled(r.Context(), tenant, *body.Enabled); err != nil {
			writeError(w, r, err)
			return
		}
	}
	h.GetScanningSettings(w, r)
}

// GetScanningStatus handles GET /api/scanning/status.
// @Summary Get tenant scanning status
// @Tags Scanning
// @Produce json
// @Success 200 {object} ScanningStatusResponse
// @Failure 500 {object} ErrorResponse
// @Router /scanning/status [get]
func (h *Handlers) GetScanningStatus(w http.ResponseWriter, r *http.Request) {
	state, err := h.scanningStore.GetScanningState(r.Context(), requestTenant(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, scanningStatusResponse(state, false))
}

// CreateScanningRescan handles POST /api/scanning/rescans.
// @Summary Create a tenant scanning rescan request
// @Tags Scanning
// @Accept json
// @Produce json
// @Param request body DaemonReaderRequest true "Rescan request"
// @Success 202 {object} StatusOnlyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 501 {object} ErrorResponse
// @Router /scanning/rescans [post]
func (h *Handlers) CreateScanningRescan(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeAndValidateJSON[DaemonReaderRequest](h, w, r)
	if !ok {
		return
	}
	if _, err := h.registry.GetProvider(body.Reader); err != nil {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User(fmt.Sprintf("reader %q not found", body.Reader)), err))
		return
	}
	if h.daemon == nil {
		writeError(w, r, errors.E(errors.Unimplemented, errors.User("rescan not configured")))
		return
	}
	h.daemon.Rescan(daemon.RunRequest{Tenant: requestTenant(r), Reader: body.Reader})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "rescanning"})
}

// GetAdminScanningSettings handles GET /api/admin/scanning/settings.
// @Summary Get global scanning settings
// @Tags Admin
// @Produce json
// @Success 200 {object} AdminScanningSettingsResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/scanning/settings [get]
func (h *Handlers) GetAdminScanningSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.scanningStore.GetSchedulerConfig(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, adminScanningSettingsResponse(cfg))
}

// PatchAdminScanningSettings handles PATCH /api/admin/scanning/settings.
// @Summary Update global scanning settings
// @Tags Admin
// @Accept json
// @Produce json
// @Param request body AdminScanningSettingsPatchRequest true "Global scanning settings patch"
// @Success 200 {object} AdminScanningSettingsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/scanning/settings [patch]
func (h *Handlers) PatchAdminScanningSettings(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeAndValidateJSON[AdminScanningSettingsPatchRequest](h, w, r)
	if !ok {
		return
	}
	cfg, err := h.scanningStore.PatchSchedulerConfig(r.Context(), store.SchedulerConfigPatch{
		MaxConcurrentScans: body.MaxConcurrentScans,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, adminScanningSettingsResponse(cfg))
}

func (h *Handlers) applyScanningEnabled(ctx context.Context, tenant store.Tenant, enabled bool) error {
	if enabled {
		return h.scanningStore.SetScanningEnabled(ctx, tenant, true)
	}
	return h.scanningStore.SetScanningEnabled(ctx, tenant, false)
}

func scanningStatusResponse(state store.TenantScanningState, includeTenant bool) ScanningStatusResponse {
	tenantID := ""
	if includeTenant {
		tenantID = state.TenantID
	}
	return ScanningStatusResponse{
		TenantID:      tenantID,
		ActiveReader:  state.ActiveReader,
		Enabled:       state.Enabled,
		State:         string(state.State),
		ReasonCode:    string(state.ReasonCode),
		PublicMessage: state.PublicMessage,
		LastStartedAt: state.LastStartedAt,
		LastStoppedAt: state.LastStoppedAt,
		LastFailedAt:  state.LastFailedAt,
		NextRetryAt:   state.NextRetryAt,
		RetryCount:    state.RetryCount,
		UpdatedAt:     state.UpdatedAt,
	}
}

func adminScanningSettingsResponse(cfg store.SchedulerConfig) AdminScanningSettingsResponse {
	return AdminScanningSettingsResponse{MaxConcurrentScans: cfg.MaxConcurrentScans, UpdatedAt: cfg.UpdatedAt}
}

func (h *Handlers) queueReaderScanning(ctx context.Context, tenant store.Tenant, reader string) {
	if err := h.scanningStore.SetActiveScanningReader(ctx, tenant, reader); err != nil {
		h.logger.Warn("failed to queue scanning after reader setup", "reader", reader, "error", err)
	}
}

func intPointer(value int) *int {
	return &value
}
