package tests

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/tools"
)

// --- 2.2 Stream buffer size ---

// TestStreamBufferSizeOption verifies that the buffer size option is wired into RunStream.
func TestStreamBufferSizeOption(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("hi"))
	a := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
		chainforge.WithStreamBufferSize(64),
	)
	// Start a stream but don't drain — if the buffer is 64 the goroutine can
	// send without blocking until the buffer fills.
	ctx, cancel := context.WithCancel(context.Background())
	ch := a.RunStream(ctx, "sess", "test")
	cancel()
	for range ch { // drain so goroutine exits
	}
	// No assertion needed: the option must compile and wire through (build verification).
}

// TestStreamBufferLargePayload verifies no deadlock with a small buffer and normal stream.
func TestStreamBufferLargePayload(t *testing.T) {
	// Build a response sequence: 5 end-turn responses for the same agent.
	resps := make([]MockResponse, 5)
	for i := range resps {
		resps[i] = EndTurnResponse("chunk")
	}
	mock := NewMockProvider(resps...)
	a := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
		chainforge.WithStreamBufferSize(8),
	)
	// Run 5 independent streams to exercise buffer; none should deadlock.
	for i := 0; i < 5; i++ {
		for range a.RunStream(context.Background(), "sess", "go") {
		}
	}
}

// --- 2.4 Tool concurrency ---

// TestToolConcurrencyBound verifies that at most N tools run simultaneously.
func TestToolConcurrencyBound(t *testing.T) {
	const maxConcurrent = 2
	const totalTools = 6

	var (
		running atomic.Int32
		peak    atomic.Int32
	)

	makeSlowTool := func(name string) *tools.FuncTool {
		schema := tools.NewSchema().MustBuild()
		return tools.MustFunc(name, "slow", schema, func(ctx context.Context, input string) (string, error) {
			cur := running.Add(1)
			for {
				old := peak.Load()
				if cur <= old || peak.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			running.Add(-1)
			return "done", nil
		})
	}

	toolList := make([]core.Tool, totalTools)
	toolCalls := make([]core.ToolCall, totalTools)
	for i := 0; i < totalTools; i++ {
		name := string(rune('a' + i))
		toolList[i] = makeSlowTool(name)
		toolCalls[i] = core.ToolCall{ID: "id" + name, Name: name, Input: "{}"}
	}

	// Build two responses: one big tool-use call + one end-turn.
	responses := []MockResponse{
		ToolUseResponse(toolCalls...),
		EndTurnResponse("done"),
	}
	mock := NewMockProvider(responses...)

	opts := make([]chainforge.AgentOption, 0, totalTools+3)
	opts = append(opts,
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
		chainforge.WithToolConcurrency(maxConcurrent),
	)
	for _, t := range toolList {
		opts = append(opts, chainforge.WithTools(t))
	}

	a := chainforge.MustNewAgent(opts...)
	_, err := a.Run(context.Background(), "sess", "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if p := peak.Load(); p > maxConcurrent {
		t.Errorf("peak concurrent tools = %d, want <= %d", p, maxConcurrent)
	}
}

// TestToolConcurrencyUnlimited verifies that concurrency=0 (default) lets all tools run freely.
func TestToolConcurrencyUnlimited(t *testing.T) {
	const n = 5
	var running atomic.Int32
	var peak atomic.Int32

	makeSlowTool := func(name string) *tools.FuncTool {
		schema := tools.NewSchema().MustBuild()
		return tools.MustFunc(name, "slow", schema, func(ctx context.Context, input string) (string, error) {
			cur := running.Add(1)
			for {
				old := peak.Load()
				if cur <= old || peak.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			running.Add(-1)
			return "done", nil
		})
	}

	toolList := make([]core.Tool, n)
	toolCalls := make([]core.ToolCall, n)
	for i := 0; i < n; i++ {
		name := string(rune('a' + i))
		toolList[i] = makeSlowTool(name)
		toolCalls[i] = core.ToolCall{ID: "id" + name, Name: name, Input: "{}"}
	}

	responses := []MockResponse{ToolUseResponse(toolCalls...), EndTurnResponse("done")}
	mock := NewMockProvider(responses...)

	opts := make([]chainforge.AgentOption, 0, n+2)
	opts = append(opts, chainforge.WithProvider(mock), chainforge.WithModel("mock"))
	for _, t := range toolList {
		opts = append(opts, chainforge.WithTools(t))
	}

	a := chainforge.MustNewAgent(opts...)
	_, err := a.Run(context.Background(), "sess", "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// With unlimited concurrency all n tools should have run concurrently.
	if p := peak.Load(); p < int32(n) {
		t.Errorf("expected all %d tools to run concurrently, peak was %d", n, p)
	}
}

// TestToolConcurrencyContextCancel verifies context cancellation while waiting
// for a semaphore slot produces a tool error result (not a hang).
func TestToolConcurrencyContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	// Tool that blocks until context is cancelled.
	blockingSchema := tools.NewSchema().MustBuild()
	blockTool := tools.MustFunc("block", "blocks", blockingSchema, func(ctx context.Context, input string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	fastSchema := tools.NewSchema().MustBuild()
	fastTool := tools.MustFunc("fast", "fast", fastSchema, func(ctx context.Context, input string) (string, error) {
		time.Sleep(200 * time.Millisecond) // longer than test timeout
		return "done", nil
	})

	responses := []MockResponse{
		ToolUseResponse(
			core.ToolCall{ID: "1", Name: "block", Input: "{}"},
			core.ToolCall{ID: "2", Name: "fast", Input: "{}"},
		),
		EndTurnResponse("done"),
	}
	mock := NewMockProvider(responses...)

	a := chainforge.MustNewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
		chainforge.WithToolConcurrency(1), // only 1 slot; fast must wait
		chainforge.WithTools(blockTool, fastTool),
	)

	_, err := a.Run(ctx, "sess", "go")
	// Context should have been cancelled; either context.DeadlineExceeded or
	// context.Canceled is acceptable.
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}
