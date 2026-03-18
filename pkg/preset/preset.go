// Package preset provides named constructors for common agent configurations.
// These are thin convenience wrappers — they set sensible defaults and reduce
// boilerplate for the 90% case. All options are still passable via the opts
// variadic, and none of the preset behaviour is enforced at runtime.
package preset

import (
	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
)

// ChatbotConfig configures a Chatbot preset.
type ChatbotConfig struct {
	// SystemPrompt is the system message prepended to every conversation.
	SystemPrompt string

	// MaxHistory caps how many messages are loaded from memory per Run.
	// 0 (default) means unlimited.
	MaxHistory int

	// Memory is the backing store for conversation history.
	// Defaults to a new in-memory store if nil.
	Memory core.MemoryStore
}

// Chatbot creates a conversational agent with memory and sensible defaults.
// MaxIterations is set to 1 — chatbots rarely need an agentic loop.
// Pass extra AgentOptions to override any field (e.g. WithTools, WithRetry).
//
//	agent, err := preset.Chatbot(p, "claude-sonnet-4-6", preset.ChatbotConfig{
//	    SystemPrompt: "You are a helpful assistant.",
//	    MaxHistory:   20,
//	})
func Chatbot(provider core.Provider, model string, cfg ChatbotConfig, opts ...chainforge.AgentOption) (*chainforge.Agent, error) {
	mem := cfg.Memory
	if mem == nil {
		mem = inmemory.New()
	}

	base := []chainforge.AgentOption{
		chainforge.WithProvider(provider),
		chainforge.WithModel(model),
		chainforge.WithMemory(mem),
		chainforge.WithMaxIterations(1),
	}
	if cfg.SystemPrompt != "" {
		base = append(base, chainforge.WithSystemPrompt(cfg.SystemPrompt))
	}
	if cfg.MaxHistory > 0 {
		base = append(base, chainforge.WithMaxHistory(cfg.MaxHistory))
	}
	// Extra opts override anything above.
	return chainforge.NewAgent(append(base, opts...)...)
}

// ToolAgentConfig configures a ToolAgent preset.
type ToolAgentConfig struct {
	// SystemPrompt is the system message prepended to every conversation.
	SystemPrompt string

	// Tools are the tools available to the agent.
	Tools []core.Tool

	// MaxIterations caps the agent loop (default: 10).
	MaxIterations int

	// MaxRetries is the number of total provider call attempts (default: 3).
	// 1 means no retries.
	MaxRetries int

	// Memory is optional. When set, conversation history is persisted.
	Memory core.MemoryStore
}

// ToolAgent creates an agent with tools, retry, and sensible defaults.
// Pass extra AgentOptions to override any field.
//
//	agent, err := preset.ToolAgent(p, "claude-sonnet-4-6", preset.ToolAgentConfig{
//	    SystemPrompt: "You are a research assistant.",
//	    Tools:        []core.Tool{searchTool, calcTool},
//	    MaxRetries:   3,
//	})
func ToolAgent(provider core.Provider, model string, cfg ToolAgentConfig, opts ...chainforge.AgentOption) (*chainforge.Agent, error) {
	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	base := []chainforge.AgentOption{
		chainforge.WithProvider(provider),
		chainforge.WithModel(model),
		chainforge.WithMaxIterations(maxIter),
		chainforge.WithRetry(maxRetries),
	}
	if cfg.SystemPrompt != "" {
		base = append(base, chainforge.WithSystemPrompt(cfg.SystemPrompt))
	}
	if len(cfg.Tools) > 0 {
		base = append(base, chainforge.WithTools(cfg.Tools...))
	}
	if cfg.Memory != nil {
		base = append(base, chainforge.WithMemory(cfg.Memory))
	}
	return chainforge.NewAgent(append(base, opts...)...)
}
