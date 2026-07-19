package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func geminiMetadata(model string) llm.ProviderMetadata {
	return llm.ProviderMetadata{
		DisplayName: "Gemini",
		ConfigSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"model":{"type":"string","default":"` + model + `"}}
		}`),
		ModelOptions: []llm.ModelOption{{ID: model, DisplayName: model, Quality: "High", Cost: "Medium", Recommended: true}},
	}
}

func geminiProviderForTest(t *testing.T, model string) llm.Provider {
	t.Helper()
	provider, err := Provider(geminiMetadata(model))
	if err != nil {
		t.Fatalf("Provider() error = %v", err)
	}
	return provider
}

func geminiClientForServer(t *testing.T, server *httptest.Server, model string) llm.Client {
	t.Helper()
	provider := geminiProviderForTest(t, model)
	created, err := provider.NewClient(llm.ClientConfig{
		Config:      json.RawMessage(`{"model":"` + model + `"}`),
		Credentials: []byte(`{"api_key":" gemini-key "}`),
	})
	if err != nil {
		t.Fatalf("NewClient error = %v", err)
	}
	client, ok := created.(*client)
	if !ok {
		t.Fatalf("client type = %T, want *client", created)
	}
	client.baseURL = server.URL
	client.httpClient = server.Client()
	return client
}

func TestProviderRequiresAPIKeyAndAppliesCatalogDefaults(t *testing.T) {
	provider := geminiProviderForTest(t, "gemini-catalog")
	if _, err := provider.NewClient(llm.ClientConfig{}); err == nil {
		t.Fatal("NewClient error = nil, want missing API key error")
	}

	got, err := provider.NewClient(llm.ClientConfig{Credentials: []byte(`{"api_key":" gemini-key "}`)})
	if err != nil {
		t.Fatalf("NewClient error = %v", err)
	}
	client, ok := got.(*client)
	if !ok {
		t.Fatalf("client type = %T, want *client", got)
	}
	if client.apiKey != "gemini-key" || client.model != "gemini-catalog" || client.baseURL != defaultBaseURL {
		t.Fatalf("client = %+v, want trimmed key and catalog model", client)
	}
	if provider.Metadata.Name != ProviderName || len(provider.Metadata.Capabilities) != 2 ||
		provider.Metadata.Capabilities[0] != llm.CapabilityTextGeneration ||
		provider.Metadata.Capabilities[1] != llm.CapabilityJSONSchema {
		t.Fatalf("provider metadata = %#v, want Gemini implementation capabilities", provider.Metadata)
	}
}

func TestProviderRejectsUndeclaredBaseURLConfig(t *testing.T) {
	provider := geminiProviderForTest(t, flashModel)
	_, err := provider.NewClient(llm.ClientConfig{
		Config:      json.RawMessage(`{"model":"gemini-3.5-flash","base_url":"https://attacker.invalid"}`),
		Credentials: json.RawMessage(`{"api_key":"gemini-key"}`),
	})
	if errors.WhatKind(err) != errors.InvalidInput || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("NewClient error = %v, want invalid input for base_url", err)
	}
}

func TestCompleteUsesInteractionsAPIWithStructuredOutputs(t *testing.T) {
	var captured interactionsRequest
	var requestPath, apiKey, authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		apiKey = r.Header.Get("x-goog-api-key")
		authorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("Decode request body error = %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"completed",
			"steps":[{"type":"model_output","content":[{"type":"text","text":"{\"amount\":\"10\"}"}]}],
			"usage":{"total_input_tokens":7,"total_output_tokens":3,"total_tokens":10}
		}`))
	}))
	defer server.Close()

	client := geminiClientForServer(t, server, flashModel)
	temperature := 0.2
	response, err := client.Complete(context.Background(), llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Return JSON."},
			{Role: llm.RoleUser, Content: "extract amount"},
			{Role: llm.RoleUser, Content: "from this sample"},
		},
		MaxOutputTokens: 128,
		Temperature:     &temperature,
		ResponseFormat: llm.ResponseFormat{
			Type:   llm.ResponseFormatJSONSchema,
			Name:   "amount_result",
			Strict: true,
			Schema: json.RawMessage(`{
				"type":"object",
				"additionalProperties":false,
				"required":["amount"],
				"properties":{"amount":{"type":"string"}}
			}`),
		},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if requestPath != "/interactions" || apiKey != "gemini-key" || authorization != "" {
		t.Fatalf("request path/key/auth = %q/%q/%q", requestPath, apiKey, authorization)
	}
	if captured.Model != flashModel || captured.Input != "extract amount\n\nfrom this sample" || captured.SystemInstruction != "Return JSON." {
		t.Fatalf("captured input = %#v", captured)
	}
	if captured.Store == nil || *captured.Store {
		t.Fatalf("captured store = %v, want false", captured.Store)
	}
	if captured.GenerationConfig == nil || captured.GenerationConfig.MaxOutputTokens != 128 ||
		captured.GenerationConfig.Temperature == nil || *captured.GenerationConfig.Temperature != temperature ||
		captured.GenerationConfig.ThinkingLevel != "low" {
		t.Fatalf("generation config = %#v", captured.GenerationConfig)
	}
	if captured.ResponseFormat == nil || captured.ResponseFormat.Type != "text" ||
		captured.ResponseFormat.MIMEType != "application/json" || captured.ResponseFormat.Schema["type"] != "object" {
		t.Fatalf("response format = %#v", captured.ResponseFormat)
	}
	if response.Text != `{"amount":"10"}` || response.Usage.TotalTokens != 10 || response.FinishReason != "completed" {
		t.Fatalf("response = %#v", response)
	}
}

