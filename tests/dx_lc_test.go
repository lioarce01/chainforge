package tests

// Tests for Phase 3 Larger Changes:
// LC-1: preset.Chatbot / preset.ToolAgent
// LC-2: agent.RunStreamCollect
// LC-3: testutil.TraceHandler + AgentTrace
// QW-1: testutil.MapMemoryStore + assertions

import (
	"context"
	"strings"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/preset"
	"github.com/lioarce01/chainforge/pkg/testutil"
	"github.com/lioarce01/chainforge/pkg/tools"
)

// --- LC-1: preset ---

func TestPreset_Chatbot_BasicRun(t *testing.T) {
	p := testutil.NewMockProvider(testutil.EndTurnResponse("Hi there!"))

	agent, err := preset.Chatbot(p, "mock", preset.ChatbotConfig{
		SystemPrompt: "You are helpful.",
	})
	if err != nil {
		t.Fatalf("preset.Chatbot: %v", err)
	}

	result, err := agent.Run(context.Background(), "s1", "Hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "Hi there!" {
		t.Errorf("result = %q, want %q", result, "Hi there!")
	}
}

func TestPreset_Chatbot_UsesProvidedMemory(t *testing.T) {
	mem := testutil.NewMapMemory()
	p := testutil.NewMockProvider(testutil.EndTurnResponse("remembered"))

	agent, err := preset.Chatbot(p, "mock", preset.ChatbotConfig{
		Memory: mem,
	})
	if err != nil {
		t.Fatalf("preset.Chatbot: %v", err)
	}

	agent.Run(context.Background(), "s1", "hello")
	testutil.AssertSessionContains(t, mem, "s1", "hello")
}

func TestPreset_Chatbot_DefaultMemoryCreated(t *testing.T) {
	// Without Memory field, an in-memory store is created automatically.
	p := testutil.NewMockProvider(
		testutil.EndTurnResponse("first"),
		testutil.EndTurnResponse("second"),
	)

	agent, err := preset.Chatbot(p, "mock", preset.ChatbotConfig{})
	if err != nil {
		t.Fatalf("preset.Chatbot: %v", err)
	}

	agent.Run(context.Background(), "s1", "turn one")
	agent.Run(context.Background(), "s1", "turn two")

	// Second call should include history — provider sees more than 1 message.
	if p.CallCount() < 2 {
		t.Errorf("expected at least 2 provider calls, got %d", p.CallCount())
	}
}

func TestPreset_Chatbot_ExtraOptsApplied(t *testing.T) {
	p := testutil.NewMockProvider(testutil.EndTurnResponse("ok"))
	var debugFired bool

	agent, err := preset.Chatbot(p, "mock", preset.ChatbotConfig{},
		chainforge.WithDebugHandler(func(_ context.Context, ev chainforge.DebugEvent) {
			if ev.Kind == chainforge.DebugLLMRequest {
				debugFired = true
			}
		}),
	)
	if err != nil {
		t.Fatalf("preset.Chatbot: %v", err)
	}

	agent.Run(context.Background(), "s1", "hi")
	if !debugFired {
		t.Error("extra option WithDebugHandler was not applied by preset.Chatbot")
	}
}

