package tests

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
	"github.com/lioarce01/chainforge/pkg/tools"
)

// Test 1: Happy path — end_turn on first call; verify returned text + memory saved
func TestHappyPath(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("Hello, world!"))
	mem := inmemory.New()

	agent := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
		chainforge.WithMemory(mem),
	)

	result, err := agent.Run(context.Background(), "sess1", "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello, world!" {
		t.Errorf("got %q, want %q", result, "Hello, world!")
	}
	if mock.CallCount() != 1 {
		t.Errorf("expected 1 provider call, got %d", mock.CallCount())
	}

	// Verify memory saved
	ctx := context.Background()
	msgs, _ := mem.Get(ctx, "sess1")
	if len(msgs) < 2 {
		t.Errorf("expected at least 2 messages in memory (user+assistant), got %d", len(msgs))
	}
}

// Test 2: Single tool call → end_turn; verify tool invoked once + history has tool result
func TestSingleToolCall(t *testing.T) {
	const toolName = "echo"
	var called int
	schema := tools.NewSchema().Add("text", tools.Property{
		Type:        tools.TypeString,
		Description: "text to echo",
	}, true).MustBuild()

	echoTool := tools.MustFunc(toolName, "Echo text", schema, func(ctx context.Context, input string) (string, error) {
		called++
		return "echoed: " + input, nil
	})

	mock := NewMockProvider(
		ToolUseResponse(core.ToolCall{ID: "tc1", Name: toolName, Input: `{"text":"hello"}`}),
		EndTurnResponse("Done!"),
	)

	agent := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
		chainforge.WithTools(echoTool),
	)

	result, err := agent.Run(context.Background(), "sess2", "Echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Done!" {
		t.Errorf("got %q, want %q", result, "Done!")
	}
	if called != 1 {
		t.Errorf("tool called %d times, want 1", called)
	}
	if mock.CallCount() != 2 {
		t.Errorf("expected 2 provider calls (tool use + end), got %d", mock.CallCount())
	}

	// Verify second call includes tool result message
	calls := mock.Calls()
	msgs := calls[1].Request.Messages
	hasTool := false
	for _, m := range msgs {
		if m.Role == core.RoleTool {
			hasTool = true
			break
		}
	}
	if !hasTool {
		t.Error("second provider call missing tool result message")
	}
}

// Test 3: Multiple concurrent tool calls; verify all run concurrently (timestamp overlap)
func TestConcurrentToolCalls(t *testing.T) {
	var (
		mu         sync.Mutex
		startTimes []time.Time
	)

	makeDelayTool := func(name string) *tools.FuncTool {
		schema := tools.NewSchema().MustBuild()
		return tools.MustFunc(name, "Slow tool", schema, func(ctx context.Context, input string) (string, error) {
			mu.Lock()
			startTimes = append(startTimes, time.Now())
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			return "done", nil
		})
	}

	t1 := makeDelayTool("tool_a")
	t2 := makeDelayTool("tool_b")
	t3 := makeDelayTool("tool_c")

	mock := NewMockProvider(
		ToolUseResponse(
			core.ToolCall{ID: "tc1", Name: "tool_a", Input: "{}"},
			core.ToolCall{ID: "tc2", Name: "tool_b", Input: "{}"},
			core.ToolCall{ID: "tc3", Name: "tool_c", Input: "{}"},
		),
		EndTurnResponse("All done"),
	)

	agent := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
		chainforge.WithTools(t1, t2, t3),
	)

	start := time.Now()
	_, err := agent.Run(context.Background(), "sess3", "Run all tools")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(startTimes) != 3 {
		t.Fatalf("expected 3 tool calls, got %d", len(startTimes))
	}
	// If parallel: total < 2x single tool time; if sequential: total >= 150ms
	// Concurrent means elapsed should be well under 150ms (3 * 50ms sequential)
	if elapsed > 120*time.Millisecond {
		t.Errorf("tools appear to have run sequentially (elapsed %v, expected <120ms)", elapsed)
	}
}

// Test 4: Max iterations exceeded; verify errors.Is(err, core.ErrMaxIterations)
func TestMaxIterations(t *testing.T) {
	// Always returns tool use → infinite loop
	toolResp := ToolUseResponse(core.ToolCall{ID: "tc1", Name: "loop", Input: "{}"})

	schema := tools.NewSchema().MustBuild()
	loopTool := tools.MustFunc("loop", "loops forever", schema, func(ctx context.Context, input string) (string, error) {
		return "still going", nil
	})

	responses := make([]MockResponse, 5)
	for i := range responses {
		responses[i] = toolResp
	}
	mock := NewMockProvider(responses...)

	agent := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
		chainforge.WithTools(loopTool),
		chainforge.WithMaxIterations(3),
	)

	_, err := agent.Run(context.Background(), "sess4", "Loop")
	if !errors.Is(err, core.ErrMaxIterations) {
		t.Errorf("expected ErrMaxIterations, got %v", err)
	}
}

