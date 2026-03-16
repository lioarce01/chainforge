package core

import "context"

// Provider is the single interface every LLM adapter must satisfy.
type Provider interface {
	// Name returns a human-readable identifier, e.g. "anthropic", "openai".
	Name() string
	// Chat sends a request and waits for the full response.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	// ChatStream sends a request and returns a channel of events.
	// The channel is closed when streaming is complete or on error.
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
}

// Tool is anything the agent can call.
type Tool interface {
	// Definition returns the schema the LLM sees.
	Definition() ToolDefinition
	// Call executes the tool with the given JSON input and returns a string result.
	Call(ctx context.Context, input string) (string, error)
}

// MemoryStore persists conversation history across agent Run calls.
type MemoryStore interface {
	// Get retrieves all messages for a session.
	Get(ctx context.Context, sessionID string) ([]Message, error)
	// Append adds messages to a session.
	Append(ctx context.Context, sessionID string, msgs ...Message) error
	// Clear removes all messages for a session.
	Clear(ctx context.Context, sessionID string) error
}
