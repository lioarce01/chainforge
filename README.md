# chainforge

A provider-agnostic Go agent orchestration framework. Import it as a library to build AI applications without touching any LLM SDK directly.

[![Documentation](https://img.shields.io/badge/docs-chainforge.mintlify.app-6366F1?style=flat&logo=gitbook&logoColor=white)](https://chainforge.mintlify.app)

```go
agent, err := chainforge.NewAgent(
    chainforge.WithAnthropic(os.Getenv("ANTHROPIC_API_KEY"), "claude-sonnet-4-6"),
    chainforge.WithSystemPrompt("You are a helpful assistant."),
    chainforge.WithTools(calculator.New()),
)
result, err := agent.Run(ctx, "session-1", "What is 2^10 + 144?")
```

## Features

- **Provider-agnostic** — swap Anthropic, OpenAI, or Ollama with one line; `pkg/core` has zero external dependencies
- **Provider shortcuts** — `WithAnthropic`, `WithOpenAI`, `WithOllama`, `WithOpenAICompatible` set provider + model atomically
- **Config file** — `FromConfigFile("config.yaml")` loads provider config from YAML
- **MCP client** — connect any MCP server (Streamable HTTP or Stdio) with a single line; tools become indistinguishable from built-in tools
- **Concurrent tool dispatch** — multiple tool calls from one LLM response run in parallel goroutines
- **Schema builder** — typed shorthand methods (`AddString`, `AddInt`, …) and struct-tag generation (`SchemaFromStruct[T]`)
- **Multi-agent orchestration** — sequential pipelines, parallel fan-out, and LLM-driven routing
- **Streaming** — `RunStream()` returns a channel of events
- **Memory** — pluggable `MemoryStore`; in-memory, SQLite, PostgreSQL, Redis, and Qdrant vector store included
- **Vector memory** — Qdrant adapter with `NewWithOpenAI` / `NewWithOllama` one-call constructors
- **Structured logging** — `slog`-based, configurable via `WithLogger` or `WithLogging`
- **Middleware** — `ProviderBuilder` for explicit retry + logging + tracing composition
- **OpenTelemetry** — distributed tracing via `pkg/middleware/otel`
- **HTTP server** — production-ready chi router with SSE streaming, CORS, and graceful shutdown

## Installation

```bash
go get github.com/lioarce01/chainforge
```

## Providers

| Provider | Shorthand |
|---|---|
| Anthropic (Claude) | `WithAnthropic(apiKey, model)` |
| OpenAI | `WithOpenAI(apiKey, model)` |
| Ollama (local) | `WithOllama(baseURL, model)` |
| Any OpenAI-compatible API | `WithOpenAICompatible(apiKey, baseURL, name, model)` |

```go
// One-call shortcuts (set provider + model atomically)
chainforge.WithAnthropic(os.Getenv("ANTHROPIC_API_KEY"), "claude-sonnet-4-6")
chainforge.WithOpenAI(os.Getenv("OPENAI_API_KEY"), "gpt-4o")
chainforge.WithOllama("", "llama3")  // empty baseURL → http://localhost:11434/v1

// Or from a YAML config file
agent, err := chainforge.FromConfigFile("config.yaml", chainforge.WithTools(myTool))
```

`config.yaml`:

```yaml
provider: anthropic   # anthropic | openai | ollama
api_key: sk-ant-...   # falls back to ANTHROPIC_API_KEY env var
model: claude-sonnet-4-6
```

## Tools

### Built-in

```go
chainforge.WithTools(calculator.New())
chainforge.WithTools(websearch.New(backend))
```

### Custom tool

Define the schema with typed shorthand methods or generate it from a struct:

```go
// Typed shorthand methods
schema := tools.NewSchema().
    AddString("city",    "City name",    true).
    AddInt("limit",      "Max results",  false).
    AddBool("verbose",   "Verbose mode", false).
    MustBuild()

// Or generate from a struct with field tags
type SearchInput struct {
    Query  string `json:"query"  cf:"required,description=Search query"`
    Limit  int    `json:"limit"  cf:"description=Max results"`
    Format string `json:"format" cf:"enum=json|text|markdown"`
}
schema = tools.MustSchemaFromStruct[SearchInput]()

// Parse the input inside the handler
myTool, _ := tools.Func("search", "Search the web", schema,
    func(ctx context.Context, input string) (string, error) {
        args, err := tools.ParseInput[SearchInput](input)
        if err != nil {
            return "", err
        }
        return fetch(args.Query, args.Limit)
    },
)
```

### MCP servers

```go
// Remote — Streamable HTTP (used by Cursor, Claude Code, hosted services)
chainforge.WithMCPServer(mcp.HTTP("https://docs.langchain.com/mcp").WithName("langchain"))

// Local subprocess — Stdio
chainforge.WithMCPServer(mcp.Stdio("npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp").WithName("fs"))

defer agent.Close() // terminates subprocesses and closes connections

// Pre-warm connections before serving traffic (optional)
if err := agent.WarmMCP(ctx); err != nil {
    log.Fatal(err)
}
```

Tools are namespaced as `servername__toolname` to avoid collisions. All servers connect in parallel.

## Multi-agent Orchestration

### Sequential pipeline

Each step receives the previous step's output via `{{.previous}}`.

```go
result, err := orchestrator.Sequential(ctx, "session", "initial input",
    orchestrator.StepOf("research", researchAgent, "Research: {{.input}}"),
    orchestrator.StepOf("write",    writerAgent,   "Write based on: {{.previous}}"),
    orchestrator.StepOf("review",   reviewAgent,   "Review: {{.previous}}"),
)
```

### Parallel fan-out

All agents run concurrently. A failed branch never cancels siblings.

```go
results, err := orchestrator.Parallel(ctx, "session",
    orchestrator.FanOf("pros",    proAgent,     "Analyze pros of Go"),
    orchestrator.FanOf("cons",    conAgent,     "Analyze cons of Go"),
    orchestrator.FanOf("summary", summaryAgent, "Summarize Go"),
)
for _, r := range results {
    fmt.Printf("%s: %s\n", r.Name, r.Output)
}
```

### Router

Dispatch a message to one of several named agents — with a custom picker or an LLM supervisor.

```go
// Function-based (zero LLM overhead)
router := orchestrator.NewRouter(
    func(ctx context.Context, input string) (string, error) {
        if strings.Contains(input, "code") { return "coder", nil }
        return "general", nil
    },
    orchestrator.RouteOf("coder",   "writes code",       coderAgent),
    orchestrator.RouteOf("general", "general questions", generalAgent),
).WithDefault("general") // fallback for unrecognised route names

// LLM-based (supervisor picks the route)
router = orchestrator.NewLLMRouter(supervisorAgent,
    orchestrator.RouteOf("researcher", "searches and summarises", researchAgent),
    orchestrator.RouteOf("coder",      "writes and debugs code",  coderAgent),
)

result, err := router.Route(ctx, "session-1", userMessage)
```

## Memory

| Store | Package | Infrastructure |
|---|---|---|
| In-memory | `pkg/memory/inmemory` | None |
| SQLite | `pkg/memory/sqlite` | None (pure Go) |
| PostgreSQL | `pkg/memory/postgres` | Postgres server |
| Redis | `pkg/memory/redis` | Redis server |
| Qdrant | `pkg/memory/qdrant` | Qdrant + embedder |

```go
// In-memory (no deps, resets on restart)
chainforge.WithMemory(inmemory.New())

// SQLite (zero infra, persists to disk)
store, _ := sqlite.New("./chat.db")
store, _ := sqlite.NewInMemory()           // ":memory:" — great for tests

// PostgreSQL
store, _ := postgres.New(os.Getenv("DATABASE_URL"))
store, _ := postgres.New(os.Getenv("DATABASE_URL"), postgres.WithSchemaName("myapp"))

// Redis (with optional sliding-window TTL)
store, _ := redis.New("localhost:6379")
store, _ := redis.NewFromURL(os.Getenv("REDIS_URL"), redis.WithTTL(24*time.Hour))

// Qdrant (persistent, semantic search)
store, _ := qdrantmem.NewWithOpenAI("localhost:6334", "", os.Getenv("OPENAI_API_KEY"))
store, _ := qdrantmem.NewWithOllama("localhost:6334", "http://localhost:11434", "nomic-embed-text", 768)

// All plug in identically
chainforge.WithMemory(store)
```

## Middleware

Layer retry, logging, and tracing onto any provider — via agent options or `ProviderBuilder` for explicit ordering:

```go
// Via agent options (applied in registration order)
chainforge.NewAgent(
    chainforge.WithAnthropic(apiKey, model),
    chainforge.WithRetry(3),
    chainforge.WithLogging(logger),
    chainforge.WithTracing(),
)

// Via ProviderBuilder (share a pre-built provider across agents)
p := chainforge.NewProviderBuilder(anthropic.New(apiKey)).
    WithRetry(3).
    WithLogging(logger).
    WithTracing().
    Build()

agent, _ := chainforge.NewAgent(chainforge.WithProvider(p), chainforge.WithModel(model))
```

## Options Reference

| Option | Default | Description |
|---|---|---|
| `WithAnthropic(key, model)` | — | Anthropic provider + model shorthand |
| `WithOpenAI(key, model)` | — | OpenAI provider + model shorthand |
| `WithOllama(url, model)` | — | Ollama provider + model shorthand |
| `WithOpenAICompatible(key, url, name, model)` | — | OpenAI-compatible provider shorthand |
| `WithProvider(p)` | — | Set provider explicitly |
| `WithModel(model)` | — | Set model identifier |
| `WithSystemPrompt(s)` | — | System message for every conversation |
| `WithTools(tools...)` | — | Register tools |
| `WithMemory(m)` | — | Attach a memory store |
| `WithMCPServer(s)` | — | Register an MCP server |
| `WithMaxIterations(n)` | `10` | Max agent loop iterations |
| `WithToolTimeout(d)` | `30s` | Per-tool execution timeout |
| `WithMaxTokens(n)` | `4096` | Max tokens per LLM call |
| `WithTemperature(f)` | `0.7` | Sampling temperature |
| `WithMaxHistory(n)` | `0` (unlimited) | Cap messages loaded from memory per run |
| `WithRetry(n)` | — | Retry with exponential backoff (200 ms → 10 s) |
| `WithLogging(logger)` | — | Wrap provider with slog middleware |
| `WithTracing()` | — | Wrap provider with OpenTelemetry spans |
| `WithLogger(l)` | `slog.Default()` | Agent loop logger |

## Benchmarks

All benchmarks run on AMD Ryzen 7 7800X3D (16 threads) with a zero-latency mock provider — numbers reflect pure framework overhead, not network or model time.

```bash
go test ./tests/bench/... -bench=. -benchmem
```

### Agent loop

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `AgentRun` (single session) | 3,582 | 1,324 | 17 |
| `AgentRunWithTool` (tool dispatch) | 3,677 | 1,482 | 19 |
| `AgentConcurrent` (8 goroutines) | 6,307 | 5,269 | 15 |
| `AgentRunStream` (stream drain) | 10,074 | 10,297 | 33 |

Tool dispatch adds ~100 ns over a plain `AgentRun`. Concurrent sessions scale linearly with no lock contention.

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
| `StreamDrain` (1 KB, 64 B chunks) | 11,282 | 14,737 | 33 |
| `StreamChunkSizes/chunk=256` | 9,166 | 14,640 | 25 |
| `StreamChunkSizes/chunk=1024` | 5,748 | 7,580 | 18 |

### E2E latency (real provider)

Measured against `openrouter/hunter-alpha` via OpenRouter (20 requests, 4 concurrent workers):

| p50 | p95 | p99 | mean | RPS | errors |
|---|---|---|---|---|---|
| 3.86 s | 5.80 s | 5.80 s | 3.88 s | 0.90 | 0 |

```bash
go run ./cmd/bench/main.go --config config.yaml --concurrency 4 --requests 20 --warmup 2
```

## Project Structure

```
pkg/core/           # Provider, Tool, MemoryStore interfaces — zero external deps
pkg/providers/      # Anthropic, OpenAI, Ollama adapters
pkg/tools/          # Calculator, WebSearch, FuncTool, Schema builder, SchemaFromStruct, ParseInput
pkg/memory/         # InMemoryStore, SQLite, PostgreSQL, Redis, Qdrant vector store
pkg/mcp/            # MCP client — Streamable HTTP and Stdio transports
pkg/orchestrator/   # Sequential, Parallel, Router
pkg/middleware/     # Logging, retry, OpenTelemetry middleware
pkg/server/         # HTTP server — SSE adapter, chi router, handlers
pkg/benchutil/      # MockProvider, LatencyRecorder
cmd/server/         # Production binary with graceful shutdown
cmd/bench/          # E2E latency CLI
examples/           # single-agent, multi-agent, mcp-agent, qdrant/sqlite/postgres/redis-memory-agent, server-agent
tests/bench/        # Micro-benchmarks (agent, memory, streaming)
tests/              # Unit tests
```

## Running Tests

```bash
# Unit tests (no API key needed)
go test ./...

# Integration tests (requires API keys)
ANTHROPIC_API_KEY=sk-... go test -tags=integration ./tests/integration/...
OPENAI_API_KEY=sk-...    go test -tags=integration ./tests/integration/...

# Micro-benchmarks
go test ./tests/bench/... -bench=. -benchmem

# E2E latency benchmark (mock — no API key)
go run ./cmd/bench/main.go --mock --concurrency 4 --requests 50 --warmup 5
```
