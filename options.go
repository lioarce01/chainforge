package chainforge

import (
	"log/slog"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
	mcppkg "github.com/lioarce01/chainforge/pkg/mcp"
	"github.com/lioarce01/chainforge/pkg/middleware/logging"
	cfotel "github.com/lioarce01/chainforge/pkg/middleware/otel"
	"github.com/lioarce01/chainforge/pkg/middleware/retry"
	"github.com/lioarce01/chainforge/pkg/providers/gemini"
)

// agentConfig holds all configuration for an Agent.
type agentConfig struct {
	provider         core.Provider
	model            string
	systemPrompt     string
	tools            []core.Tool
	memory           core.MemoryStore
	maxIterations    int
	toolTimeout      time.Duration
	maxTokens        int
	temperature      float64
	logger           *slog.Logger
	mcpServers       []mcppkg.ServerConfig
	providerWrappers []func(core.Provider) core.Provider
	maxHistory       int           // 0 = unlimited
	runTimeout       time.Duration // 0 = no timeout
}

func defaultConfig() agentConfig {
	return agentConfig{
		maxIterations: 10,
		toolTimeout:   30 * time.Second,
		maxTokens:     4096,
		temperature:   0.7,
		logger:        slog.Default(),
	}
}

// AgentOption configures an Agent.
type AgentOption func(*agentConfig)

// WithProvider sets the LLM provider.
func WithProvider(p core.Provider) AgentOption {
	return func(c *agentConfig) { c.provider = p }
}

// WithModel sets the model identifier (e.g. "claude-sonnet-4-6").
func WithModel(model string) AgentOption {
	return func(c *agentConfig) { c.model = model }
}

// WithSystemPrompt sets the system prompt prepended to every conversation.
func WithSystemPrompt(prompt string) AgentOption {
	return func(c *agentConfig) { c.systemPrompt = prompt }
}

// WithTools registers tools the agent may call.
func WithTools(tools ...core.Tool) AgentOption {
	return func(c *agentConfig) { c.tools = append(c.tools, tools...) }
}

// WithMemory sets a memory store for cross-run history persistence.
func WithMemory(m core.MemoryStore) AgentOption {
	return func(c *agentConfig) { c.memory = m }
}

// WithMaxIterations caps the agent loop iterations (default: 10).
func WithMaxIterations(n int) AgentOption {
	return func(c *agentConfig) { c.maxIterations = n }
}

// WithToolTimeout sets the per-tool execution timeout (default: 30s).
func WithToolTimeout(d time.Duration) AgentOption {
	return func(c *agentConfig) { c.toolTimeout = d }
}

// WithMaxTokens sets the max tokens for each LLM call (default: 4096).
func WithMaxTokens(n int) AgentOption {
	return func(c *agentConfig) { c.maxTokens = n }
}

// WithTemperature sets the sampling temperature (default: 0.7).
func WithTemperature(t float64) AgentOption {
	return func(c *agentConfig) { c.temperature = t }
}

// WithLogger sets a structured logger (default: slog.Default()).
func WithLogger(l *slog.Logger) AgentOption {
	return func(c *agentConfig) { c.logger = l }
}

// WithMCPServer registers a single MCP server whose tools become available in the agent.
// Connection is deferred until the first Run call.
//
//	chainforge.WithMCPServer(mcp.Stdio("npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp").WithName("fs"))
//	chainforge.WithMCPServer(mcp.HTTP("https://api.example.com/mcp").WithName("myserver"))
func WithMCPServer(s mcppkg.ServerConfig) AgentOption {
	return func(c *agentConfig) { c.mcpServers = append(c.mcpServers, s) }
}

// WithMCPServers registers multiple MCP servers at once.
func WithMCPServers(servers ...mcppkg.ServerConfig) AgentOption {
	return func(c *agentConfig) { c.mcpServers = append(c.mcpServers, servers...) }
}

// WithLogging wraps the provider with structured slog logging.
// Every Chat and ChatStream call is logged with latency, token counts, and errors.
// If logger is nil, slog.Default() is used.
// Applied after all options are resolved, so order relative to WithProvider does not matter.
func WithLogging(logger *slog.Logger) AgentOption {
	return func(c *agentConfig) {
		c.providerWrappers = append(c.providerWrappers, func(p core.Provider) core.Provider {
			return logging.NewLoggedProvider(p, logger)
		})
	}
}

// WithRetry wraps the provider with automatic retry on transient errors.
// maxAttempts is the total number of attempts (1 = no retry, 3 = 1 attempt + 2 retries).
// Uses exponential backoff: 200ms, 400ms, 800ms … capped at 10s.
// Context cancellation and deadline errors are never retried.
// Applied after all options are resolved, so order relative to WithProvider does not matter.
func WithRetry(maxAttempts int) AgentOption {
	return func(c *agentConfig) {
		c.providerWrappers = append(c.providerWrappers, func(p core.Provider) core.Provider {
			return retry.New(p, maxAttempts)
		})
	}
}

// WithRunTimeout sets a per-run deadline. If the agent loop does not complete
// within d, Run and RunWithUsage return context.DeadlineExceeded.
// 0 (default) means no timeout.
func WithRunTimeout(d time.Duration) AgentOption {
	return func(c *agentConfig) { c.runTimeout = d }
}

// WithMaxHistory limits how many messages are loaded from memory on each Run call.
// Only the most recent n messages are used; older messages are dropped for that turn.
// This prevents context window overflow on long-running sessions.
// 0 (default) means unlimited — all history is loaded.
func WithMaxHistory(n int) AgentOption {
	return func(c *agentConfig) { c.maxHistory = n }
}

// WithGemini configures the agent to use a Google Gemini provider.
// apiKey is your Gemini API key; model is the model name (e.g. "gemini-2.0-flash").
// If provider creation fails, the error is silently swallowed and NewAgent will
// return an error because provider will be nil.
func WithGemini(apiKey, model string) AgentOption {
	return func(c *agentConfig) {
		p, err := gemini.New(apiKey, model)
		if err != nil {
			return
		}
		c.provider = p
		c.model = model
	}
}

// WithTracing wraps the provider with OpenTelemetry tracing.
// Each Chat call becomes a span; ChatStream spans cover the full stream duration.
// If InitTracerProvider has not been called, the global noop tracer is used — no error.
// Applied after all options are resolved, so order relative to WithProvider does not matter.
func WithTracing() AgentOption {
	return func(c *agentConfig) {
		c.providerWrappers = append(c.providerWrappers, func(p core.Provider) core.Provider {
			return cfotel.NewTracedProvider(p, cfotel.Tracer())
		})
	}
}
