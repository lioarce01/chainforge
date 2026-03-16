package chainforge

import (
	"log/slog"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
	mcppkg "github.com/lioarce01/chainforge/pkg/mcp"
)

// agentConfig holds all configuration for an Agent.
type agentConfig struct {
	provider      core.Provider
	model         string
	systemPrompt  string
	tools         []core.Tool
	memory        core.MemoryStore
	maxIterations int
	toolTimeout   time.Duration
	maxTokens     int
	temperature   float64
	logger        *slog.Logger
	mcpServers    []mcppkg.ServerConfig
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
