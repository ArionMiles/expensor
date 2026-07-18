package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type llmProviderInfoJSON struct {
	Name         string            `json:"name"`
	DisplayName  string            `json:"display_name"`
	Description  string            `json:"description"`
	AuthType     string            `json:"auth_type"`
	Capabilities []llm.Capability  `json:"capabilities"`
	ConfigSchema json.RawMessage   `json:"config_schema,omitempty"`
	ModelOptions []llm.ModelOption `json:"model_options,omitempty"`
}

type llmProviderStatusJSON struct {
	Name              string          `json:"name"`
	Config            json.RawMessage `json:"config"`
	ConfigPresent     bool            `json:"config_present"`
	CredentialsStored bool            `json:"credentials_stored"`
	Active            bool            `json:"active"`
	Ready             bool            `json:"ready"`
}

type llmProviderConfigRequest struct {
	Config json.RawMessage `json:"config" validate:"required"`
}

type llmProviderCredentialsRequest struct {
	APIKey string `json:"api_key" validate:"required,no_control_chars"`
}

type llmProviderHealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ListLLMProviders handles GET /api/llm/providers.
//
// @Summary List LLM providers
// @Tags LLM
// @Produce json
// @Success 200 {array} LLMProviderInfoResponse
// @Router /llm/providers [get]
func (h *Handlers) ListLLMProviders(w http.ResponseWriter, _ *http.Request) {
	if h.llmRegistry == nil {
		writeJSON(w, http.StatusOK, []llmProviderInfoJSON{})
		return
	}
	providers := h.llmRegistry.ListProviders()
	out := make([]llmProviderInfoJSON, 0, len(providers))
	for _, provider := range providers {
		meta := provider.Metadata
		out = append(out, llmProviderInfoJSON{
			Name:         meta.Name,
			DisplayName:  meta.DisplayName,
			Description:  meta.Description,
			AuthType:     string(meta.Auth.Type),
			Capabilities: append([]llm.Capability(nil), meta.Capabilities...),
			ConfigSchema: cloneRawJSON(meta.ConfigSchema),
			ModelOptions: append([]llm.ModelOption(nil), meta.ModelOptions...),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// GetLLMProviderStatus handles GET /api/llm/providers/{name}/status.
//
// @Summary Get LLM provider status
// @Tags LLM
// @Produce json
// @Param name path string true "Provider name" example(openai)
// @Success 200 {object} LLMProviderStatusResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /llm/providers/{name}/status [get]
func (h *Handlers) GetLLMProviderStatus(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.llmProviderByPath(w, r)
	if !ok {
		return
	}
	status, err := h.llmProviderStatus(r.Context(), requestTenant(r), provider.Metadata.Name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// SaveLLMProviderConfig handles PUT /api/llm/providers/{name}/config.
//
// @Summary Save LLM provider config
// @Tags LLM
// @Accept json
// @Produce json
// @Param name path string true "Provider name" example(openai)
// @Param request body LLMProviderConfigSaveRequest true "Provider config"
// @Success 200 {object} StatusOnlyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /llm/providers/{name}/config [put]
func (h *Handlers) SaveLLMProviderConfig(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.llmProviderByPath(w, r)
	if !ok {
		return
	}
	body, ok := decodeAndValidateJSON[llmProviderConfigRequest](h, w, r)
	if !ok {
		return
	}
	if !json.Valid(body.Config) {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User("invalid provider config JSON")))
		return
	}
	if err := h.llmRuntimeStore.SetLLMProviderConfig(r.Context(), requestTenant(r), provider.Metadata.Name, body.Config); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// SaveLLMProviderCredentials handles PUT /api/llm/providers/{name}/credentials.
//
// @Summary Save LLM provider credentials
// @Tags LLM
// @Accept json
// @Produce json
// @Param name path string true "Provider name" example(openai)
// @Param request body LLMProviderCredentialsRequest true "Provider credentials"
// @Success 200 {object} StatusOnlyResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /llm/providers/{name}/credentials [put]
func (h *Handlers) SaveLLMProviderCredentials(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.llmProviderByPath(w, r)
	if !ok {
		return
	}
	body, ok := decodeAndValidateJSON[llmProviderCredentialsRequest](h, w, r)
	if !ok {
		return
	}
	credentials, err := json.Marshal(map[string]string{"api_key": strings.TrimSpace(body.APIKey)})
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.llmRuntimeStore.SetLLMProviderCredentials(r.Context(), requestTenant(r), provider.Metadata.Name, credentials); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// HealthCheckLLMProvider handles POST /api/llm/providers/{name}/healthcheck.
//
// @Summary Check LLM provider connectivity
// @Tags LLM
// @Produce json
// @Param name path string true "Provider name" example(openai)
// @Success 200 {object} LLMProviderHealthResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Router /llm/providers/{name}/healthcheck [post]
func (h *Handlers) HealthCheckLLMProvider(w http.ResponseWriter, r *http.Request) {
	provider, client, ok := h.llmProviderClientFromRuntime(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 75*time.Second)
	defer cancel()
	if err := client.HealthCheck(ctx); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, llmProviderHealthResponse{
		Status:  "ok",
		Message: provider.Metadata.DisplayName + " connection is healthy.",
	})
}

// ActivateLLMProvider handles POST /api/llm/providers/{name}/activate.
//
// @Summary Activate an LLM provider
// @Tags LLM
// @Produce json
// @Param name path string true "Provider name" example(openai)
// @Success 200 {object} StatusOnlyResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Router /llm/providers/{name}/activate [post]
func (h *Handlers) ActivateLLMProvider(w http.ResponseWriter, r *http.Request) {
	provider, client, ok := h.llmProviderClientFromRuntime(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 75*time.Second)
	defer cancel()
	if err := client.HealthCheck(ctx); err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.llmRuntimeStore.SetActiveLLMProvider(r.Context(), requestTenant(r), provider.Metadata.Name); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

// DisconnectLLMProvider handles DELETE /api/llm/providers/{name}.
//
// @Summary Disconnect an LLM provider
// @Tags LLM
// @Param name path string true "Provider name" example(openai)
// @Success 204
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /llm/providers/{name} [delete]
func (h *Handlers) DisconnectLLMProvider(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.llmProviderByPath(w, r)
	if !ok {
		return
	}
	if err := h.llmRuntimeStore.DeleteLLMProviderRuntime(r.Context(), requestTenant(r), provider.Metadata.Name); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) llmProviderByPath(w http.ResponseWriter, r *http.Request) (llm.Provider, bool) {
	if h.llmRegistry == nil {
		writeError(w, r, errors.E(errors.NotFound, errors.User("LLM providers are not configured")))
		return llm.Provider{}, false
	}
	name := r.PathValue("name")
	provider, err := h.llmRegistry.GetProvider(name)
	if err != nil {
		writeError(w, r, errors.E(errors.NotFound, errors.User("LLM provider not found"), err))
		return llm.Provider{}, false
	}
	return provider, true
}

func (h *Handlers) llmProviderStatus(ctx context.Context, tenant store.Tenant, provider string) (llmProviderStatusJSON, error) {
	config, configPresent, err := h.llmRuntimeStore.GetLLMProviderConfig(ctx, tenant, provider)
	if err != nil {
		return llmProviderStatusJSON{}, err
	}
	_, credentialsStored, err := h.llmRuntimeStore.GetLLMProviderCredentials(ctx, tenant, provider)
	if err != nil {
		return llmProviderStatusJSON{}, err
	}
	activeRuntime, activeFound, err := h.llmRuntimeStore.GetActiveLLMProviderRuntime(ctx, tenant)
	if err != nil {
		return llmProviderStatusJSON{}, err
	}
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	active := activeFound && activeRuntime.Provider == provider
	return llmProviderStatusJSON{
		Name:              provider,
		Config:            cloneRawJSON(config),
		ConfigPresent:     configPresent,
		CredentialsStored: credentialsStored,
		Active:            active,
		Ready:             active && credentialsStored,
	}, nil
}

func (h *Handlers) llmProviderClientFromRuntime(w http.ResponseWriter, r *http.Request) (llm.Provider, llm.Client, bool) {
	provider, ok := h.llmProviderByPath(w, r)
	if !ok {
		return llm.Provider{}, nil, false
	}
	tenant := requestTenant(r)
	config, _, err := h.llmRuntimeStore.GetLLMProviderConfig(r.Context(), tenant, provider.Metadata.Name)
	if err != nil {
		writeError(w, r, err)
		return llm.Provider{}, nil, false
	}
	credentials, found, err := h.llmRuntimeStore.GetLLMProviderCredentials(r.Context(), tenant, provider.Metadata.Name)
	if err != nil {
		writeError(w, r, err)
		return llm.Provider{}, nil, false
	}
	if !found {
		writeError(w, r, errors.E(errors.Conflict, errors.User("LLM provider credentials are not configured")))
		return llm.Provider{}, nil, false
	}
	client, err := provider.NewClient(llm.ClientConfig{
		Config:      cloneRawJSON(config),
		Credentials: cloneRawJSON(credentials),
	})
	if err != nil {
		writeError(w, r, errors.E(
			errors.Conflict,
			errors.User("LLM provider configuration is invalid."),
			err,
		))
		return llm.Provider{}, nil, false
	}
	client = llm.NewInstrumentedClient(
		client,
		provider.Metadata.Name,
		h.llmScope,
		h.logger.With("component", "llm"),
	)
	return provider, client, true
}

func cloneRawJSON(in []byte) json.RawMessage {
	if len(in) == 0 {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return json.RawMessage(out)
}
