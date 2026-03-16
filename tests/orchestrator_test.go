package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/orchestrator"
)

// Test 1: Sequential pipeline — output of step 1 fed to step 2
func TestSequentialPipeline(t *testing.T) {
	mock1 := NewMockProvider(EndTurnResponse("step1 result"))
	mock2 := NewMockProvider(EndTurnResponse("step2 result"))

	agent1 := chainforge.MustNewAgent(chainforge.WithProvider(mock1), chainforge.WithModel("test"))
	agent2 := chainforge.MustNewAgent(chainforge.WithProvider(mock2), chainforge.WithModel("test"))

	result, err := orchestrator.Sequential(context.Background(), "sess",
		"initial input",
		orchestrator.StepOf("research", agent1, "Research: {{.input}}"),
		orchestrator.StepOf("write", agent2, "Write based on: {{.previous}}"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "step2 result" {
		t.Errorf("got %q, want %q", result, "step2 result")
	}

	// Verify step2 received step1's output in its message
	calls2 := mock2.Calls()
	if len(calls2) == 0 {
		t.Fatal("agent2 not called")
	}
	msgs := calls2[0].Request.Messages
	found := false
	for _, m := range msgs {
		if strings.Contains(m.Content, "step1 result") {
			found = true
			break
		}
	}
	if !found {
		t.Error("step2 did not receive step1 output")
	}
}

// Test 2: Sequential — template rendering with {{.input}} and {{.previous}}
func TestSequentialTemplateRendering(t *testing.T) {
	captured := ""
	mock := NewMockProvider(
		MockResponse{
			Response: core.ChatResponse{
				Message:    core.Message{Role: core.RoleAssistant, Content: "first"},
				StopReason: core.StopReasonEndTurn,
			},
		},
		MockResponse{
			Response: core.ChatResponse{
				Message:    core.Message{Role: core.RoleAssistant, Content: "second"},
				StopReason: core.StopReasonEndTurn,
			},
		},
	)

	captureAgent := chainforge.MustNewAgent(chainforge.WithProvider(mock), chainforge.WithModel("test"))

	_, err := orchestrator.Sequential(context.Background(), "sess",
		"hello",
		orchestrator.StepOf("step1", captureAgent, "Input was: {{.input}}"),
		orchestrator.StepOf("step2", captureAgent, "Previous was: {{.previous}} and input was: {{.input}}"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = captured

	calls := mock.Calls()
	if len(calls) < 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	// First call should contain "Input was: hello"
	msg1 := calls[0].Request.Messages
	foundInput := false
	for _, m := range msg1 {
		if strings.Contains(m.Content, "Input was: hello") {
			foundInput = true
		}
	}
	if !foundInput {
		t.Error("first step template not rendered correctly")
	}
}

// Test 3: Sequential — step failure wraps step name
func TestSequentialStepFailure(t *testing.T) {
	mock := NewMockProvider(MockResponse{Err: fmt.Errorf("provider down")})
	agent := chainforge.MustNewAgent(chainforge.WithProvider(mock), chainforge.WithModel("test"))

	_, err := orchestrator.Sequential(context.Background(), "sess",
		"input",
		orchestrator.StepOf("failing-step", agent, "{{.input}}"),
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failing-step") {
		t.Errorf("error should mention step name, got: %v", err)
	}
}

// Test 4: Parallel — all branches run and return results
func TestParallelAllBranches(t *testing.T) {
	mockA := NewMockProvider(EndTurnResponse("result A"))
	mockB := NewMockProvider(EndTurnResponse("result B"))
	mockC := NewMockProvider(EndTurnResponse("result C"))

	agentA := chainforge.MustNewAgent(chainforge.WithProvider(mockA), chainforge.WithModel("test"))
	agentB := chainforge.MustNewAgent(chainforge.WithProvider(mockB), chainforge.WithModel("test"))
	agentC := chainforge.MustNewAgent(chainforge.WithProvider(mockC), chainforge.WithModel("test"))

	results, err := orchestrator.Parallel(context.Background(), "sess",
		orchestrator.FanOf("a", agentA, "Do A"),
		orchestrator.FanOf("b", agentB, "Do B"),
		orchestrator.FanOf("c", agentC, "Do C"),
	)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	byName := make(map[string]orchestrator.ParallelResult)
	for _, r := range results {
		byName[r.Name] = r
	}

	for name, expected := range map[string]string{"a": "result A", "b": "result B", "c": "result C"} {
		r, ok := byName[name]
		if !ok {
			t.Errorf("missing result for %q", name)
			continue
		}
		if r.Error != nil {
			t.Errorf("branch %q had error: %v", name, r.Error)
		}
		if r.Output != expected {
			t.Errorf("branch %q: got %q, want %q", name, r.Output, expected)
		}
	}
}

// Test 5: Parallel — partial failure does not cancel siblings; all results returned
func TestParallelPartialFailure(t *testing.T) {
	mockOK := NewMockProvider(EndTurnResponse("success"))
	mockFail := NewMockProvider(MockResponse{Err: fmt.Errorf("branch failed")})

	agentOK := chainforge.MustNewAgent(chainforge.WithProvider(mockOK), chainforge.WithModel("test"))
	agentFail := chainforge.MustNewAgent(chainforge.WithProvider(mockFail), chainforge.WithModel("test"))

	results, err := orchestrator.Parallel(context.Background(), "sess",
		orchestrator.FanOf("ok", agentOK, "Run"),
		orchestrator.FanOf("fail", agentFail, "Run"),
	)
	if err != nil {
		t.Fatalf("top-level error should be nil for partial failure, got: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	byName := make(map[string]orchestrator.ParallelResult)
	for _, r := range results {
		byName[r.Name] = r
	}

	if byName["ok"].Error != nil {
		t.Errorf("ok branch should have no error, got: %v", byName["ok"].Error)
	}
	if byName["ok"].Output != "success" {
		t.Errorf("ok branch output: %q", byName["ok"].Output)
	}
	if byName["fail"].Error == nil {
		t.Error("fail branch should have error")
	}
}
