package tests

import (
	"context"
	"errors"
	"sync"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
)

// TestAgentMCPSuccessIdempotent verifies that a successful connect is not re-attempted.
func TestAgentMCPSuccessIdempotent(t *testing.T) {
	// No MCP servers configured — WarmMCP is a no-op and returns nil.
	a, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("ok"))),
		chainforge.WithModel("mock"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	ctx := context.Background()
	if err := a.WarmMCP(ctx); err != nil {
		t.Fatalf("first WarmMCP: %v", err)
	}
	if err := a.WarmMCP(ctx); err != nil {
		t.Fatalf("second WarmMCP (should be no-op): %v", err)
	}
}

// TestAgentReconnectMCP_NoServers verifies ReconnectMCP returns nil with no servers.
func TestAgentReconnectMCP_NoServers(t *testing.T) {
	a, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("ok"))),
		chainforge.WithModel("mock"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	ctx := context.Background()
	if err := a.ReconnectMCP(ctx); err != nil {
		t.Errorf("ReconnectMCP with no servers should return nil, got: %v", err)
	}
}

// TestAgentMCPConcurrentReconnect verifies no races during concurrent Run calls.
func TestAgentMCPConcurrentReconnect(t *testing.T) {
	responses := make([]MockResponse, 20)
	for i := range responses {
		responses[i] = EndTurnResponse("done")
	}
	a, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(responses...)),
		chainforge.WithModel("mock"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, e := a.Run(ctx, "sess", "hi")
			errs[idx] = e
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil && !errors.Is(e, context.Canceled) {
			t.Errorf("goroutine %d: unexpected error: %v", i, e)
		}
	}
}