func TestHealthCheckUsesMinimalThinking(t *testing.T) {
	var captured interactionsRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("Decode request body error = %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"completed",
			"steps":[{"type":"model_output","content":[{"type":"text","text":"{\"ok\":true}"}]}]
		}`))
	}))
	defer server.Close()

	client := geminiClientForServer(t, server, flashLiteModel)
	if err := client.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if captured.GenerationConfig == nil || captured.GenerationConfig.ThinkingLevel != "minimal" {
		t.Fatalf("generation config = %#v, want minimal thinking", captured.GenerationConfig)
	}
}

func TestInteractionsPayloadValidatesUnsupportedInputs(t *testing.T) {
	client := &client{model: flashModel}
	tests := []struct {
		name string
		req  llm.Request
	}{
		{name: "tools", req: llm.Request{Tools: []llm.Tool{{Name: "lookup"}}, Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}}},
		{name: "assistant role", req: llm.Request{Messages: []llm.Message{{Role: llm.RoleAssistant, Content: "hello"}}}},
		{name: "missing user message", req: llm.Request{Messages: []llm.Message{{Role: llm.RoleSystem, Content: "hello"}}}},
		{name: "invalid schema", req: llm.Request{
			Messages:       []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
			ResponseFormat: llm.ResponseFormat{Type: llm.ResponseFormatJSONSchema, Schema: json.RawMessage(`{"bad"`)},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := client.interactionsPayload(tc.req); errors.WhatKind(err) != errors.InvalidInput {
				t.Fatalf("interactionsPayload() error = %v, want invalid input", err)
			}
		})
	}
}

func TestInteractionsPayloadDoesNotSetThinkingForUnknownModels(t *testing.T) {
	client := &client{model: "custom-model"}
	payload, err := client.interactionsPayload(llm.Request{
		Messages:       []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
		ResponseFormat: llm.ResponseFormat{Type: llm.ResponseFormatJSONObject},
	})
	if err != nil {
		t.Fatalf("interactionsPayload() error = %v", err)
	}
	if payload.GenerationConfig != nil {
		t.Fatalf("generation config = %#v, want omitted for unknown model", payload.GenerationConfig)
	}
}

func TestCompleteMapsGeminiErrorsWithoutExposingProviderBody(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantKind   errors.Kind
		wantUser   string
	}{
		{
			name:       "invalid key",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"message":"secret key detail","status":"UNAUTHENTICATED","details":[{"reason":"API_KEY_INVALID"}]}}`,
			wantKind:   errors.Unauthenticated,
			wantUser:   "Gemini API key was rejected. Check the key and try again.",
		},
		{
			name:       "quota",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":{"message":"private quota detail","status":"RESOURCE_EXHAUSTED"}}`,
			wantKind:   errors.ResourceExhausted,
			wantUser:   "Gemini quota or rate limit was exceeded. Wait a moment or check billing.",
		},
		{
			name:       "model unavailable",
			statusCode: http.StatusNotFound,
			body:       `{"error":{"message":"private model detail","status":"NOT_FOUND"}}`,
			wantKind:   errors.Conflict,
			wantUser:   "Gemini model is unavailable. Choose another model and try again.",
		},
		{
			name:       "provider unavailable",
			statusCode: http.StatusBadGateway,
			body:       `{"error":{"message":"private outage detail","status":"INTERNAL"}}`,
			wantKind:   errors.BadGateway,
			wantUser:   "Gemini is temporarily unavailable. Try again shortly.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			client := geminiClientForServer(t, server, flashModel)
			_, err := client.Complete(context.Background(), llm.Request{
				Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
			})
			if errors.WhatKind(err) != tc.wantKind || errors.UserMsg(err) != tc.wantUser {
				t.Fatalf("Complete() error = %v user=%q", err, errors.UserMsg(err))
			}
			if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("Complete() error exposed provider body: %v", err)
			}
		})
	}
}
