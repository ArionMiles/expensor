package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// GetReaderConfig handles GET /api/providers/{name}/config.
// @Summary Get reader runtime config
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(thunderbird)
// @Success 200 {object} ProviderConfigResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /providers/{name}/config [get]
func (h *Handlers) GetReaderConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetProvider(name); err != nil {
		writeError(w, r, err)
		return
	}

	data, ok, err := h.readerRuntimeStore.GetReaderConfig(r.Context(), requestTenant(r), name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	//nolint:gosec // provider config is stored JSON returned with application/json content type
	_, _ = w.Write(data)
}

// SaveReaderConfig handles PUT /api/providers/{name}/config.
// @Summary Save reader runtime config
// @Tags Providers
// @Accept json
// @Produce json
// @Param name path string true "Provider name" example(thunderbird)
// @Param request body ProviderConfigRequest true "Provider config JSON"
// @Success 200 {object} ProviderConfigSaveResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /providers/{name}/config [put]
func (h *Handlers) SaveReaderConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetProvider(name); err != nil {
		writeError(w, r, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCredentialsSize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User("failed to read body"), err))
		return
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User("invalid JSON body"), err))
		return
	}

	if err := h.readerRuntimeStore.SetReaderConfig(r.Context(), requestTenant(r), name, json.RawMessage(body)); err != nil {
		writeError(w, r, err)
		return
	}

	h.queueReaderScanning(r.Context(), requestTenant(r), name)
	h.logger.Info("provider config saved", "reader", name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// ReaderStatus handles GET /api/providers/{name}/status.
// Returns overall readiness: credentials present, auth valid, config present.
// @Summary Get provider readiness status
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(thunderbird)
// @Success 200 {object} ProviderStatusResponse
// @Failure 404 {object} ErrorResponse
// @Router /providers/{name}/status [get]
func (h *Handlers) ReaderStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	provider, err := h.registry.GetProvider(name)
	if err != nil {
		writeError(w, r, err)
		return
	}

	type readerStatus struct {
		CredentialsUploaded bool             `json:"credentials_uploaded"`
		Authenticated       bool             `json:"authenticated"`
		ConfigPresent       bool             `json:"config_present"`
		AuthType            plugins.AuthType `json:"auth_type"`
		AuthState           string           `json:"auth_state"`
		Ready               bool             `json:"ready"`
	}

	meta := provider.Metadata
	st := readerStatus{AuthType: meta.Auth.Type}

	if meta.Auth.RequiresCredentialsUpload {
		_, ok, err := h.readerRuntimeStore.GetReaderSecret(r.Context(), requestTenant(r), name)
		if err != nil {
			writeError(w, r, err)
			return
		}
		st.CredentialsUploaded = ok
	} else {
		st.CredentialsUploaded = true
	}

	st.Authenticated, st.AuthState = h.resolveReaderAuthStatus(r.Context(), requestTenant(r), name, meta)

	if len(meta.ConfigSchema) == 0 {
		st.ConfigPresent = true
	} else {
		_, ok, err := h.readerRuntimeStore.GetReaderConfig(r.Context(), requestTenant(r), name)
		if err != nil {
			writeError(w, r, err)
			return
		}
		st.ConfigPresent = ok
	}

	st.Ready = st.CredentialsUploaded && st.Authenticated && st.ConfigPresent
	writeJSON(w, http.StatusOK, st)
}
