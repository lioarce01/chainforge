package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/tools"
)

func TestRunWithUsage_SingleIteration(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("hello"))
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, usage, err := agent.RunWithUsage(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatal(err)
	}
	if usage.InputTokens != 10 || usage.OutputTokens != 5 {
		t.Errorf("expected Usage{10,5}, got %+v", usage)
	}
}

func TestRunWithUsage_AccumulatesAcrossIterations(t *testing.T) {
	// Tool-use path: LLM → tool call → LLM → end_turn (2 LLM calls)
	mock := NewMockProvider(
		ToolUseResponse(core.ToolCall{ID: "c1", Name: "noop", Input: "{}"}),
		EndTurnResponse("done"),
	)
	noopTool := tools.MustFunc("noop", "does nothing", nil, func(ctx context.Context, input string) (string, error) {
		return "ok", nil
	})
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
		chainforge.WithTools(noopTool),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, usage, err := agent.RunWithUsage(context.Background(), "s2", "hi")
	if err != nil {
		t.Fatal(err)
	}
	// ToolUseResponse: {20,10}, EndTurnResponse: {10,5} → sum {30,15}
	if usage.InputTokens != 30 || usage.OutputTokens != 15 {
		t.Errorf("expected Usage{30,15}, got %+v", usage)
	}
}

func TestRunWithUsage_ZeroOnError(t *testing.T) {
	mock := NewMockProvider(MockResponse{Err: fmt.Errorf("provider down")})
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, usage, err := agent.RunWithUsage(context.Background(), "s3", "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if usage != (core.Usage{}) {
		t.Errorf("expected zero usage on error, got %+v", usage)
	}
}

func TestRunStream_DoneEventCarriesUsage(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("streamed"))
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
	)
	if err != nil {
		t.Fatal(err)
	}

	ch := agent.RunStream(context.Background(), "s4", "hi")
	var doneEvent *core.StreamEvent
	for ev := range ch {
		if ev.Type == core.StreamEventDone {
			e := ev
			doneEvent = &e
		}
	}
	if doneEvent == nil {
		t.Fatal("no Done event received")
	}
	if doneEvent.Usage == nil {
		t.Fatal("Done event has nil Usage")
	}
	if doneEvent.Usage.InputTokens != 10 || doneEvent.Usage.OutputTokens != 5 {
		t.Errorf("expected Usage{10,5}, got %+v", doneEvent.Usage)
	}
}

func TestWithRunTimeout_ExpiresReturnsDeadline(t *testing.T) {
	// Provider that blocks until context is cancelled
	slow := &slowProvider{}
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(slow),
		chainforge.WithModel("test-model"),
		chainforge.WithRunTimeout(1*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = agent.Run(context.Background(), "s5", "hi")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestWithRunTimeout_ZeroMeansNoTimeout(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("ok"))
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("test-model"),
		chainforge.WithRunTimeout(0),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := agent.Run(context.Background(), "s6", "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

// slowProvider blocks on Chat until context is cancelled.
type slowProvider struct{}

func (s *slowProvider) Name() string { return "slow" }
func (s *slowProvider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	<-ctx.Done()
	return core.ChatResponse{}, ctx.Err()
}
func (s *slowProvider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
