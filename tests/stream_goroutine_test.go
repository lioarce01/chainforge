package tests

import (
	"context"
	"runtime"
	"testing"
	"time"

	chainforge "github.com/lioarce01/chainforge"
)

// TestRunStreamGoroutineLeakOnCancel verifies that cancelling ctx while the
// stream goroutine is running does not leave it blocked forever.
func TestRunStreamGoroutineLeakOnCancel(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("hello world"))
	a := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
	)

	baseline := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	ch := a.RunStream(ctx, "sess", "hello")

	// Read one event then cancel immediately.
	<-ch
	cancel()

	// Drain remaining so the goroutine can unblock.
	for range ch {
	}

	// Give the goroutine time to fully exit.
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	// Allow a small margin for unrelated Go runtime goroutines.
	if after > baseline+3 {
		t.Errorf("possible goroutine leak: baseline=%d after=%d", baseline, after)
	}
}

// TestRunStreamCallerStopsDraining verifies stopping consumption and cancelling
// ctx allows the goroutine to exit cleanly (no deadlock).
func TestRunStreamCallerStopsDraining(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("test response"))
	a := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
	)

	ctx, cancel := context.WithCancel(context.Background())
	ch := a.RunStream(ctx, "sess", "test")

	// Cancel immediately before draining.
	cancel()
	// Drain so the test doesn't block.
	for range ch {
	}
	// Reaching here without deadlock or panic is the pass condition.
}

// TestRunStreamCompletesNormally verifies that a full stream is received intact.
func TestRunStreamCompletesNormally(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("final answer"))
	a := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
	)

	var text string
	for ev := range a.RunStream(context.Background(), "sess", "question") {
		if ev.Error != nil {
			t.Fatalf("unexpected error: %v", ev.Error)
		}
		text += ev.TextDelta
	}
	if text != "final answer" {
		t.Errorf("got %q, want %q", text, "final answer")
	}
}
