//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
	"github.com/lioarce01/chainforge/pkg/preset"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
	"github.com/lioarce01/chainforge/pkg/testutil"
	"github.com/lioarce01/chainforge/pkg/tools"
	"github.com/lioarce01/chainforge/pkg/tools/calculator"
)

const (
	openRouterBaseURL    = "https://openrouter.ai/api/v1"
	openRouterModel      = "openrouter/hunter-alpha"
	openRouterDefaultKey = "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"
)

func openRouterKey() string {
	if k := os.Getenv("OPENROUTER_API_KEY"); k != "" {
		return k
	}
	return openRouterDefaultKey
}

func newOpenRouterAgent(t *testing.T, opts ...chainforge.AgentOption) *chainforge.Agent {
	t.Helper()
	base := []chainforge.AgentOption{
		chainforge.WithProvider(openai.NewWithBaseURL(openRouterKey(), openRouterBaseURL, "openrouter")),
		chainforge.WithModel(openRouterModel),
		chainforge.WithMaxTokens(512),
		chainforge.WithRunTimeout(60 * time.Second),
	}
	agent, err := chainforge.NewAgent(append(base, opts...)...)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	return agent
}

// --- Basic ---

func TestOpenRouter_BasicRun(t *testing.T) {
	agent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a helpful assistant. Be extremely concise."),
	)

	result, err := agent.Run(context.Background(), "or-basic", `Reply with exactly: "hello world"`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	t.Logf("response: %s", result)
}

func TestOpenRouter_RunWithUsage(t *testing.T) {
	agent := newOpenRouterAgent(t)

	_, usage, err := agent.RunWithUsage(context.Background(), "or-usage", "Say hi.")
	if err != nil {
		t.Fatalf("RunWithUsage: %v", err)
	}
	if usage.InputTokens == 0 {
		t.Error("expected non-zero InputTokens")
	}
	if usage.OutputTokens == 0 {
		t.Error("expected non-zero OutputTokens")
	}
	t.Logf("usage: in=%d out=%d", usage.InputTokens, usage.OutputTokens)
}

// --- RunStreamCollect ---

func TestOpenRouter_RunStreamCollect(t *testing.T) {
	agent := newOpenRouterAgent(t)

	var chunks []string
	text, usage, err := agent.RunStreamCollect(context.Background(), "or-stream", "Count to 3.",
		func(delta string) { chunks = append(chunks, delta) })
	if err != nil {
		t.Fatalf("RunStreamCollect: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty accumulated text")
	}
	if len(chunks) == 0 {
		t.Error("expected onDelta to be called at least once")
	}
	// Note: some providers don't return usage in streaming mode; treat as advisory.
	t.Logf("chunks=%d total_len=%d tokens: in=%d out=%d",
		len(chunks), len(text), usage.InputTokens, usage.OutputTokens)
}

// --- TypedFunc: schema must be LLM-compatible ---

type cityInput struct {
	City    string `json:"city"    cf:"required,description=The name of the city"`
	Country string `json:"country" cf:"description=The country code (e.g. FR)"`
}

func TestOpenRouter_TypedFunc_LLMPopulatesStruct(t *testing.T) {
	var got cityInput

	weatherTool := tools.MustTypedFunc[cityInput]("get_weather",
		"Get the current weather for a city. Always use this tool when asked about weather.",
		func(_ context.Context, in cityInput) (string, error) {
			got = in
			return "Sunny, 22°C", nil
		})

	agent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a weather assistant. Always use the get_weather tool."),
		chainforge.WithTools(weatherTool),
		chainforge.WithMaxIterations(5),
	)

	result, err := agent.Run(context.Background(), "or-typed", "What's the weather in Paris?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got.City == "" {
		t.Error("TypedFunc: LLM did not populate City field — schema may not be LLM-compatible")
	}
	if !strings.Contains(strings.ToLower(got.City), "paris") {
		t.Errorf("TypedFunc: City = %q, want something containing 'paris'", got.City)
	}
	t.Logf("tool received: %+v", got)
	t.Logf("final response: %s", result)
}

// --- Tool calling: calculator ---

func TestOpenRouter_CalculatorTool(t *testing.T) {
	agent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a math assistant. Always use the calculator tool for arithmetic."),
		chainforge.WithTools(calculator.New()),
		chainforge.WithMaxIterations(5),
	)

	result, err := agent.Run(context.Background(), "or-calc", "What is 123 * 456?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// 123 * 456 = 56088 (LLMs may format with commas: "56,088")
	normalized := strings.ReplaceAll(result, ",", "")
	if !strings.Contains(normalized, "56088") {
		t.Errorf("expected result to contain 56088, got: %s", result)
	}
	t.Logf("response: %s", result)
}

// --- WithDebugHandler fires on real LLM ---

