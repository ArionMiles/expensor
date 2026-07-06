// Package llm provides provider-neutral primitives for LLM-backed workflows.
package llm

import (
	"context"
	"encoding/json"
)

// Capability describes a provider feature that workflows may require.
type Capability string

const (
	CapabilityTextGeneration Capability = "text_generation"
	CapabilityTools          Capability = "tools"
	CapabilityJSONSchema     Capability = "json_schema"
	CapabilityStreaming      Capability = "streaming"
)

// Role identifies the speaker for a model message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a provider-neutral chat/message input or output.
type Message struct {
	Role       Role   `json:"role" yaml:"role"`
	Content    string `json:"content" yaml:"content"`
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty" yaml:"tool_call_id,omitempty"`
}

// Tool declares a callable tool exposed to an LLM provider.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ToolCall is a provider-neutral model tool invocation.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ResponseFormatType identifies structured output requirements.
type ResponseFormatType string

const (
	ResponseFormatText       ResponseFormatType = "text"
	ResponseFormatJSONObject ResponseFormatType = "json_object"
	ResponseFormatJSONSchema ResponseFormatType = "json_schema"
)

// ResponseFormat describes expected model output shape.
type ResponseFormat struct {
	Type   ResponseFormatType `json:"type"`
	Schema json.RawMessage    `json:"schema,omitempty"`
}

// Usage records provider token accounting in provider-neutral fields.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// Request is the provider-neutral input for a model completion.
type Request struct {
	Workflow             string          `json:"workflow,omitempty"`
	Purpose              string          `json:"purpose,omitempty"`
	Messages             []Message       `json:"messages"`
	Tools                []Tool          `json:"tools,omitempty"`
	ResponseFormat       ResponseFormat  `json:"response_format,omitempty"`
	RequiredCapabilities []Capability    `json:"required_capabilities,omitempty"`
	MaxOutputTokens      int             `json:"max_output_tokens,omitempty"`
	Temperature          *float64        `json:"temperature,omitempty"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
}

// Response is the provider-neutral output from a model completion.
type Response struct {
	Messages     []Message  `json:"messages,omitempty"`
	Text         string     `json:"text,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	Usage        Usage      `json:"usage,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

// Client is implemented by concrete LLM providers.
type Client interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

// ClientConfig contains tenant runtime state needed to construct a provider client.
type ClientConfig struct {
	Config      json.RawMessage
	Credentials json.RawMessage
}
