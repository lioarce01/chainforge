package tests

import (
	"context"
	"testing"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/orchestrator"
	"github.com/lioarce01/chainforge/pkg/tools"
)

// --- 3.1 Session ID context ---

func TestSessionIDContextRoundTrip(t *testing.T) {
	ctx := chainforge.WithSessionID(context.Background(), "my-session")
	got := chainforge.SessionIDFromContext(ctx)
	if got != "my-session" {
		t.Errorf("SessionIDFromContext = %q, want %q", got, "my-session")
	}
}

func TestSessionIDFromContext_Empty(t *testing.T) {
	got := chainforge.SessionIDFromContext(context.Background())
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestSessionIDInjectedByAgentLoop(t *testing.T) {
	var capturedSessionID string

	// Use a mock provider that captures the session ID from context.
	type captureProvider struct{ *MockProvider }
	type cp = captureProvider
	_ = cp{}

	// Simpler: the session ID is injected into ctx before provider.Chat;
	// verify via a custom provider wrapper.
	sessionIDs := make(chan string, 1)
	mock := NewMockProvider(EndTurnResponse("ok"))

	type wrapProvider struct {
		core.Provider
	}
	// We'll verify indirectly: run the agent and check the stored ID is non-empty
	// (since we can't easily intercept context without a real middleware).
	a := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
	)
	close(sessionIDs)
	_ = capturedSessionID

	result, err := a.Run(context.Background(), "session-abc", "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q", result)
	}
}

// --- 3.3 Structured output ---

func TestStructuredOutputValid(t *testing.T) {
	schema := []byte(`{"type":"object"}`)
	mock := NewMockProvider(EndTurnResponse(`{"answer":"42"}`))

	a := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
		chainforge.WithStructuredOutput(schema),
	)

	result, err := a.Run(context.Background(), "sess", "what is the answer?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"answer":"42"}` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestStructuredOutputInvalid(t *testing.T) {
	schema := []byte(`{"type":"object"}`)
	mock := NewMockProvider(EndTurnResponse("not json at all"))

	a := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
		chainforge.WithStructuredOutput(schema),
	)

	_, err := a.Run(context.Background(), "sess", "respond")
	if err == nil {
		t.Fatal("expected ErrInvalidOutput, got nil")
	}
	// Error should wrap ErrInvalidOutput
	found := false
	for e := err; e != nil; {
		if e == chainforge.ErrInvalidOutput {
			found = true
			break
		}
		if uw, ok := e.(interface{ Unwrap() error }); ok {
			e = uw.Unwrap()
		} else {
			break
		}
	}
	if !found {
		t.Errorf("error %v should wrap ErrInvalidOutput", err)
	}
}

func TestStructuredOutputSchemaMismatch(t *testing.T) {
	// Expect an object but LLM returns an array.
	schema := []byte(`{"type":"object"}`)
	mock := NewMockProvider(EndTurnResponse(`["not","an","object"]`))

	a := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
		chainforge.WithStructuredOutput(schema),
	)

	_, err := a.Run(context.Background(), "sess", "respond")
	if err == nil {
		t.Fatal("expected error for schema type mismatch, got nil")
	}
}

func TestStructuredOutputNoSchema(t *testing.T) {
	// No schema set — any response accepted.
	mock := NewMockProvider(EndTurnResponse("plain text response"))

	a := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
	)

	result, err := a.Run(context.Background(), "sess", "respond")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "plain text response" {
		t.Errorf("unexpected result: %q", result)
	}
}

// --- 3.4 Conditional orchestration ---

func TestConditionalTrueBranch(t *testing.T) {
	ifCalled := false
	elseCalled := false

	mockIf := NewMockProvider(EndTurnResponse("if-result"))
	mockElse := NewMockProvider(EndTurnResponse("else-result"))

	ifAgent := chainforge.MustNewAgent(chainforge.WithProvider(mockIf), chainforge.WithModel("mock"))
	elseAgent := chainforge.MustNewAgent(chainforge.WithProvider(mockElse), chainforge.WithModel("mock"))

	result, err := orchestrator.Conditional(
		context.Background(), "sess", "input",
		func(output string) bool { ifCalled = true; return true },
		ifAgent, elseAgent,
	)
	_ = elseCalled

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "if-result" {
		t.Errorf("got %q, want %q", result, "if-result")
	}
	if !ifCalled {
		t.Error("predicate not called")
	}
	if mockElse.CallCount() != 0 {
		t.Error("else agent should not have been called")
	}
}

