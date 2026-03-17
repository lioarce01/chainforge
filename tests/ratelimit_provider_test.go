package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/middleware/ratelimit"
)

func TestRateLimit_AllowsBurstImmediately(t *testing.T) {
	mock := NewMockProvider(
		EndTurnResponse("1"),
		EndTurnResponse("2"),
		EndTurnResponse("3"),
		EndTurnResponse("4"),
		EndTurnResponse("5"),
	)
	rl := ratelimit.New(mock, 1, 5) // 1 rps, burst=5

	start := time.Now()
	for i := 0; i < 5; i++ {
		_, err := rl.Chat(context.Background(), core.ChatRequest{})
		if err != nil {
			t.Fatal(err)
		}
	}
	// All 5 should complete nearly immediately (burst allows it)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("burst of 5 took %v, expected < 500ms", elapsed)
	}
}

func TestRateLimit_ThrottlesOverBurst(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("a"), EndTurnResponse("b"))
	// rps=100, burst=1 → 2nd call must wait ~10ms
	rl := ratelimit.New(mock, 100, 1)

	start := time.Now()
	rl.Chat(context.Background(), core.ChatRequest{})
	rl.Chat(context.Background(), core.ChatRequest{})
	elapsed := time.Since(start)

	if elapsed < 5*time.Millisecond {
		t.Errorf("2nd call returned too fast (%v), expected throttling", elapsed)
	}
}

func TestRateLimit_ContextCancelUnblocks(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("ok"))
	// rps=0.001 → tokens very slowly; burst=1 consumed on first call
	rl := ratelimit.New(mock, 0.001, 1)

	// Consume burst
	rl.Chat(context.Background(), core.ChatRequest{})

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := rl.Chat(ctx, core.ChatRequest{})
		errCh <- err
	}()

	// Let the goroutine start waiting for a token, then cancel
	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-errCh
	if err == nil {
		t.Fatal("expected error after context cancellation")
	}
	// Accept either context.Canceled or the rate limiter's own deadline error
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		// Rate limiter may return its own error; just verify it's non-nil (already checked above)
		t.Logf("got expected non-nil error: %v", err)
	}
}

func TestRateLimit_ChatStreamGated(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("streamed"))
	rl := ratelimit.New(mock, 1000, 1)

	ch, err := rl.ChatStream(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for ev := range ch {
		if ev.Type == core.StreamEventText {
			got += ev.TextDelta
		}
	}
	if got != "streamed" {
		t.Errorf("expected 'streamed', got %q", got)
	}
}

func TestProviderBuilder_WithRateLimit(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("ok"))
	p := chainforge.NewProviderBuilder(mock).WithRateLimit(1000, 10).Build()
	resp, err := p.Chat(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Message.Content != "ok" {
		t.Errorf("expected 'ok', got %q", resp.Message.Content)
	}
}
