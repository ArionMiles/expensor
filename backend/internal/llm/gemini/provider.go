package gemini

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
	ProviderName   = "gemini"
	defaultBaseURL = "https://generativelanguage.googleapis.com/v1"
	flashModel     = "gemini-3.5-flash"
	flashLiteModel = "gemini-3.5-flash-lite"
	defaultTimeout = 60 * time.Second
)

type credentials struct {
	APIKey string `json:"api_key"`
}

type providerConfig struct {
	Model string `json:"model"`
}

type client struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// Provider returns the Gemini Developer API-backed LLM provider registration.
func Provider(metadata llm.ProviderMetadata) (llm.Provider, error) {
	const op = "llm.gemini.Provider"

	defaultModel, ok := llm.ConfigStringDefault(metadata.ConfigSchema, "model")
	if !ok {
		return llm.Provider{}, errors.B.Op(op).KindInvalidInput().Text("Gemini provider metadata requires a model default").Build()
	}
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
			return newClient(input, defaultModel)
		},
	}, nil
}

func newClient(input llm.ClientConfig, defaultModel string) (llm.Client, error) {
	const op = "llm.gemini.NewClient"

	var creds credentials
	if len(input.Credentials) > 0 {
		if err := json.Unmarshal(input.Credentials, &creds); err != nil {
			return nil, errors.B.Op(op).KindInvalidInput().Text("decoding Gemini credentials").Err(err).Build()
		}
	}
	if strings.TrimSpace(creds.APIKey) == "" {
		return nil, errors.B.Op(op).KindFailedPrecondition().Text("Gemini API key is not configured").Build()
	}

	cfg := providerConfig{Model: defaultModel}
	if len(input.Config) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(input.Config))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cfg); err != nil {
			return nil, errors.B.Op(op).KindInvalidInput().Text("decoding Gemini config").Err(err).Build()
		}
	}
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}

	return &client{
		apiKey:     strings.TrimSpace(creds.APIKey),
		model:      cfg.Model,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}, nil
}

func (c *client) HealthCheck(ctx context.Context) error {
	const op = "llm.gemini.HealthCheck"

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
			Name:   "gemini_healthcheck",
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
		return errors.B.Op(op).KindBadGateway().Text("Gemini healthcheck returned an invalid structured response").Build()
	}
	return nil
}

func (c *client) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	const op = "llm.gemini.Complete"

	payload, err := c.interactionsPayload(req)
	if err != nil {
		return llm.Response{}, errors.B.Op(op).Err(err).Build()
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return llm.Response{}, errors.B.Op(op).KindInternal().Text("building Gemini request").Err(err).Build()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/interactions", bytes.NewReader(body))
	if err != nil {
		return llm.Response{}, errors.B.Op(op).KindInternal().Text("building Gemini request").Err(err).Build()
	}
	httpReq.Header.Set("x-goog-api-key", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return llm.Response{}, errors.B.Op(op).KindUnavailable().Text("calling Gemini").Err(err).Build()
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
	if err != nil {
		return llm.Response{}, errors.B.Op(op).KindBadGateway().Text("reading Gemini response").Err(err).Build()
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return llm.Response{}, errors.B.Op(op).Err(geminiProviderError(httpResp.StatusCode, respBody)).Build()
	}

	var resp interactionsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return llm.Response{}, errors.B.Op(op).KindBadGateway().Text("decoding Gemini response").Err(err).Build()
	}
	if resp.Status != "completed" {
		return llm.Response{}, errors.B.Op(op).Err(geminiInteractionStatusError(resp.Status)).Build()
	}
	text := responseText(resp)
	if text == "" {
		return llm.Response{}, errors.B.Op(op).KindBadGateway().Text("Gemini response did not include output text").Build()
	}
	return llm.Response{
		Text:         text,
		Messages:     []llm.Message{{Role: llm.RoleAssistant, Content: text}},
		Usage:        resp.Usage.toLLMUsage(),
		FinishReason: resp.Status,
	}, nil
}

func (c *client) interactionsPayload(req llm.Request) (interactionsRequest, error) {
	const op = "llm.gemini.interactionsPayload"

	if len(req.Tools) > 0 {
		return interactionsRequest{}, errors.B.Op(op).KindInvalidInput().Text("Gemini tool requests are not supported").Build()
	}
	systemMessages := make([]string, 0, 1)
	userMessages := make([]string, 0, len(req.Messages))
	for _, message := range req.Messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		switch message.Role {
		case llm.RoleSystem:
			systemMessages = append(systemMessages, content)
		case llm.RoleUser:
			userMessages = append(userMessages, content)
		default:
			return interactionsRequest{}, errors.B.Op(op).KindInvalidInput().Textf("unsupported Gemini message role %q", message.Role).Build()

		}
	}
	if len(userMessages) == 0 {
		return interactionsRequest{}, errors.B.Op(op).KindInvalidInput().Text("Gemini request requires at least one non-empty user message").Build()
	}

	store := false
	payload := interactionsRequest{
		Model:             c.model,
		Input:             strings.Join(userMessages, "\n\n"),
		SystemInstruction: strings.Join(systemMessages, "\n\n"),
		Store:             &store,
	}
	if req.MaxOutputTokens > 0 || req.Temperature != nil {
		payload.GenerationConfig = &interactionsGenerationConfig{
			MaxOutputTokens: req.MaxOutputTokens,
			Temperature:     req.Temperature,
		}
	}
	if req.Workflow == "provider_setup" && req.Purpose == "healthcheck" && supportsMinimalThinkingLevel(c.model) {
		if payload.GenerationConfig == nil {
			payload.GenerationConfig = &interactionsGenerationConfig{}
		}
		payload.GenerationConfig.ThinkingLevel = "minimal"
	} else if supportsLowThinkingLevel(c.model) &&
		(req.ResponseFormat.Type == llm.ResponseFormatJSONSchema || req.ResponseFormat.Type == llm.ResponseFormatJSONObject) {
		if payload.GenerationConfig == nil {
			payload.GenerationConfig = &interactionsGenerationConfig{}
		}
		payload.GenerationConfig.ThinkingLevel = "low"
	}
	format, err := geminiResponseFormat(req.ResponseFormat)
	if err != nil {
		return interactionsRequest{}, errors.B.Op(op).Err(err).Build()
	}
	payload.ResponseFormat = format
	return payload, nil
}

