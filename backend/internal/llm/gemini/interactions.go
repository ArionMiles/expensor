package gemini

import "github.com/ArionMiles/expensor/backend/internal/llm"

type interactionsRequest struct {
	Model             string                        `json:"model"`
	Input             string                        `json:"input"`
	SystemInstruction string                        `json:"system_instruction,omitempty"`
	ResponseFormat    *interactionsResponseFormat   `json:"response_format,omitempty"`
	Store             *bool                         `json:"store"`
	GenerationConfig  *interactionsGenerationConfig `json:"generation_config,omitempty"`
}

type interactionsResponseFormat struct {
	Type     string         `json:"type"`
	MIMEType string         `json:"mime_type"`
	Schema   map[string]any `json:"schema"`
}

type interactionsGenerationConfig struct {
	MaxOutputTokens int      `json:"max_output_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	ThinkingLevel   string   `json:"thinking_level,omitempty"`
}

type interactionsResponse struct {
	Status string `json:"status"`
	Steps  []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"steps"`
	Usage interactionsUsage `json:"usage"`
}

type interactionsUsage struct {
	InputTokens  int `json:"total_input_tokens"`
	OutputTokens int `json:"total_output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func (u interactionsUsage) toLLMUsage() llm.Usage {
	return llm.Usage{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
		TotalTokens:  u.TotalTokens,
	}
}
