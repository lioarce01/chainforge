package core

import "encoding/json"

// Role represents who sent a message in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one turn in a conversation.
type Message struct {
	Role       Role        `json:"role"`
	Content    string      `json:"content,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"` // for RoleTool results
	Name       string      `json:"name,omitempty"`         // tool name for RoleTool results
}

// ToolDefinition describes a tool the LLM can invoke.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolCall is a single tool invocation requested by the model.
type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"` // JSON string
}

// StopReason indicates why the model stopped generating.
type StopReason string

const (
	StopReasonEndTurn   StopReason = "end_turn"
	StopReasonToolUse   StopReason = "tool_use"
	StopReasonMaxTokens StopReason = "max_tokens"
	StopReasonStop      StopReason = "stop"
)

// ChatRequest is what we send to a provider.
type ChatRequest struct {
	Model    string
	Messages []Message
	Tools    []ToolDefinition
	Options  ChatOptions
}

// ChatOptions are per-call overrides.
type ChatOptions struct {
	MaxTokens    int
	Temperature  float64
	SystemPrompt string
}

// ChatResponse is what a provider returns.
type ChatResponse struct {
	Message    Message
	StopReason StopReason
	Usage      Usage
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// StreamEvent is emitted during streaming responses.
type StreamEvent struct {
	Type       StreamEventType
	TextDelta  string
	ToolCall   *ToolCall
	StopReason StopReason
	Usage      *Usage
	Error      error
}

// StreamEventType categorises a StreamEvent.
type StreamEventType string

const (
	StreamEventText     StreamEventType = "text"
	StreamEventToolCall StreamEventType = "tool_call"
	StreamEventDone     StreamEventType = "done"
	StreamEventError    StreamEventType = "error"
)
