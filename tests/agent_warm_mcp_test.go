package tests

import (
	"context"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
)

func TestWarmMCP_NoServers_ReturnsNil(t *testing.T) {
	a, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("hi"))),
		chainforge.WithModel("mock"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if err := a.WarmMCP(context.Background()); err != nil {
		t.Errorf("WarmMCP with no servers: %v", err)
	}
}

func TestWarmMCP_Idempotent(t *testing.T) {
	a, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("hi"))),
		chainforge.WithModel("mock"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	// Call twice — must not panic and must return nil.
	if err := a.WarmMCP(context.Background()); err != nil {
		t.Errorf("WarmMCP first call: %v", err)
	}
	if err := a.WarmMCP(context.Background()); err != nil {
		t.Errorf("WarmMCP second call: %v", err)
	}
}

func TestWarmMCP_ThenRun(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("warmed"))
	a, err := chainforge.NewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if err := a.WarmMCP(context.Background()); err != nil {
		t.Fatalf("WarmMCP: %v", err)
	}
	result, err := a.Run(context.Background(), "sess", "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "warmed" {
		t.Errorf("result = %q, want %q", result, "warmed")
	}
}

func TestWarmMCP_CancelledContext_NoServers(t *testing.T) {
	// With no MCP servers, WarmMCP returns nil regardless of context state.
	a, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("hi"))),
		chainforge.WithModel("mock"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	if err := a.WarmMCP(ctx); err != nil {
		t.Errorf("WarmMCP with cancelled context and no servers: %v", err)
	}
}
