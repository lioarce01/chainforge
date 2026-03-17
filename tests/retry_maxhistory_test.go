package tests

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
)

// --- WithRetry tests ---

// countingProvider counts calls and fails the first n attempts.
type countingProvider struct {
	calls     atomic.Int32
	failFirst int
	response  string
}

func (p *countingProvider) Name() string { return "counting" }

func (p *countingProvider) Chat(_ context.Context, _ core.ChatRequest) (core.ChatResponse, error) {
	n := int(p.calls.Add(1))
	if n <= p.failFirst {
		return core.ChatResponse{}, fmt.Errorf("transient error attempt %d", n)
	}
	return core.ChatResponse{
		Message:    core.Message{Role: core.RoleAssistant, Content: p.response},
		StopReason: core.StopReasonEndTurn,
		Usage:      core.Usage{InputTokens: 5, OutputTokens: 3},
	}, nil
}

func (p *countingProvider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan core.StreamEvent, 2)
	go func() {
		defer close(ch)
		ch <- core.StreamEvent{Type: core.StreamEventText, TextDelta: resp.Message.Content}
		ch <- core.StreamEvent{Type: core.StreamEventDone, StopReason: core.StopReasonEndTurn}
	}()
	return ch, nil
}

func TestWithRetry_SucceedsAfterTransientError(t *testing.T) {
	p := &countingProvider{failFirst: 2, response: "ok"}

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithRetry(3),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "ok" {
		t.Fatalf("want ok, got %q", result)
	}
	if p.calls.Load() != 3 {
		t.Fatalf("want 3 calls (2 failures + 1 success), got %d", p.calls.Load())
	}
}

func TestWithRetry_ExhaustsAttemptsReturnsError(t *testing.T) {
	p := &countingProvider{failFirst: 5, response: "ok"} // always fails within 3 attempts

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithRetry(3),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), "s1", "hi")
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if p.calls.Load() != 3 {
		t.Fatalf("want 3 attempts, got %d", p.calls.Load())
	}
}

func TestWithRetry_NoRetryOnContextCancel(t *testing.T) {
	p := &countingProvider{failFirst: 5, response: "ok"}

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithRetry(3),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err = agent.Run(ctx, "s1", "hi")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	p := &countingProvider{failFirst: 0, response: "instant"}

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithRetry(3),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "instant" {
		t.Fatalf("want instant, got %q", result)
	}
	if p.calls.Load() != 1 {
		t.Fatalf("want 1 call, got %d", p.calls.Load())
	}
}

// --- WithMaxHistory tests ---

func TestWithMaxHistory_LimitsMessages(t *testing.T) {
	mem := inmemory.New()
	ctx := context.Background()
	sessionID := "sess"

	// Pre-populate 10 messages in memory.
	for i := 0; i < 10; i++ {
		role := core.RoleUser
		if i%2 == 1 {
			role = core.RoleAssistant
		}
		_ = mem.Append(ctx, sessionID, core.Message{
			Role:    role,
			Content: fmt.Sprintf("message %d", i),
		})
	}

	// Track how many messages the provider receives.
	var receivedMsgs int
	spy := &spyProvider{onChat: func(req core.ChatRequest) {
		receivedMsgs = len(req.Messages)
	}, response: "ok"}

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(spy),
		chainforge.WithModel("mock"),
		chainforge.WithMemory(mem),
		chainforge.WithMaxHistory(4), // only last 4 messages
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(ctx, sessionID, "new message")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Provider should receive: 4 history messages + 1 new user message = 5
	if receivedMsgs != 5 {
		t.Fatalf("want 5 messages (4 history + 1 new), got %d", receivedMsgs)
	}
}

func TestWithMaxHistory_UnlimitedByDefault(t *testing.T) {
	mem := inmemory.New()
	ctx := context.Background()
	sessionID := "sess"

	for i := 0; i < 8; i++ {
		role := core.RoleUser
		if i%2 == 1 {
			role = core.RoleAssistant
		}
		_ = mem.Append(ctx, sessionID, core.Message{Role: role, Content: fmt.Sprintf("msg %d", i)})
	}

	var receivedMsgs int
	spy := &spyProvider{onChat: func(req core.ChatRequest) {
		receivedMsgs = len(req.Messages)
	}, response: "ok"}

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(spy),
		chainforge.WithModel("mock"),
		chainforge.WithMemory(mem),
		// no WithMaxHistory — should load all 8
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(ctx, sessionID, "new")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// 8 history + 1 new = 9
	if receivedMsgs != 9 {
		t.Fatalf("want 9 messages, got %d", receivedMsgs)
	}
}

func TestWithMaxHistory_ShorterThanHistory(t *testing.T) {
	mem := inmemory.New()
	ctx := context.Background()

	// Only 2 messages — maxHistory=10 should not truncate.
	_ = mem.Append(ctx, "s", core.Message{Role: core.RoleUser, Content: "a"})
	_ = mem.Append(ctx, "s", core.Message{Role: core.RoleAssistant, Content: "b"})

	var receivedMsgs int
	spy := &spyProvider{onChat: func(req core.ChatRequest) {
		receivedMsgs = len(req.Messages)
	}, response: "ok"}

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(spy),
		chainforge.WithModel("mock"),
		chainforge.WithMemory(mem),
		chainforge.WithMaxHistory(10),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(ctx, "s", "new")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// 2 history + 1 new = 3 (maxHistory doesn't truncate when below limit)
	if receivedMsgs != 3 {
		t.Fatalf("want 3 messages, got %d", receivedMsgs)
	}
}

// spyProvider captures the ChatRequest without failing.
type spyProvider struct {
	onChat   func(req core.ChatRequest)
	response string
}

func (s *spyProvider) Name() string { return "spy" }

func (s *spyProvider) Chat(_ context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	if s.onChat != nil {
		s.onChat(req)
	}
	return core.ChatResponse{
		Message:    core.Message{Role: core.RoleAssistant, Content: s.response},
		StopReason: core.StopReasonEndTurn,
		Usage:      core.Usage{InputTokens: 5, OutputTokens: 3},
	}, nil
}

func (s *spyProvider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	resp, err := s.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan core.StreamEvent, 2)
	go func() {
		defer close(ch)
		ch <- core.StreamEvent{Type: core.StreamEventText, TextDelta: resp.Message.Content}
		ch <- core.StreamEvent{Type: core.StreamEventDone, StopReason: core.StopReasonEndTurn}
	}()
	return ch, nil
}
