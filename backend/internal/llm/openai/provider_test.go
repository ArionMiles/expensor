package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func openAIMetadata(model, baseURL string) llm.ProviderMetadata {
	return llm.ProviderMetadata{
		DisplayName: "OpenAI",
		ConfigSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"model":{"type":"string","default":"` + model + `"},
				"base_url":{"type":"string","default":"` + baseURL + `"}
			}
		}`),
		ModelOptions: []llm.ModelOption{{ID: model, DisplayName: model, Quality: "High", Cost: "Medium", Recommended: true}},
	}
}

func openAIProviderForTest(t *testing.T, model, baseURL string) llm.Provider {
	t.Helper()
	provider, err := Provider(openAIMetadata(model, baseURL))
	if err != nil {
		t.Fatalf("Provider() error = %v", err)
	}
	return provider
}

func TestProviderRequiresAPIKeyAndAppliesCatalogDefaults(t *testing.T) {
	provider := openAIProviderForTest(t, "gpt-catalog", "https://catalog.openai.example/v1")
	_, err := provider.NewClient(llm.ClientConfig{})
	if err == nil {
		t.Fatal("NewClient error = nil, want missing API key error")
	}

	got, err := provider.NewClient(llm.ClientConfig{Credentials: []byte(`{"api_key":" sk-test "}`)})
	if err != nil {
		t.Fatalf("NewClient error = %v", err)
	}
	client, ok := got.(*client)
	if !ok {
		t.Fatalf("client type = %T, want *client", got)
	}
	if client.apiKey != "sk-test" || client.model != "gpt-catalog" || client.baseURL != "https://catalog.openai.example/v1" {
		t.Fatalf("client = %+v, want trimmed key and default model/base URL", client)
	}
	if provider.Metadata.Name != ProviderName || len(provider.Metadata.Capabilities) != 2 ||
		provider.Metadata.Capabilities[0] != llm.CapabilityTextGeneration ||
		provider.Metadata.Capabilities[1] != llm.CapabilityJSONSchema {
		t.Fatalf("provider metadata = %#v, want OpenAI implementation capabilities", provider.Metadata)
	}
}

func TestCompleteUsesResponsesAPIWithStructuredOutputs(t *testing.T) {
	var captured responsesRequest
	var authHeader string
	var requestPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		authHeader = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("Decode request body error = %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output_text":"{\"amount\":\"10\"}",
			"usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10},
			"output":[{"finish_reason":"stop","content":[{"type":"output_text","text":"fallback"}]}]
		}`))
	}))
	defer server.Close()

	config, err := json.Marshal(providerConfig{Model: "gpt-test", BaseURL: server.URL + "/"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	provider := openAIProviderForTest(t, "gpt-default", "https://catalog.openai.example/v1")
	got, err := provider.NewClient(llm.ClientConfig{
		Config:      config,
		Credentials: []byte(`{"api_key":"sk-test"}`),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	temperature := 0.2
	resp, err := got.Complete(context.Background(), llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Return JSON."},
			{Role: llm.RoleUser, Content: "extract amount"},
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

	if requestPath != "/responses" || authHeader != "Bearer sk-test" {
		t.Fatalf("request path/auth = %q/%q, want /responses with bearer token", requestPath, authHeader)
	}
	if captured.Model != "gpt-test" || captured.MaxOutputTokens != 128 {
		t.Fatalf("captured model/tokens = %q/%d", captured.Model, captured.MaxOutputTokens)
	}
	if captured.Store == nil || *captured.Store {
		t.Fatalf("captured store = %v, want false", captured.Store)
	}
	if captured.Temperature == nil || *captured.Temperature != temperature {
		t.Fatalf("captured temperature = %v, want %v", captured.Temperature, temperature)
	}
	if len(captured.Input) != 2 || captured.Input[0].Role != string(llm.RoleSystem) || captured.Input[1].Role != string(llm.RoleUser) {
		t.Fatalf("captured input = %#v, want system and user messages", captured.Input)
	}
	if captured.Text == nil || captured.Text.Format.Type != "json_schema" || captured.Text.Format.Name != "amount_result" || !captured.Text.Format.Strict {
		t.Fatalf("captured text format = %#v, want strict json_schema format", captured.Text)
	}
	if captured.Text.Format.Schema["type"] != "object" {
		t.Fatalf("captured schema = %#v, want object schema", captured.Text.Format.Schema)
	}
	if resp.Text != `{"amount":"10"}` || resp.Usage.TotalTokens != 10 || resp.FinishReason != "stop" {
		t.Fatalf("response = %+v, want parsed output text, usage and finish reason", resp)
	}
}

func TestCompleteMapsOpenAIErrorResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{
			"error":{
				"message":"You exceeded your current quota.",
				"type":"insufficient_quota",
				"code":"insufficient_quota"
			}
		}`))
	}))
	defer server.Close()

	config, err := json.Marshal(providerConfig{Model: "gpt-test", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	provider := openAIProviderForTest(t, "gpt-default", "https://catalog.openai.example/v1")
	got, err := provider.NewClient(llm.ClientConfig{
		Config:      config,
		Credentials: []byte(`{"api_key":"sk-test"}`),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = got.Complete(context.Background(), llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if errors.WhatKind(err) != errors.ResourceExhausted {
		t.Fatalf("Complete() error = %v, want resource exhausted", err)
	}
	if errors.UserMsg(err) != "OpenAI API quota is unavailable. Add billing credits or choose another LLM provider." {
		t.Fatalf("user message = %q", errors.UserMsg(err))
	}
}
