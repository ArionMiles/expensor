package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	ProviderName   = "openai"
	defaultTimeout = 60 * time.Second
)

type credentials struct {
	APIKey string `json:"api_key"`
}

type providerConfig struct {
	Model   string `json:"model"`
	BaseURL string `json:"base_url"`
}

type clientDefaults struct {
	Model   string
	BaseURL string
}

type client struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// Provider returns the OpenAI API-backed LLM provider registration.
func Provider(metadata llm.ProviderMetadata) (llm.Provider, error) {
	const op = "llm.openai.Provider"

	defaultModel, ok := llm.ConfigStringDefault(metadata.ConfigSchema, "model")
	if !ok {
		return llm.Provider{}, errors.B.Op(op).KindInvalidInput().Text("OpenAI provider metadata requires a model default").Build()
	}
	defaultBaseURL, ok := llm.ConfigStringDefault(metadata.ConfigSchema, "base_url")
	if !ok {
		return llm.Provider{}, errors.B.Op(op).KindInvalidInput().Text("OpenAI provider metadata requires a base URL default").Build()
	}
	defaults := clientDefaults{Model: defaultModel, BaseURL: defaultBaseURL}
	metadata.Name = ProviderName
	metadata.ConfigSchema = append(json.RawMessage(nil), metadata.ConfigSchema...)
	metadata.ModelOptions = append([]llm.ModelOption(nil), metadata.ModelOptions...)
	metadata.Capabilities = []llm.Capability{
		llm.CapabilityTextGeneration,
		llm.CapabilityJSONSchema,
	}
	return llm.Provider{
		Metadata: metadata,
		NewClient: func(input llm.ClientConfig) (llm.Client, error) {
			return newClient(input, defaults)
		},
	}, nil
}

func newClient(input llm.ClientConfig, defaults clientDefaults) (llm.Client, error) {
	const op = "llm.openai.NewClient"

	var creds credentials
	if len(input.Credentials) > 0 {
		if err := json.Unmarshal(input.Credentials, &creds); err != nil {
			return nil, errors.B.Op(op).KindInvalidInput().Text("decoding OpenAI credentials").Err(err).Build()
		}
	}
	if strings.TrimSpace(creds.APIKey) == "" {
		return nil, errors.B.Op(op).KindFailedPrecondition().Text("OpenAI API key is not configured").Build()
	}

	cfg := providerConfig(defaults)
	if len(input.Config) > 0 {
		if err := json.Unmarshal(input.Config, &cfg); err != nil {
			return nil, errors.B.Op(op).KindInvalidInput().Text("decoding OpenAI config").Err(err).Build()
		}
	}
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.Model == "" {
		cfg.Model = defaults.Model
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaults.BaseURL
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
		return errors.B.Op(op).Err(err).Build()
	}
	var out struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal([]byte(resp.Text), &out); err != nil || !out.OK {
		return errors.B.Op(op).KindBadGateway().Text("OpenAI healthcheck returned an invalid structured response").Build()
	}
	return nil
}

func (c *client) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	const op = "llm.openai.Complete"

	payload, err := c.responsesPayload(req)
	if err != nil {
		return llm.Response{}, errors.B.Op(op).Err(err).Build()
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return llm.Response{}, errors.B.Op(op).KindInternal().Text("building OpenAI request").Err(err).Build()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return llm.Response{}, errors.B.Op(op).KindInternal().Text("building OpenAI request").Err(err).Build()
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return llm.Response{}, errors.B.Op(op).KindUnavailable().Text("calling OpenAI").Err(err).Build()
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
	if err != nil {
		return llm.Response{}, errors.B.Op(op).KindBadGateway().Text("reading OpenAI response").Err(err).Build()
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return llm.Response{}, errors.B.Op(op).Err(openAIProviderError(httpResp.StatusCode, respBody)).Build()
	}

	var resp responsesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return llm.Response{}, errors.B.Op(op).KindBadGateway().Text("decoding OpenAI response").Err(err).Build()
	}
	if resp.Error != nil {
		return llm.Response{}, errors.B.Op(op).Err(openAIProviderFailure(httpResp.StatusCode, resp.Error.Code, resp.Error.Type)).Build()
	}

	text := strings.TrimSpace(resp.OutputText)
	if text == "" {
		text = strings.TrimSpace(resp.FirstOutputText())
	}
	if text == "" {
		return llm.Response{}, errors.B.Op(op).KindBadGateway().Text("OpenAI response did not include output text").Build()
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
		return responsesRequest{}, errors.B.Op(op).KindInvalidInput().Text("OpenAI request requires at least one non-empty message").Build()
	}
	if req.ResponseFormat.Type != "" && req.ResponseFormat.Type != llm.ResponseFormatText {
		format, err := responseTextFormat(req.ResponseFormat)
		if err != nil {
			return responsesRequest{}, errors.B.Op(op).Err(err).Build()
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
			return responsesTextFormat{}, errors.B.Op(op).KindInvalidInput().Text("json_schema response format requires a valid schema").Build()
		}
		var schema map[string]any
		if err := json.Unmarshal(format.Schema, &schema); err != nil {
			return responsesTextFormat{}, errors.B.Op(op).KindInvalidInput().Text("decoding json_schema response format").Err(err).Build()
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
		return responsesTextFormat{}, errors.B.Op(op).KindInvalidInput().Textf("unsupported OpenAI response format %q", format.Type).Build()
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
		return errors.B.KindUnauthenticated().UserMsg("OpenAI API key was rejected. Check the key and try again.").Build()
	case "insufficient_quota":
		return errors.B.KindResourceExhausted().UserMsg("OpenAI API quota is unavailable. Add billing credits or choose another LLM provider.").Build()

	case "rate_limit_exceeded":
		return errors.B.KindResourceExhausted().UserMsg("OpenAI rate limit exceeded. Wait a moment and try again.").Build()
	}

	switch status {
	case http.StatusUnauthorized:
		return errors.B.KindUnauthenticated().UserMsg("OpenAI API key was rejected. Check the key and try again.").Build()
	case http.StatusTooManyRequests:
		return errors.B.KindResourceExhausted().UserMsg("LLM provider request was rate limited. Wait a moment and try again.").Build()
	default:
		return errors.B.KindBadGateway().UserMsg("LLM provider request failed.").Build()
	}
}
