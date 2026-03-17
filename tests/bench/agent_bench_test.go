package bench_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/benchutil"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
)

// BenchmarkAgentRun measures the agent loop overhead with a mock provider
// at zero latency. This captures pure framework cost: memory lookups,
// message marshalling, and goroutine scheduling.
func BenchmarkAgentRun(b *testing.B) {
	provider := benchutil.NewMockProvider(benchutil.LargeResponseText(512))
	mem := inmemory.New()
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(provider),
		chainforge.WithModel("mock-model"),
		chainforge.WithMemory(mem),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer agent.Close()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := agent.Run(ctx, fmt.Sprintf("bench-session-%d", i), "hello")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAgentRunStream measures streaming drain overhead.
func BenchmarkAgentRunStream(b *testing.B) {
	provider := benchutil.NewMockProvider(benchutil.LargeResponseText(512))
	mem := inmemory.New()
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(provider),
		chainforge.WithModel("mock-model"),
		chainforge.WithMemory(mem),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer agent.Close()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ch := agent.RunStream(ctx, fmt.Sprintf("bench-stream-%d", i), "hello")
		for range ch {
		}
	}
}

// BenchmarkAgentRunWithTool measures the tool dispatch overhead.
func BenchmarkAgentRunWithTool(b *testing.B) {
	provider := benchutil.NewMockToolProvider(
		"echo",
		`{"message":"bench"}`,
		"tool result processed",
	)

	echoTool := &echoTool{}
	mem := inmemory.New()

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(provider),
		chainforge.WithModel("mock-model"),
		chainforge.WithTools(echoTool),
		chainforge.WithMemory(mem),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer agent.Close()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		provider.Reset()
		_, err := agent.Run(ctx, fmt.Sprintf("tool-bench-%d", i), "use the echo tool")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAgentConcurrent measures throughput under concurrent load.
// Simulates multiple independent sessions running simultaneously.
func BenchmarkAgentConcurrent(b *testing.B) {
	const concurrency = 8

	provider := benchutil.NewMockProvider("concurrent response text here")
	mem := inmemory.New()
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(provider),
		chainforge.WithModel("mock-model"),
		chainforge.WithMemory(mem),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer agent.Close()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sessionID := fmt.Sprintf("concurrent-session-%d-%d", concurrency, i)
			i++
			_, err := agent.Run(ctx, sessionID, "hello concurrent")
			if err != nil {
				b.Error(err)
			}
		}
	})
}

// BenchmarkAgentSharedSession measures repeated calls on the same session
// (history grows each iteration — tests memory/history load performance).
func BenchmarkAgentSharedSession(b *testing.B) {
	provider := benchutil.NewMockProvider("response for shared session")
	mem := inmemory.New()
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(provider),
		chainforge.WithModel("mock-model"),
		chainforge.WithMemory(mem),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer agent.Close()

	ctx := context.Background()
	const sessionID = "shared-bench-session"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := agent.Run(ctx, sessionID, fmt.Sprintf("message %d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAgentRunParallelSessions runs N goroutines each with their own session.
func BenchmarkAgentRunParallelSessions(b *testing.B) {
	for _, n := range []int{1, 4, 16} {
		n := n
		b.Run(fmt.Sprintf("goroutines=%d", n), func(b *testing.B) {
			provider := benchutil.NewMockProvider("parallel session response")
			mem := inmemory.New()
			agent, err := chainforge.NewAgent(
				chainforge.WithProvider(provider),
				chainforge.WithModel("mock-model"),
				chainforge.WithMemory(mem),
			)
			if err != nil {
				b.Fatal(err)
			}
			defer agent.Close()

			ctx := context.Background()
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				var wg sync.WaitGroup
				wg.Add(n)
				for j := 0; j < n; j++ {
					j := j
					go func() {
						defer wg.Done()
						sid := fmt.Sprintf("par-session-%d-%d", i, j)
						if _, err := agent.Run(ctx, sid, "parallel hello"); err != nil {
							b.Error(err)
						}
					}()
				}
				wg.Wait()
			}
		})
	}
}

// echoTool is a minimal tool used in benchmarks.
type echoTool struct{}

func (e *echoTool) Definition() core.ToolDefinition {
	return core.ToolDefinition{
		Name:        "echo",
		Description: "Echoes the input message",
		InputSchema: []byte(`{"type":"object","properties":{"message":{"type":"string"}},"required":["message"]}`),
	}
}

func (e *echoTool) Call(_ context.Context, input string) (string, error) {
	return input, nil
}