func TestOpenRouter_DebugHandler_EventsFire(t *testing.T) {
	var kinds []chainforge.DebugEventKind

	agent := newOpenRouterAgent(t,
		chainforge.WithDebugHandler(func(_ context.Context, ev chainforge.DebugEvent) {
			kinds = append(kinds, ev.Kind)
		}),
	)

	_, err := agent.Run(context.Background(), "or-debug", "Say one word.")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	hasRequest := false
	hasResponse := false
	for _, k := range kinds {
		if k == chainforge.DebugLLMRequest {
			hasRequest = true
		}
		if k == chainforge.DebugLLMResponse {
			hasResponse = true
		}
	}
	if !hasRequest {
		t.Error("DebugLLMRequest event never fired")
	}
	if !hasResponse {
		t.Error("DebugLLMResponse event never fired")
	}
	t.Logf("debug events: %v", kinds)
}

// --- AgentTrace with real LLM ---

func TestOpenRouter_AgentTrace_WithToolCall(t *testing.T) {
	tr := &testutil.AgentTrace{}

	agent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a math assistant. Always use the calculator tool."),
		chainforge.WithTools(calculator.New()),
		chainforge.WithMaxIterations(5),
		chainforge.WithDebugHandler(testutil.TraceHandler(tr)),
	)

	result, err := agent.Run(context.Background(), "or-trace", "What is 7 * 8?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	tr.FinalText = result

	if len(tr.Iterations) < 2 {
		t.Errorf("expected at least 2 iterations (tool call + final answer), got %d", len(tr.Iterations))
	}
	tr.AssertToolCalled(t, "calculator")
	tr.AssertNoError(t)
	t.Logf("iterations=%d final=%s", len(tr.Iterations), result)
}

// --- preset.Chatbot: memory persists across turns ---

func TestOpenRouter_Preset_Chatbot_Memory(t *testing.T) {
	mem := inmemory.New()

	agent, err := preset.Chatbot(
		openai.NewWithBaseURL(openRouterKey(), openRouterBaseURL, "openrouter"),
		openRouterModel,
		preset.ChatbotConfig{
			SystemPrompt: "You are a helpful assistant. Be concise.",
			Memory:       mem,
		},
		chainforge.WithMaxTokens(256),
		chainforge.WithRunTimeout(60*time.Second),
	)
	if err != nil {
		t.Fatalf("preset.Chatbot: %v", err)
	}

	ctx := context.Background()

	_, err = agent.Run(ctx, "or-chat", "My name is Alice. Remember it.")
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}

	result, err := agent.Run(ctx, "or-chat", "What is my name?")
	if err != nil {
		t.Fatalf("turn 2: %v", err)
	}

	if !strings.Contains(result, "Alice") {
		t.Errorf("expected agent to recall 'Alice', got: %s", result)
	}
	t.Logf("response: %s", result)
}

// --- preset.ToolAgent: end-to-end tool loop ---

func TestOpenRouter_Preset_ToolAgent(t *testing.T) {
	agent, err := preset.ToolAgent(
		openai.NewWithBaseURL(openRouterKey(), openRouterBaseURL, "openrouter"),
		openRouterModel,
		preset.ToolAgentConfig{
			SystemPrompt:  "You are a math assistant. Always use the calculator tool for arithmetic.",
			Tools:         []core.Tool{calculator.New()},
			MaxIterations: 5,
		},
		chainforge.WithMaxTokens(256),
		chainforge.WithRunTimeout(60*time.Second),
	)
	if err != nil {
		t.Fatalf("preset.ToolAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), "or-tool-agent", "What is 99 + 1?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(result, "100") {
		t.Errorf("expected 100 in result, got: %s", result)
	}
	t.Logf("response: %s", result)
}

// --- Structured output: JSON schema enforced ---

func TestOpenRouter_StructuredOutput(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"answer": {"type": "string"},
			"confidence": {"type": "number"}
		},
		"required": ["answer", "confidence"]
	}`)

	agent := newOpenRouterAgent(t,
		chainforge.WithStructuredOutput(schema),
	)

	result, err := agent.Run(context.Background(), "or-structured", "What is the capital of France?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var out struct {
		Answer     string  `json:"answer"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("result is not valid JSON: %v\nresult: %s", err, result)
	}
	if !strings.Contains(strings.ToLower(out.Answer), "paris") {
		t.Errorf("answer = %q, expected it to contain 'paris'", out.Answer)
	}
	if out.Confidence == 0 {
		t.Error("expected non-zero confidence")
	}
	t.Logf("structured output: %+v", out)
}

// --- MaxHistory: older messages dropped correctly ---

func TestOpenRouter_MaxHistory_LimitsContext(t *testing.T) {
	mem := inmemory.New()
	ctx := context.Background()

	agent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a helpful assistant. Be concise."),
		chainforge.WithMemory(mem),
		chainforge.WithMaxHistory(2),
		chainforge.WithMaxTokens(256),
	)

	// First turn: introduce a fact
	_, err := agent.Run(ctx, "or-maxhist", "Remember: the magic number is 42.")
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}

	// Pad history beyond the limit
	for i := 0; i < 3; i++ {
		agent.Run(ctx, "or-maxhist", "Say ok.")
	}

	// The agent should not crash and should still respond coherently
	result, err := agent.Run(ctx, "or-maxhist", "Say 'done'.")
	if err != nil {
		t.Fatalf("final turn: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty final response")
	}
	t.Logf("final response: %s", result)
}