// Test 5: Unknown tool name; verify error fed as tool result, not hard failure
func TestUnknownTool(t *testing.T) {
	mock := NewMockProvider(
		ToolUseResponse(core.ToolCall{ID: "tc1", Name: "nonexistent", Input: "{}"}),
		EndTurnResponse("I see the tool failed"),
	)

	agent := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
		// No tools registered
	)

	result, err := agent.Run(context.Background(), "sess5", "Call unknown tool")
	if err != nil {
		t.Fatalf("expected no error (tool error fed back to LLM), got: %v", err)
	}
	if result != "I see the tool failed" {
		t.Errorf("unexpected result: %q", result)
	}

	// The second provider call should include a tool result with error text
	calls := mock.Calls()
	if len(calls) < 2 {
		t.Fatal("expected 2 provider calls")
	}
	msgs := calls[1].Request.Messages
	for _, m := range msgs {
		if m.Role == core.RoleTool && strings.Contains(m.Content, "error") {
			return // found
		}
	}
	t.Error("tool error not fed back to provider as tool result")
}

// Test 6: Context cancellation mid-run; verify ctx.Err() returned
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	schema := tools.NewSchema().MustBuild()
	cancelTool := tools.MustFunc("cancel_me", "cancels ctx", schema, func(ctx context.Context, input string) (string, error) {
		cancel() // cancel context during tool execution
		time.Sleep(10 * time.Millisecond)
		return "done", nil
	})

	mock := NewMockProvider(
		ToolUseResponse(core.ToolCall{ID: "tc1", Name: "cancel_me", Input: "{}"}),
		EndTurnResponse("too late"),
	)

	agent := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
		chainforge.WithTools(cancelTool),
	)

	_, err := agent.Run(ctx, "sess6", "Cancel me")
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// Test 7: Tool timeout; verify error result returned to LLM (not panic)
func TestToolTimeout(t *testing.T) {
	schema := tools.NewSchema().MustBuild()
	slowTool := tools.MustFunc("slow", "slow tool", schema, func(ctx context.Context, input string) (string, error) {
		select {
		case <-time.After(500 * time.Millisecond):
			return "finally", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})

	mock := NewMockProvider(
		ToolUseResponse(core.ToolCall{ID: "tc1", Name: "slow", Input: "{}"}),
		EndTurnResponse("tool timed out, ok"),
	)

	agent := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
		chainforge.WithTools(slowTool),
		chainforge.WithToolTimeout(10*time.Millisecond), // very short timeout
	)

	result, err := agent.Run(context.Background(), "sess7", "Be slow")
	if err != nil {
		t.Fatalf("unexpected hard error (tool timeout should be non-fatal): %v", err)
	}
	if result != "tool timed out, ok" {
		t.Errorf("unexpected result: %q", result)
	}
}

// Test 8: Memory persistence across two Run calls on same session
func TestMemoryPersistence(t *testing.T) {
	mem := inmemory.New()
	mock := NewMockProvider(
		EndTurnResponse("First response"),
		EndTurnResponse("Second response"),
	)

	agent := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
		chainforge.WithMemory(mem),
	)

	ctx := context.Background()

	_, err := agent.Run(ctx, "sess8", "First message")
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	_, err = agent.Run(ctx, "sess8", "Second message")
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	// Second call should include history from first call
	calls := mock.Calls()
	if len(calls) < 2 {
		t.Fatal("expected 2 provider calls")
	}
	secondReqMsgs := calls[1].Request.Messages
	// Should have: system (if any), first user, first assistant, second user
	if len(secondReqMsgs) < 2 {
		t.Errorf("expected history in second call, got %d messages", len(secondReqMsgs))
	}
}

// Test 9: System prompt always appears first in every ChatRequest.Messages
func TestSystemPromptFirst(t *testing.T) {
	mem := inmemory.New()
	schema := tools.NewSchema().MustBuild()
	echoTool := tools.MustFunc("echo", "echo", schema, func(ctx context.Context, input string) (string, error) {
		return "result", nil
	})

	mock := NewMockProvider(
		ToolUseResponse(core.ToolCall{ID: "tc1", Name: "echo", Input: "{}"}),
		EndTurnResponse("Done"),
	)

	agent := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
		chainforge.WithSystemPrompt("You are a helpful assistant."),
		chainforge.WithMemory(mem),
		chainforge.WithTools(echoTool),
	)

	_, err := agent.Run(context.Background(), "sess9", "Go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, call := range mock.Calls() {
		msgs := call.Request.Messages
		if len(msgs) == 0 {
			t.Errorf("call %d: no messages", i)
			continue
		}
		if msgs[0].Role != core.RoleSystem {
			t.Errorf("call %d: first message is %q, want system", i, msgs[0].Role)
		}
	}
}