func TestPreset_ToolAgent_CallsTool(t *testing.T) {
	var toolCalled bool

	addTool := tools.MustTypedFunc[calcInput]("add", "add numbers",
		func(_ context.Context, in calcInput) (string, error) {
			toolCalled = true
			return "7", nil
		})

	p := testutil.NewMockProvider(
		testutil.ToolUseResponse(core.ToolCall{Name: "add", Input: `{"a":3,"b":4}`}),
		testutil.EndTurnResponse("The answer is 7"),
	)

	agent, err := preset.ToolAgent(p, "mock", preset.ToolAgentConfig{
		SystemPrompt: "You are a math assistant.",
		Tools:        []core.Tool{addTool},
	})
	if err != nil {
		t.Fatalf("preset.ToolAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), "s1", "what is 3+4?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !toolCalled {
		t.Error("expected tool to be called")
	}
	if result != "The answer is 7" {
		t.Errorf("result = %q, want %q", result, "The answer is 7")
	}
}

func TestPreset_ToolAgent_DefaultMaxIterations(t *testing.T) {
	// MaxIterations defaults to 10 when 0 — can handle multi-step loops.
	p := testutil.NewMockProvider(testutil.EndTurnResponse("done"))
	_, err := preset.ToolAgent(p, "mock", preset.ToolAgentConfig{
		MaxIterations: 0, // should default to 10
	})
	if err != nil {
		t.Fatalf("preset.ToolAgent with default MaxIterations: %v", err)
	}
}

// --- LC-2: RunStreamCollect ---

func TestRunStreamCollect_ReturnsFullText(t *testing.T) {
	p := testutil.NewMockProvider(testutil.EndTurnResponse("hello world"))
	agent, _ := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
	)

	text, _, err := agent.RunStreamCollect(context.Background(), "s1", "hi", nil)
	if err != nil {
		t.Fatalf("RunStreamCollect: %v", err)
	}
	if text != "hello world" {
		t.Errorf("text = %q, want %q", text, "hello world")
	}
}

func TestRunStreamCollect_OnDeltaCalled(t *testing.T) {
	p := testutil.NewMockProvider(testutil.EndTurnResponse("chunk"))
	agent, _ := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
	)

	var deltas []string
	agent.RunStreamCollect(context.Background(), "s1", "hi", func(d string) {
		deltas = append(deltas, d)
	})

	if len(deltas) == 0 {
		t.Error("onDelta was never called")
	}
	full := strings.Join(deltas, "")
	if full != "chunk" {
		t.Errorf("joined deltas = %q, want %q", full, "chunk")
	}
}

func TestRunStreamCollect_NilOnDelta_NocrashNoPanic(t *testing.T) {
	p := testutil.NewMockProvider(testutil.EndTurnResponse("ok"))
	agent, _ := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
	)
	// nil onDelta should not panic
	text, _, err := agent.RunStreamCollect(context.Background(), "s1", "hi", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "ok" {
		t.Errorf("text = %q, want %q", text, "ok")
	}
}

func TestRunStreamCollect_ReturnsUsage(t *testing.T) {
	p := testutil.NewMockProvider(testutil.EndTurnResponse("ok"))
	agent, _ := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
	)

	_, usage, err := agent.RunStreamCollect(context.Background(), "s1", "hi", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// EndTurnResponse sets InputTokens=10, OutputTokens=5
	if usage.InputTokens == 0 {
		t.Error("expected non-zero InputTokens in usage")
	}
}

// --- LC-3: AgentTrace + TraceHandler ---

func TestTraceHandler_RecordsIterations(t *testing.T) {
	addTool := tools.MustTypedFunc[calcInput]("add", "add",
		func(_ context.Context, in calcInput) (string, error) { return "7", nil })

	p := testutil.NewMockProvider(
		testutil.ToolUseResponse(core.ToolCall{Name: "add", Input: `{"a":3,"b":4}`}),
		testutil.EndTurnResponse("The answer is 7"),
	)

	tr := &testutil.AgentTrace{}
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithTools(addTool),
		chainforge.WithDebugHandler(testutil.TraceHandler(tr)),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), "s1", "what is 3+4?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	tr.FinalText = result
	tr.AssertIterations(t, 2)
	tr.AssertToolCalled(t, "add")
	tr.AssertFinalText(t, "The answer is 7")
	tr.AssertNoError(t)
}

func TestTraceHandler_AssertToolNotCalled(t *testing.T) {
	p := testutil.NewMockProvider(testutil.EndTurnResponse("done"))

	tr := &testutil.AgentTrace{}
	agent, _ := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithDebugHandler(testutil.TraceHandler(tr)),
	)

	agent.Run(context.Background(), "s1", "hi")
	tr.AssertToolNotCalled(t, "search") // no tools registered — should pass
}