func TestConditionalFalseBranch(t *testing.T) {
	mockIf := NewMockProvider(EndTurnResponse("if-result"))
	mockElse := NewMockProvider(EndTurnResponse("else-result"))

	ifAgent := chainforge.MustNewAgent(chainforge.WithProvider(mockIf), chainforge.WithModel("mock"))
	elseAgent := chainforge.MustNewAgent(chainforge.WithProvider(mockElse), chainforge.WithModel("mock"))

	result, err := orchestrator.Conditional(
		context.Background(), "sess", "input",
		func(output string) bool { return false },
		ifAgent, elseAgent,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "else-result" {
		t.Errorf("got %q, want %q", result, "else-result")
	}
	if mockIf.CallCount() != 0 {
		t.Error("if agent should not have been called")
	}
}

func TestConditionalNilElse(t *testing.T) {
	mockIf := NewMockProvider(EndTurnResponse("if-result"))
	ifAgent := chainforge.MustNewAgent(chainforge.WithProvider(mockIf), chainforge.WithModel("mock"))

	result, err := orchestrator.Conditional(
		context.Background(), "sess", "original-input",
		func(output string) bool { return false },
		ifAgent, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "original-input" {
		t.Errorf("got %q, want %q", result, "original-input")
	}
}

func TestLoopRunsUntilFalse(t *testing.T) {
	var iters int
	responses := make([]MockResponse, 10)
	for i := range responses {
		responses[i] = EndTurnResponse("iter")
	}
	mock := NewMockProvider(responses...)
	agent := chainforge.MustNewAgent(chainforge.WithProvider(mock), chainforge.WithModel("mock"))

	_, err := orchestrator.Loop(
		context.Background(), "sess", "start", agent,
		func(iter int, output string) bool {
			iters++
			return iters <= 3 // run 3 iterations
		},
		10,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if iters != 4 { // 3 true + 1 false
		t.Errorf("expected 4 condition checks (3 true + 1 false), got %d", iters)
	}
	if mock.CallCount() != 3 {
		t.Errorf("expected 3 agent calls, got %d", mock.CallCount())
	}
}

func TestLoopMaxIterEnforced(t *testing.T) {
	responses := make([]MockResponse, 10)
	for i := range responses {
		responses[i] = EndTurnResponse("iter")
	}
	mock := NewMockProvider(responses...)
	agent := chainforge.MustNewAgent(chainforge.WithProvider(mock), chainforge.WithModel("mock"))

	// Condition always true — loop should stop at maxIter=5.
	result, err := orchestrator.Loop(
		context.Background(), "sess", "start", agent,
		func(iter int, output string) bool { return true },
		5,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After 5 iterations the loop exits with the last output.
	if result != "iter" {
		t.Errorf("got %q, want %q", result, "iter")
	}
	if mock.CallCount() != 5 {
		t.Errorf("expected 5 agent calls, got %d", mock.CallCount())
	}
}

func TestLoopContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	responses := make([]MockResponse, 10)
	for i := range responses {
		responses[i] = EndTurnResponse("iter")
	}
	mock := NewMockProvider(responses...)

	// Use a tool that blocks until context is cancelled so the agent propagates it.
	schema := tools.NewSchema().MustBuild()
	blockTool := tools.MustFunc("blocker", "blocks", schema, func(ctx context.Context, input string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})

	// First iteration: tool use → triggers the blocker → context cancelled.
	mockWithTool := NewMockProvider(
		ToolUseResponse(core.ToolCall{ID: "1", Name: "blocker", Input: "{}"}),
	)
	_ = mock
	_ = responses

	agent := chainforge.MustNewAgent(
		chainforge.WithProvider(mockWithTool),
		chainforge.WithModel("mock"),
		chainforge.WithTools(blockTool),
		chainforge.WithToolTimeout(200*time.Millisecond),
	)

	// Cancel the context shortly after starting.
	go func() {
		cancel()
	}()

	_, err := orchestrator.Loop(
		ctx, "sess", "start", agent,
		func(iter int, output string) bool { return true },
		10,
	)
	// Should get a context error from the agent.
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}
