// Package ollama provides an Ollama-compatible provider by embedding the OpenAI provider.
// Ollama exposes an OpenAI-compatible API at /api/chat and /v1/chat/completions.
package ollama

import (
	"github.com/lioarce01/chainforge/pkg/providers/openai"
)

const defaultBaseURL = "http://localhost:11434/v1"

// New creates an Ollama provider that connects to a local Ollama server.
// baseURL defaults to http://localhost:11434/v1 if empty.
func New(baseURL string) *openai.Provider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	// Ollama doesn't require auth; use a placeholder key
	return openai.NewWithBaseURL("ollama", baseURL, "ollama")
}
