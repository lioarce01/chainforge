package tests

import (
	"context"
	"errors"
	"strings"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
)

func endTurnResponse(text string) MockResponse {
	return MockResponse{
		Response: core.ChatResponse{
			Message:    core.Message{Role: core.RoleAssistant, Content: text},
			StopReason: "end_turn",
		},
	}
}

// TestHistorySummarizer_FiresWhenOverLimit verifies that when history exceeds
// maxHistory the summarizer agent is called and the result is persisted.
func TestHistorySummarizer_FiresWhenOverLimit(t *testing.T) {
	mem := inmemory.New()
	ctx := context.Background()
	sessionID := "test-session"

	// Pre-populate 5 messages.
	for i := 0; i < 5; i++ {
		_ = mem.Append(ctx, sessionID, core.Message{Role: core.RoleUser, Content: "msg"})
	}

	summarizerProvider := NewMockProvider(endTurnResponse("concise summary"))
	summarizer, err := chainforge.NewAgent(
		chainforge.WithProvider(summarizerProvider),
		chainforge.WithModel("mock"),
	)
	if err != nil {
		t.Fatalf("NewAgent summarizer: %v", err)
	}

	mainProvider := NewMockProvider(endTurnResponse("final answer"))
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(mainProvider),
		chainforge.WithModel("mock"),
		chainforge.WithMemory(mem),
		chainforge.WithMaxHistory(3),
		chainforge.WithHistorySummarizer(summarizer),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(ctx, sessionID, "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "final answer" {
		t.Errorf("result = %q, want %q", result, "final answer")
	}

	// Verify the summarizer was called.
	if summarizerProvider.CallCount() == 0 {
		t.Error("expected summarizer provider to be called at least once")
	}

	// Verify the summary message was persisted (it should appear in history).
	persisted, err := mem.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("mem.Get: %v", err)
	}
	found := false
	for _, m := range persisted {
		if strings.HasPrefix(m.Content, "[Summary:") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a [Summary:...] message in persisted history, got: %v", persisted)
	}
}

// TestHistorySummarizer_SkipsWhenUnderLimit verifies the summarizer is not
// called when history is within the limit.
func TestHistorySummarizer_SkipsWhenUnderLimit(t *testing.T) {
	mem := inmemory.New()
	ctx := context.Background()
	sessionID := "under-limit"

	// Only 2 messages — below maxHistory=5.
	_ = mem.Append(ctx, sessionID, core.Message{Role: core.RoleUser, Content: "hi"})
	_ = mem.Append(ctx, sessionID, core.Message{Role: core.RoleAssistant, Content: "hello"})

	summarizerProvider := NewMockProvider(endTurnResponse("should not be called"))
	summarizer, _ := chainforge.NewAgent(
		chainforge.WithProvider(summarizerProvider),
		chainforge.WithModel("mock"),
	)

	mainProvider := NewMockProvider(endTurnResponse("ok"))
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(mainProvider),
		chainforge.WithModel("mock"),
		chainforge.WithMemory(mem),
		chainforge.WithMaxHistory(5),
		chainforge.WithHistorySummarizer(summarizer),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(ctx, sessionID, "ping")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if summarizerProvider.CallCount() > 0 {
		t.Error("summarizer should not have been called when history is within limit")
	}
}

// TestHistorySummarizer_NoMaxHistory verifies that without WithMaxHistory the
// summarizer is never triggered.
func TestHistorySummarizer_NoMaxHistory(t *testing.T) {
	mem := inmemory.New()
	ctx := context.Background()
	sessionID := "no-maxhistory"

	for i := 0; i < 10; i++ {
		_ = mem.Append(ctx, sessionID, core.Message{Role: core.RoleUser, Content: "x"})
	}

	summarizerProvider := NewMockProvider(endTurnResponse("never"))
	summarizer, _ := chainforge.NewAgent(
		chainforge.WithProvider(summarizerProvider),
		chainforge.WithModel("mock"),
	)

	mainProvider := NewMockProvider(endTurnResponse("done"))
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(mainProvider),
		chainforge.WithModel("mock"),
		chainforge.WithMemory(mem),
		// No WithMaxHistory — summarizer should be a no-op.
		chainforge.WithHistorySummarizer(summarizer),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(ctx, sessionID, "test")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if summarizerProvider.CallCount() > 0 {
		t.Error("summarizer must not fire when maxHistory is 0 (unlimited)")
	}
}

// TestHistorySummarizer_SummaryMessagePrefix verifies the summary message
// is prepended with the expected "[Summary: ...]" format.
func TestHistorySummarizer_SummaryMessagePrefix(t *testing.T) {
	mem := inmemory.New()
	ctx := context.Background()
	sessionID := "prefix-test"

	for i := 0; i < 4; i++ {
		_ = mem.Append(ctx, sessionID, core.Message{Role: core.RoleUser, Content: "old message"})
	}

	summarizerProvider := NewMockProvider(endTurnResponse("KEY_FACTS"))
	summarizer, _ := chainforge.NewAgent(
		chainforge.WithProvider(summarizerProvider),
		chainforge.WithModel("mock"),
	)

	mainProvider := NewMockProvider(endTurnResponse("done"))
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(mainProvider),
		chainforge.WithModel("mock"),
		chainforge.WithMemory(mem),
		chainforge.WithMaxHistory(2),
		chainforge.WithHistorySummarizer(summarizer),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(ctx, sessionID, "new")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	persisted, _ := mem.Get(ctx, sessionID)
	if len(persisted) == 0 {
		t.Fatal("expected persisted messages")
	}

	// First persisted message must be the summary.
	first := persisted[0].Content
	if !strings.HasPrefix(first, "[Summary:") {
		t.Errorf("first persisted message = %q, want prefix \"[Summary:\"", first)
	}
	if !strings.Contains(first, "KEY_FACTS") {
		t.Errorf("summary message %q does not contain expected summary text", first)
	}
}

// TestHistorySummarizer_ErrorPropagates verifies that a summarizer error is
// returned from Run instead of silently dropped.
func TestHistorySummarizer_ErrorPropagates(t *testing.T) {
	mem := inmemory.New()
	ctx := context.Background()
	sessionID := "err-test"

	for i := 0; i < 5; i++ {
		_ = mem.Append(ctx, sessionID, core.Message{Role: core.RoleUser, Content: "msg"})
	}

	// Summarizer returns an error.
	summarizerProvider := NewMockProvider(MockResponse{Err: errors.New("summarizer failed")})
	summarizer, _ := chainforge.NewAgent(
		chainforge.WithProvider(summarizerProvider),
		chainforge.WithModel("mock"),
	)

	mainProvider := NewMockProvider(endTurnResponse("should not reach"))
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(mainProvider),
		chainforge.WithModel("mock"),
		chainforge.WithMemory(mem),
		chainforge.WithMaxHistory(3),
		chainforge.WithHistorySummarizer(summarizer),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(ctx, sessionID, "hello")
	if err == nil {
		t.Error("expected error from failing summarizer, got nil")
	}
}
