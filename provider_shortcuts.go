package chainforge

import (
	"github.com/lioarce01/chainforge/pkg/providers/anthropic"
	"github.com/lioarce01/chainforge/pkg/providers/ollama"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
)

// WithAnthropic is a shorthand that sets the Anthropic provider and model in one call.
//
//	chainforge.NewAgent(
//	    chainforge.WithAnthropic(os.Getenv("ANTHROPIC_API_KEY"), "claude-sonnet-4-6"),
//	)
func WithAnthropic(apiKey, model string) AgentOption {
	return func(c *agentConfig) {
		c.provider = anthropic.New(apiKey)
		c.model = model
	}
}

// WithOpenAI is a shorthand that sets the OpenAI provider and model in one call.
func WithOpenAI(apiKey, model string) AgentOption {
	return func(c *agentConfig) {
		c.provider = openai.New(apiKey)
		c.model = model
	}
}

// WithOllama is a shorthand that sets the Ollama provider and model in one call.
// If baseURL is empty, it defaults to http://localhost:11434/v1.
func WithOllama(baseURL, model string) AgentOption {
	return func(c *agentConfig) {
		c.provider = ollama.New(baseURL)
		c.model = model
	}
}

// WithOpenAICompatible is a shorthand for any OpenAI-compatible provider.
// name is used as the provider identifier in logs and traces.
func WithOpenAICompatible(apiKey, baseURL, name, model string) AgentOption {
	return func(c *agentConfig) {
		c.provider = openai.NewWithBaseURL(apiKey, baseURL, name)
		c.model = model
	}
}
