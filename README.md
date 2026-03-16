# chainforge

A provider-agnostic Go agent orchestration framework. Import it as a library to build AI applications without touching any LLM SDK directly.

```go
agent, err := chainforge.NewAgent(
    chainforge.WithProvider(anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))),
    chainforge.WithModel("claude-sonnet-4-6"),
    chainforge.WithSystemPrompt("You are a helpful assistant."),
    chainforge.WithTools(calculator.New()),
    chainforge.WithMemory(inmemory.New()),
)
result, err := agent.Run(ctx, "session-1", "What is 2^10 + 144?")
```

## Features

- **Provider-agnostic** — swap Anthropic, OpenAI, or Ollama with one line; `pkg/core` has zero external dependencies
- **MCP client** — connect any MCP server (Streamable HTTP or Stdio) with a single line; tools become indistinguishable from built-in tools
- **Concurrent tool dispatch** — multiple tool calls from one LLM response run in parallel goroutines
- **Multi-agent orchestration** — sequential pipelines and parallel fan-out
- **Streaming** — `RunStream()` returns a channel of events
- **Memory** — pluggable `MemoryStore` interface; in-memory implementation included
- **Structured logging** — `slog`-based, configurable via `WithLogger`

## Installation

```bash
go get github.com/lioarce01/chainforge
```

## Providers

| Provider | Package |
|---|---|
| Anthropic (Claude) | `pkg/providers/anthropic` |
| OpenAI | `pkg/providers/openai` |
| Ollama (local) | `pkg/providers/ollama` |
| Any OpenAI-compatible API | `openai.NewWithBaseURL(...)` |

Swap providers with zero other changes:

```go
chainforge.WithProvider(anthropic.New(os.Getenv("ANTHROPIC_API_KEY")))
chainforge.WithProvider(openai.New(os.Getenv("OPENAI_API_KEY")))
chainforge.WithProvider(ollama.New("http://localhost:11434"))
```

## Tools

### Built-in

```go
chainforge.WithTools(calculator.New())
chainforge.WithTools(websearch.New(backend))
```

### Custom tool

```go
schema := tools.NewSchema().
    Add("query", tools.Property{Type: tools.TypeString, Description: "search query"}, true).
    MustBuild()

myTool, _ := tools.Func("search", "Search for information", schema,
    func(ctx context.Context, input string) (string, error) {
        return result, nil
    },
)
```

### MCP servers

Connect any MCP-compatible server. Tools are automatically discovered and namespaced as `servername__toolname`.

```go
// Remote server — Streamable HTTP (used by Cursor, Claude Code, hosted MCP services)
chainforge.WithMCPServer(mcp.HTTP("https://docs.langchain.com/mcp").WithName("langchain"))

// Local subprocess — Stdio (requires Node.js)
chainforge.WithMCPServer(mcp.Stdio("npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp").WithName("fs"))

// Multiple servers at once
chainforge.WithMCPServers(
    mcp.HTTP("https://api.example.com/mcp").WithName("myapi"),
    mcp.Stdio("npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp").WithName("fs"),
)

defer agent.Close() // terminates subprocesses / closes connections
```

MCP servers are connected in parallel on the first `Run` call. Connection errors are cached — subsequent calls return the same error without retry.

## Multi-agent Orchestration

### Sequential pipeline

```go
result, err := orchestrator.Sequential(ctx, "session",
    "initial input",
    orchestrator.StepOf("research", researchAgent, "Research: {{.input}}"),
    orchestrator.StepOf("write",    writerAgent,   "Write based on: {{.previous}}"),
)
```

### Parallel fan-out

```go
results, err := orchestrator.Parallel(ctx, "session",
    orchestrator.FanOf("pros",     proAgent,     "Analyze pros of Go"),
    orchestrator.FanOf("cons",     conAgent,     "Analyze cons of Go"),
    orchestrator.FanOf("summary",  summaryAgent, "Summarize Go"),
)
```

Parallel always returns all results — a failed branch doesn't cancel siblings.

## Options

| Option | Default |
|---|---|
| `WithMaxIterations(n)` | 10 |
| `WithToolTimeout(d)` | 30s |
| `WithMaxTokens(n)` | 4096 |
| `WithTemperature(f)` | 0.7 |
| `WithLogger(l)` | `slog.Default()` |

## Project Structure

```
pkg/core/          # Provider, Tool, MemoryStore interfaces — zero external deps
pkg/providers/     # Anthropic, OpenAI, Ollama adapters
pkg/tools/         # Calculator, WebSearch, FuncTool, Schema builder
pkg/memory/        # InMemoryStore
pkg/mcp/           # MCP client — Streamable HTTP and Stdio transports
pkg/orchestrator/  # Sequential and Parallel runners
examples/          # single-agent, multi-agent, mcp-agent
tests/             # Unit tests (mock provider, 14 scenarios)
```

## Running Tests

```bash
# Unit tests (no API key needed)
go test ./...

# Integration tests (requires API keys)
ANTHROPIC_API_KEY=sk-... go test -tags=integration ./tests/integration/...
OPENAI_API_KEY=sk-...    go test -tags=integration ./tests/integration/...
```