func supportsMinimalThinkingLevel(model string) bool {
	return model == "gemini-3.6-flash" || model == flashModel || model == flashLiteModel
}

func supportsLowThinkingLevel(model string) bool {
	switch model {
	case "gemini-3.8-flash", "gemini-3.7-flash", "gemini-3.6-flash", flashModel, flashLiteModel:
		return true
	default:
		return false
	}
}

func geminiResponseFormat(format llm.ResponseFormat) (*interactionsResponseFormat, error) {
	const op = "llm.gemini.responseFormat"

	switch format.Type {
	case "", llm.ResponseFormatText:
		return nil, nil
	case llm.ResponseFormatJSONSchema:
		if len(format.Schema) == 0 || !json.Valid(format.Schema) {
			return nil, errors.B.Op(op).KindInvalidInput().Text("json_schema response format requires a valid schema").Build()
		}
		var schema map[string]any
		if err := json.Unmarshal(format.Schema, &schema); err != nil {
			return nil, errors.B.Op(op).KindInvalidInput().Text("decoding json_schema response format").Err(err).Build()
		}
		return &interactionsResponseFormat{Type: "text", MIMEType: "application/json", Schema: schema}, nil
	case llm.ResponseFormatJSONObject:
		return &interactionsResponseFormat{
			Type:     "text",
			MIMEType: "application/json",
			Schema:   map[string]any{"type": "object"},
		}, nil
	default:
		return nil, errors.B.Op(op).KindInvalidInput().Textf("unsupported Gemini response format %q", format.Type).Build()
	}
}

func responseText(resp interactionsResponse) string {
	parts := make([]string, 0, 1)
	for _, step := range resp.Steps {
		if step.Type != "model_output" {
			continue
		}
		for _, content := range step.Content {
			if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
				parts = append(parts, content.Text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

func geminiInteractionStatusError(status string) error {
	detail := "Gemini interaction returned status=" + safeInteractionStatus(status)
	switch status {
	case "budget_exceeded":
		return errors.B.KindResourceExhausted().UserMsg("Gemini token budget was exceeded. Choose a smaller request or try again.").Text(detail).Build()

	case "incomplete":
		return errors.B.KindBadGateway().UserMsg("Gemini returned an incomplete response. Try again.").Text(detail).Build()
	default:
		return errors.B.KindBadGateway().UserMsg("Gemini could not complete the request.").Text(detail).Build()
	}
}

func safeInteractionStatus(status string) string {
	switch status {
	case "in_progress", "requires_action", "completed", "failed", "cancelled", "incomplete", "budget_exceeded": //nolint:misspell // Gemini API enum.
		return status
	default:
		return "unknown"
	}
}

func geminiProviderError(status int, body []byte) error {
	var parsed struct {
		Error struct {
			Status  string `json:"status"`
			Details []struct {
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &parsed)
	reason := ""
	if len(parsed.Error.Details) > 0 {
		reason = parsed.Error.Details[0].Reason
	}
	detail := geminiProviderFailureDetail(status, parsed.Error.Status, reason)
	if reason == "API_KEY_INVALID" || status == http.StatusUnauthorized {
		return errors.B.KindUnauthenticated().UserMsg("Gemini API key was rejected. Check the key and try again.").Text(detail).Build()
	}
	if reason == "SERVICE_DISABLED" || parsed.Error.Status == "FAILED_PRECONDITION" {
		return errors.B.
			KindConflict().
			UserMsg("Gemini API access is unavailable for this project or region. Check availability or enable billing.").
			Text(detail).
			Build()
	}
	if status == http.StatusForbidden {
		return errors.B.KindUnauthenticated().UserMsg("Gemini API key was rejected or lacks permission for this model.").Text(detail).Build()
	}
	if status == http.StatusNotFound {
		return errors.B.KindConflict().UserMsg("Gemini model is unavailable. Choose another model and try again.").Text(detail).Build()
	}
	if status == http.StatusTooManyRequests || parsed.Error.Status == "RESOURCE_EXHAUSTED" {
		return errors.B.KindResourceExhausted().UserMsg("Gemini quota or rate limit was exceeded. Wait a moment or check billing.").Text(detail).Build()
	}
	if status >= http.StatusInternalServerError {
		return errors.B.KindBadGateway().UserMsg("Gemini is temporarily unavailable. Try again shortly.").Text(detail).Build()
	}
	return errors.B.KindBadGateway().UserMsg("Gemini rejected the request. Check the configured model and try again.").Text(detail).Build()
}

func geminiProviderFailureDetail(status int, providerStatus, reason string) string {
	parts := []string{fmt.Sprintf("Gemini API returned HTTP %d", status)}
	if value := safeProviderCode(providerStatus); value != "" {
		parts = append(parts, "status="+value)
	}
	if value := safeProviderCode(reason); value != "" {
		parts = append(parts, "reason="+value)
	}
	return strings.Join(parts, " ")
}

func safeProviderCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 80 {
		return ""
	}
	for _, char := range value {
		if (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' {
			return ""
		}
	}
	return value
}

var _ llm.Client = (*client)(nil)
