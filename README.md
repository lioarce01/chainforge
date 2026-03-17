# chainforge

A provider-agnostic Go agent orchestration framework. Import it as a library to build AI applications without touching any LLM SDK directly.

[![Documentation](https://img.shields.io/badge/docs-chainforge.mintlify.app-6366F1?style=flat&logo=gitbook&logoColor=white)](https://chainforge.mintlify.app)

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
- **Vector memory** — Qdrant adapter for semantic search over conversation history
- **HTTP server** — production-ready chi router with SSE streaming, CORS, and graceful shutdown
- **OpenTelemetry** — distributed tracing via `pkg/middleware/otel`; plug-in without changing agent code

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

## Benchmarks

All benchmarks run on AMD Ryzen 7 7800X3D (16 threads) with a zero-latency mock provider — numbers reflect pure framework overhead, not network or model time.

```
go test ./tests/bench/... -bench=. -benchmem
```

### Agent loop

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `AgentRun` (single session) | 3,582 | 1,324 | 17 |
| `AgentRunWithTool` (tool dispatch) | 3,677 | 1,482 | 19 |
| `AgentConcurrent` (8 goroutines) | 6,307 | 5,269 | 15 |
| `AgentRunStream` (stream drain) | 10,074 | 10,297 | 33 |

Tool dispatch adds ~100 ns over a plain `AgentRun`. Concurrent sessions scale linearly with no lock contention between independent sessions.

### Memory store

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `InMemoryAppend` | 213 | 459 | **0** |
| `InMemoryConcurrentSessions` | 292 | 491 | 1 |
| `InMemoryGet` (10 messages) | 251 | 896 | 1 |
| `InMemoryGet` (100 messages) | 2,520 | 9,472 | 1 |
| `InMemoryGet` (1000 messages) | 26,485 | 90,112 | 1 |

Append is allocation-free. Get allocates a single slice regardless of history length.

### Streaming

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `StreamConcurrent` | 3,251 | 14,082 | 31 |
| `StreamDrain` (1 KB response, 64 B chunks) | 11,282 | 14,737 | 33 |
| `StreamChunkSizes/chunk=256` | 9,166 | 14,640 | 25 |
| `StreamChunkSizes/chunk=1024` | 5,748 | 7,580 | 18 |

Larger chunks reduce allocations proportionally. Concurrent stream draining outperforms sequential due to goroutine scheduling overlap.

### E2E latency (real provider)

Measured against `openrouter/hunter-alpha` via OpenRouter (20 requests, 4 concurrent workers):

| Metric | Value |
|---|---|
| p50 | 3.86 s |
| p95 | 5.80 s |
| p99 | 5.80 s |
| mean | 3.88 s |
| RPS | 0.90 |
| errors | 0 |

```bash
OPENAI_API_KEY=sk-... go run ./cmd/bench/main.go \
  --config config.yaml \
  --concurrency 4 --requests 20 --warmup 2
```

## Project Structure

```
pkg/core/           # Provider, Tool, MemoryStore interfaces — zero external deps
pkg/providers/      # Anthropic, OpenAI, Ollama adapters
pkg/tools/          # Calculator, WebSearch, FuncTool, Schema builder
pkg/memory/         # InMemoryStore, Qdrant vector store
pkg/mcp/            # MCP client — Streamable HTTP and Stdio transports
pkg/orchestrator/   # Sequential and Parallel runners
pkg/middleware/     # Logging and OpenTelemetry middleware (wrap any provider)
pkg/benchutil/      # MockProvider, MockToolProvider, LatencyRecorder
pkg/server/         # HTTP server — config, SSE adapter, chi router, handlers
cmd/server/         # Production binary with graceful shutdown
cmd/bench/          # E2E latency CLI (--mock / --concurrency / --requests)
deploy/             # Dockerfile, docker-compose, k8s manifests, Helm chart
examples/           # single-agent, multi-agent, mcp-agent, server-agent
tests/bench/        # 16 micro-benchmarks (agent, memory, streaming)
tests/              # Unit tests (mock provider, 14 scenarios)
```

## Running Tests

```bash
# Unit tests (no API key needed)
go test ./...

# Integration tests (requires API keys)
ANTHROPIC_API_KEY=sk-... go test -tags=integration ./tests/integration/...
OPENAI_API_KEY=sk-...    go test -tags=integration ./tests/integration/...

# Micro-benchmarks (no API key needed)
go test ./tests/bench/... -bench=. -benchmem

# Stable numbers (longer run)
go test ./tests/bench/... -bench=. -benchmem -benchtime=10s

# E2E latency benchmark (mock — no API key)
go run ./cmd/bench/main.go --mock --concurrency 4 --requests 50 --warmup 5
```
