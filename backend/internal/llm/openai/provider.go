package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	ProviderName   = "openai"
	defaultBaseURL = "https://api.openai.com/v1"
	defaultModel   = "gpt-5.4-mini"
	defaultTimeout = 60 * time.Second
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
	const op = "llm.openai.NewClient"

	var creds credentials
	if len(input.Credentials) > 0 {
		if err := json.Unmarshal(input.Credentials, &creds); err != nil {
			return nil, errors.E(op, errors.InvalidInput, "decoding OpenAI credentials", err)
		}
	}
	if strings.TrimSpace(creds.APIKey) == "" {
		return nil, errors.E(op, errors.FailedPrecondition, "OpenAI API key is not configured")
	}

	cfg := providerConfig{Model: defaultModel, BaseURL: defaultBaseURL}
	if len(input.Config) > 0 {
		if err := json.Unmarshal(input.Config, &cfg); err != nil {
			return nil, errors.E(op, errors.InvalidInput, "decoding OpenAI config", err)
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
	const op = "llm.openai.HealthCheck"

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
		return errors.E(op, err)
	}
	var out struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal([]byte(resp.Text), &out); err != nil || !out.OK {
		return errors.E(op, errors.BadGateway, "OpenAI healthcheck returned an invalid structured response")
	}
	return nil
}

func (c *client) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	const op = "llm.openai.Complete"

	payload, err := c.responsesPayload(req)
	if err != nil {
		return llm.Response{}, errors.E(op, err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return llm.Response{}, errors.E(op, errors.Internal, "building OpenAI request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return llm.Response{}, errors.E(op, errors.Internal, "building OpenAI request", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return llm.Response{}, errors.E(op, errors.Unavailable, "calling OpenAI", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
	if err != nil {
		return llm.Response{}, errors.E(op, errors.BadGateway, "reading OpenAI response", err)
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return llm.Response{}, errors.E(op, openAIProviderError(httpResp.StatusCode, respBody))
	}

	var resp responsesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return llm.Response{}, errors.E(op, errors.BadGateway, "decoding OpenAI response", err)
	}
	if resp.Error != nil {
		return llm.Response{}, errors.E(op, openAIProviderFailure(httpResp.StatusCode, resp.Error.Code, resp.Error.Type))
	}

	text := strings.TrimSpace(resp.OutputText)
	if text == "" {
		text = strings.TrimSpace(resp.FirstOutputText())
	}
	if text == "" {
		return llm.Response{}, errors.E(op, errors.BadGateway, "OpenAI response did not include output text")
	}
	return llm.Response{
		Text:         text,
		Messages:     []llm.Message{{Role: llm.RoleAssistant, Content: text}},
		Usage:        resp.Usage.toLLMUsage(),
		FinishReason: resp.FinishReason(),
	}, nil
}

func (c *client) responsesPayload(req llm.Request) (responsesRequest, error) {
	const op = "llm.openai.responsesPayload"

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
		return responsesRequest{}, errors.E(op, errors.InvalidInput, "OpenAI request requires at least one non-empty message")
	}
	if req.ResponseFormat.Type != "" && req.ResponseFormat.Type != llm.ResponseFormatText {
		format, err := responseTextFormat(req.ResponseFormat)
		if err != nil {
			return responsesRequest{}, errors.E(op, err)
		}
		payload.Text = &responsesText{Format: format}
	}
	return payload, nil
}

func responseTextFormat(format llm.ResponseFormat) (responsesTextFormat, error) {
	const op = "llm.openai.responseTextFormat"

	switch format.Type {
	case llm.ResponseFormatJSONSchema:
		name := strings.TrimSpace(format.Name)
		if name == "" {
			name = "expensor_response"
		}
		if len(format.Schema) == 0 || !json.Valid(format.Schema) {
			return responsesTextFormat{}, errors.E(op, errors.InvalidInput, "json_schema response format requires a valid schema")
		}
		var schema map[string]any
		if err := json.Unmarshal(format.Schema, &schema); err != nil {
			return responsesTextFormat{}, errors.E(op, errors.InvalidInput, "decoding json_schema response format", err)
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
		return responsesTextFormat{}, errors.E(op, errors.InvalidInput, fmt.Sprintf("unsupported OpenAI response format %q", format.Type))
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
		return openAIProviderFailure(status, parsed.Error.Code, parsed.Error.Type)
	}
	return openAIProviderFailure(status, "", "")
}

func openAIProviderFailure(status int, code, typ string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		code = strings.TrimSpace(typ)
	}
	switch code {
	case "invalid_api_key":
		return errors.E(errors.Unauthenticated, errors.User("OpenAI API key was rejected. Check the key and try again."))
	case "insufficient_quota":
		return errors.E(
			errors.ResourceExhausted,
			errors.User("OpenAI API quota is unavailable. Add billing credits or choose another LLM provider."),
		)
	case "rate_limit_exceeded":
		return errors.E(errors.ResourceExhausted, errors.User("OpenAI rate limit exceeded. Wait a moment and try again."))
	}

	switch status {
	case http.StatusUnauthorized:
		return errors.E(errors.Unauthenticated, errors.User("OpenAI API key was rejected. Check the key and try again."))
	case http.StatusTooManyRequests:
		return errors.E(errors.ResourceExhausted, errors.User("LLM provider request was rate limited. Wait a moment and try again."))
	default:
		return errors.E(errors.BadGateway, errors.User("LLM provider request failed."))
	}
}
