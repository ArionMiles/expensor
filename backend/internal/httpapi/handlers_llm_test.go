package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestLLMProviderLifecycle(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	h.llmRegistry = testLLMProvider(t, testLLMClient{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})

	listReq := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/llm/providers", nil)
	listRR := httptest.NewRecorder()
	h.ListLLMProviders(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listRR.Code, listRR.Body.String())
	}
	var providers []llmProviderInfoJSON
	decodeJSON(t, listRR.Body.String(), &providers)
	if len(providers) != 1 || providers[0].Name != "openai" || len(providers[0].ModelOptions) != 1 {
		t.Fatalf("providers = %+v, want OpenAI with model options", providers)
	}
	if providers[0].APIKeyURL != "https://platform.openai.com/api-keys" ||
		providers[0].APIKeyLinkText != "OpenAI dashboard" ||
		providers[0].DataUse.Mode != llm.DataUseNoTrainingByDefault ||
		providers[0].DataUse.PolicyURL == "" {
		t.Fatalf("provider metadata = %+v, want API key and data-use metadata", providers[0])
	}

	configReq := httptest.NewRequestWithContext(
		ctx,
		http.MethodPut,
		"/api/llm/providers/openai/config",
		strings.NewReader(`{"config":{"model":"gpt-5.4-mini","base_url":"https://api.openai.com/v1"}}`),
	)
	configReq.SetPathValue("name", "openai")
	configRR := httptest.NewRecorder()
	h.SaveLLMProviderConfig(configRR, configReq)
	if configRR.Code != http.StatusOK {
		t.Fatalf("save config status = %d body=%s", configRR.Code, configRR.Body.String())
	}
	if string(ms.llmProviderConfigs["tenant-a/openai"]) != `{"model":"gpt-5.4-mini","base_url":"https://api.openai.com/v1"}` {
		t.Fatalf("stored config = %s", ms.llmProviderConfigs["tenant-a/openai"])
	}

	credentialsReq := httptest.NewRequestWithContext(
		ctx,
		http.MethodPut,
		"/api/llm/providers/openai/credentials",
		strings.NewReader(`{"api_key":"  sk-test  "}`),
	)
	credentialsReq.SetPathValue("name", "openai")
	credentialsRR := httptest.NewRecorder()
	h.SaveLLMProviderCredentials(credentialsRR, credentialsReq)
	if credentialsRR.Code != http.StatusOK {
		t.Fatalf("save credentials status = %d body=%s", credentialsRR.Code, credentialsRR.Body.String())
	}
	if string(ms.llmProviderCredentials["tenant-a/openai"]) != `{"api_key":"sk-test"}` {
		t.Fatalf("stored credentials = %s", ms.llmProviderCredentials["tenant-a/openai"])
	}

	activateReq := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/llm/providers/openai/activate", nil)
	activateReq.SetPathValue("name", "openai")
	activateRR := httptest.NewRecorder()
	h.ActivateLLMProvider(activateRR, activateReq)
	if activateRR.Code != http.StatusOK {
		t.Fatalf("activate status = %d body=%s", activateRR.Code, activateRR.Body.String())
	}
	if ms.activeLLMProvider != "openai" {
		t.Fatalf("active provider = %q, want openai", ms.activeLLMProvider)
	}

	statusReq := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/llm/providers/openai/status", nil)
	statusReq.SetPathValue("name", "openai")
	statusRR := httptest.NewRecorder()
	h.GetLLMProviderStatus(statusRR, statusReq)
	if statusRR.Code != http.StatusOK {
		t.Fatalf("status response = %d body=%s", statusRR.Code, statusRR.Body.String())
	}
	var status llmProviderStatusJSON
	decodeJSON(t, statusRR.Body.String(), &status)
	if !status.ConfigPresent || !status.CredentialsStored || !status.Active || !status.Ready {
		t.Fatalf("status = %+v, want configured active ready provider", status)
	}

	disconnectReq := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/llm/providers/openai", nil)
	disconnectReq.SetPathValue("name", "openai")
	disconnectRR := httptest.NewRecorder()
	h.DisconnectLLMProvider(disconnectRR, disconnectReq)
	if disconnectRR.Code != http.StatusNoContent {
		t.Fatalf("disconnect status = %d body=%s", disconnectRR.Code, disconnectRR.Body.String())
	}
	if _, ok := ms.llmProviderConfigs["tenant-a/openai"]; ok {
		t.Fatal("config was not removed")
	}
	if _, ok := ms.llmProviderCredentials["tenant-a/openai"]; ok {
		t.Fatal("credentials were not removed")
	}
	if ms.activeLLMProvider != "" {
		t.Fatalf("active provider = %q, want cleared", ms.activeLLMProvider)
	}
}

func TestSaveLLMProviderConfigRejectsUndeclaredFields(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	h.llmRegistry = testLLMProvider(t, testLLMClient{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPut,
		"/api/llm/providers/openai/config",
		strings.NewReader(`{"config":{"model":"gpt-5.4-mini","redirect_url":"https://attacker.invalid"}}`),
	)
	req.SetPathValue("name", "openai")
	rr := httptest.NewRecorder()

	h.SaveLLMProviderConfig(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s, want 422", rr.Code, rr.Body.String())
	}
	if len(ms.llmProviderConfigs) != 0 {
		t.Fatalf("stored configs = %#v, want none", ms.llmProviderConfigs)
	}
}

func TestActivateLLMProviderMapsQuotaErrors(t *testing.T) {
	ms := &mockStore{
		llmProviderConfigs:     map[string]json.RawMessage{"tenant-a/openai": json.RawMessage(`{"model":"gpt-5.4-mini"}`)},
		llmProviderCredentials: map[string][]byte{"tenant-a/openai": []byte(`{"api_key":"sk-test"}`)},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	h.llmRegistry = testLLMProvider(t, testLLMClient{healthErr: errors.E(
		errors.ResourceExhausted,
		errors.User("OpenAI API quota is unavailable. Add billing credits or choose another LLM provider."),
		"raw provider quota message",
	)})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/llm/providers/openai/activate", nil)
	req.SetPathValue("name", "openai")
	rr := httptest.NewRecorder()

	h.ActivateLLMProvider(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if ms.activeLLMProvider != "" {
		t.Fatalf("active provider = %q, want not activated", ms.activeLLMProvider)
	}
	if !strings.Contains(rr.Body.String(), "OpenAI API quota is unavailable") || strings.Contains(rr.Body.String(), "raw provider quota message") {
		t.Fatalf("error body = %s, want friendly quota message", rr.Body.String())
	}
}

func TestActivateLLMProviderDoesNotExposeClientConstructionError(t *testing.T) {
	ms := &mockStore{
		llmProviderConfigs:     map[string]json.RawMessage{"tenant-a/openai": json.RawMessage(`{"model":"gpt-5.4-mini"}`)},
		llmProviderCredentials: map[string][]byte{"tenant-a/openai": []byte(`{"api_key":"sk-test"}`)},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	h.llmRegistry = testLLMProviderWithFactory(t, func(llm.ClientConfig) (llm.Client, error) {
		return nil, errors.E(errors.FailedPrecondition, "raw credential parsing detail")
	})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/llm/providers/openai/activate", nil)
	req.SetPathValue("name", "openai")
	rr := httptest.NewRecorder()

	h.ActivateLLMProvider(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "LLM provider configuration is invalid.") || strings.Contains(rr.Body.String(), "raw credential parsing detail") {
		t.Fatalf("error body = %s, want safe client construction error", rr.Body.String())
	}
}