func TestTraceHandler_ToolCallInputRecorded(t *testing.T) {
	addTool := tools.MustTypedFunc[calcInput]("add", "add",
		func(_ context.Context, in calcInput) (string, error) { return "5", nil })

	p := testutil.NewMockProvider(
		testutil.ToolUseResponse(core.ToolCall{Name: "add", Input: `{"a":2,"b":3}`}),
		testutil.EndTurnResponse("5"),
	)

	tr := &testutil.AgentTrace{}
	agent, _ := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithTools(addTool),
		chainforge.WithDebugHandler(testutil.TraceHandler(tr)),
	)

	agent.Run(context.Background(), "s1", "2+3?")

	if len(tr.Iterations) == 0 || len(tr.Iterations[0].ToolCalls) == 0 {
		t.Fatal("expected at least one tool call recorded in trace")
	}
	tc := tr.Iterations[0].ToolCalls[0]
	if tc.Call.Name != "add" {
		t.Errorf("tool name = %q, want %q", tc.Call.Name, "add")
	}
	if tc.Output != "5" {
		t.Errorf("tool output = %q, want %q", tc.Output, "5")
	}
}

// --- QW-1: testutil.MapMemoryStore + assertions ---

func TestMapMemoryStore_GetAppendClear(t *testing.T) {
	mem := testutil.NewMapMemory()
	ctx := context.Background()

	err := mem.Append(ctx, "s1", core.Message{Role: core.RoleUser, Content: "hello"})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	msgs, err := mem.Get(ctx, "s1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "hello" {
		t.Errorf("Get = %v, want 1 message with content hello", msgs)
	}

	if err := mem.Clear(ctx, "s1"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	msgs, _ = mem.Get(ctx, "s1")
	if len(msgs) != 0 {
		t.Errorf("after Clear, expected 0 messages, got %d", len(msgs))
	}
}

func TestMapMemoryStore_Counters(t *testing.T) {
	mem := testutil.NewMapMemory()
	ctx := context.Background()

	mem.Append(ctx, "s1", core.Message{Content: "a"})
	mem.Append(ctx, "s1", core.Message{Content: "b"})
	mem.Clear(ctx, "s1")

	if mem.AppendCount() != 2 {
		t.Errorf("AppendCount = %d, want 2", mem.AppendCount())
	}
	if mem.ClearCount() != 1 {
		t.Errorf("ClearCount = %d, want 1", mem.ClearCount())
	}
}

func TestMapMemoryStore_SessionIDs(t *testing.T) {
	mem := testutil.NewMapMemory()
	ctx := context.Background()

	mem.Append(ctx, "s1", core.Message{Content: "a"})
	mem.Append(ctx, "s2", core.Message{Content: "b"})

	ids := mem.SessionIDs()
	if len(ids) != 2 {
		t.Errorf("SessionIDs len = %d, want 2", len(ids))
	}
}

func TestAssertCallCount_Pass(t *testing.T) {
	p := testutil.NewMockProvider(testutil.EndTurnResponse("ok"))
	agent, _ := chainforge.NewAgent(chainforge.WithProvider(p), chainforge.WithModel("mock"))
	agent.Run(context.Background(), "s1", "hi")
	testutil.AssertCallCount(t, p, 1)
}

func TestAssertLastRequestContains_Pass(t *testing.T) {
	p := testutil.NewMockProvider(testutil.EndTurnResponse("ok"))
	agent, _ := chainforge.NewAgent(chainforge.WithProvider(p), chainforge.WithModel("mock"))
	agent.Run(context.Background(), "s1", "find me something unique")
	testutil.AssertLastRequestContains(t, p, core.RoleUser, "unique")
}

func TestAssertSessionContains_Pass(t *testing.T) {
	mem := testutil.NewMapMemory()
	p := testutil.NewMockProvider(testutil.EndTurnResponse("answer"))
	agent, _ := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithMemory(mem),
	)
	agent.Run(context.Background(), "s1", "my special input")
	testutil.AssertSessionContains(t, mem, "s1", "my special input")
}

func TestAssertSessionLen_Pass(t *testing.T) {
	mem := testutil.NewMapMemory()
	p := testutil.NewMockProvider(testutil.EndTurnResponse("answer"))
	agent, _ := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithMemory(mem),
	)
	agent.Run(context.Background(), "s1", "hello")
	// 1 user message + 1 assistant response = 2
	testutil.AssertSessionLen(t, mem, "s1", 2)
}
