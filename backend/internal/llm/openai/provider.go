package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/llm"
)

const (
	ProviderName    = "openai"
	defaultBaseURL  = "https://api.openai.com/v1"
	defaultModel    = "gpt-5.4-mini"
	defaultTimeout  = 60 * time.Second
	maxErrorBodyLen = 800
)

type credentials struct {
	APIKey string `json:"api_key"`
}

type providerConfig struct {
	Model   string `json:"model"`
	BaseURL string `json:"base_url"`
}

type client struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// Provider returns the OpenAI API-backed LLM provider registration.
func Provider(modelOptions []llm.ModelOption) llm.Provider {
	return llm.Provider{
		Metadata: llm.ProviderMetadata{
			Name:        ProviderName,
			DisplayName: "OpenAI",
			Description: "Connect an OpenAI API key for usage-billed LLM workflows with structured outputs.",
			Auth: llm.AuthSpec{
				Type:     llm.AuthTypeAPIKey,
				Required: true,
			},
			ConfigSchema: json.RawMessage(`{
				"type":"object",
				"properties":{
					"model":{"type":"string","default":"gpt-5.4-mini"},
					"base_url":{"type":"string","default":"https://api.openai.com/v1"}
				}
			}`),
			Capabilities: []llm.Capability{
				llm.CapabilityTextGeneration,
				llm.CapabilityJSONSchema,
			},
			ModelOptions: append([]llm.ModelOption(nil), modelOptions...),
		},
		NewClient: NewClient,
	}
}

// NewClient builds an OpenAI API client from encrypted credentials and provider config.
func NewClient(input llm.ClientConfig) (llm.Client, error) {
	var creds credentials
	if len(input.Credentials) > 0 {
		if err := json.Unmarshal(input.Credentials, &creds); err != nil {
			return nil, fmt.Errorf("decoding OpenAI credentials: %w", err)
		}
	}
	if strings.TrimSpace(creds.APIKey) == "" {
		return nil, errors.New("OpenAI API key is not configured")
	}

	cfg := providerConfig{Model: defaultModel, BaseURL: defaultBaseURL}
	if len(input.Config) > 0 {
		if err := json.Unmarshal(input.Config, &cfg); err != nil {
			return nil, fmt.Errorf("decoding OpenAI config: %w", err)
		}
	}
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}

	return &client{
		apiKey:     strings.TrimSpace(creds.APIKey),
		model:      cfg.Model,
		baseURL:    cfg.BaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}, nil
}

func (c *client) HealthCheck(ctx context.Context) error {
	schema := json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"required":["ok"],
		"properties":{"ok":{"type":"boolean"}}
	}`)
	resp, err := c.Complete(ctx, llm.Request{
		Workflow:             "provider_setup",
		Purpose:              "healthcheck",
		RequiredCapabilities: []llm.Capability{llm.CapabilityTextGeneration, llm.CapabilityJSONSchema},
		MaxOutputTokens:      64,
		ResponseFormat: llm.ResponseFormat{
			Type:   llm.ResponseFormatJSONSchema,
			Name:   "openai_healthcheck",
			Strict: true,
			Schema: schema,
		},
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Return a JSON object confirming the connection health."},
			{Role: llm.RoleUser, Content: `Return {"ok":true}.`},
		},
	})
	if err != nil {
		return err
	}
	var out struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal([]byte(resp.Text), &out); err != nil || !out.OK {
		return errors.New("OpenAI healthcheck returned an invalid structured response")
	}
	return nil
}

func (c *client) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	payload, err := c.responsesPayload(req)
	if err != nil {
		return llm.Response{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return llm.Response{}, fmt.Errorf("building OpenAI request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return llm.Response{}, fmt.Errorf("building OpenAI request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return llm.Response{}, fmt.Errorf("calling OpenAI: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
	if err != nil {
		return llm.Response{}, fmt.Errorf("reading OpenAI response: %w", err)
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return llm.Response{}, openAIProviderError(httpResp.StatusCode, respBody)
	}

	var resp responsesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return llm.Response{}, fmt.Errorf("decoding OpenAI response: %w", err)
	}
	if resp.Error != nil {
		return llm.Response{}, &llm.ProviderError{
			Provider:   ProviderName,
			StatusCode: httpResp.StatusCode,
			Code:       resp.Error.Code,
			Message:    resp.Error.Message,
		}
	}

	text := strings.TrimSpace(resp.OutputText)
	if text == "" {
		text = strings.TrimSpace(resp.FirstOutputText())
	}
	if text == "" {
		return llm.Response{}, errors.New("OpenAI response did not include output text")
	}
	return llm.Response{
		Text:         text,
		Messages:     []llm.Message{{Role: llm.RoleAssistant, Content: text}},
		Usage:        resp.Usage.toLLMUsage(),
		FinishReason: resp.FinishReason(),
	}, nil
}

func (c *client) responsesPayload(req llm.Request) (responsesRequest, error) {
	payload := responsesRequest{
		Model:           c.model,
		Input:           make([]responsesInputItem, 0, len(req.Messages)),
		MaxOutputTokens: req.MaxOutputTokens,
	}
	store := false
	payload.Store = &store
	if req.Temperature != nil {
		payload.Temperature = req.Temperature
	}
	for _, msg := range req.Messages {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		payload.Input = append(payload.Input, responsesInputItem{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}
	if len(payload.Input) == 0 {
		return responsesRequest{}, errors.New("OpenAI request requires at least one non-empty message")
	}
	if req.ResponseFormat.Type != "" && req.ResponseFormat.Type != llm.ResponseFormatText {
		format, err := responseTextFormat(req.ResponseFormat)
		if err != nil {
			return responsesRequest{}, err
		}
		payload.Text = &responsesText{Format: format}
	}
	return payload, nil
}

func responseTextFormat(format llm.ResponseFormat) (responsesTextFormat, error) {
	switch format.Type {
	case llm.ResponseFormatJSONSchema:
		name := strings.TrimSpace(format.Name)
		if name == "" {
			name = "expensor_response"
		}
		if len(format.Schema) == 0 || !json.Valid(format.Schema) {
			return responsesTextFormat{}, errors.New("json_schema response format requires a valid schema")
		}
		var schema map[string]any
		if err := json.Unmarshal(format.Schema, &schema); err != nil {
			return responsesTextFormat{}, fmt.Errorf("decoding json_schema response format: %w", err)
		}
		return responsesTextFormat{
			Type:   "json_schema",
			Name:   name,
			Strict: format.Strict,
			Schema: schema,
		}, nil
	case llm.ResponseFormatJSONObject:
		return responsesTextFormat{Type: "json_object"}, nil
	default:
		return responsesTextFormat{}, fmt.Errorf("unsupported OpenAI response format %q", format.Type)
	}
}

func openAIProviderError(status int, body []byte) error {
	var parsed struct {
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != nil {
		code := strings.TrimSpace(parsed.Error.Code)
		if code == "" {
			code = strings.TrimSpace(parsed.Error.Type)
		}
		return &llm.ProviderError{
			Provider:   ProviderName,
			StatusCode: status,
			Code:       code,
			Message:    parsed.Error.Message,
		}
	}
	message := strings.TrimSpace(string(body))
	if len(message) > maxErrorBodyLen {
		message = message[:maxErrorBodyLen] + "..."
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return &llm.ProviderError{Provider: ProviderName, StatusCode: status, Message: message}
}
