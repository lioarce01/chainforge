package bench_test

import (
	"context"
	"fmt"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/benchutil"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
)

// BenchmarkStreamDrain measures how fast we can drain a stream channel.
func BenchmarkStreamDrain(b *testing.B) {
	provider := benchutil.NewMockProvider(benchutil.LargeResponseText(1024))
	provider.ChunkSize = 64
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
		ch := agent.RunStream(ctx, fmt.Sprintf("stream-bench-%d", i), "stream test")
		var textLen int
		for ev := range ch {
			textLen += len(ev.TextDelta)
		}
		_ = textLen
	}
}

// BenchmarkStreamChunkSizes compares drain overhead at different chunk granularities.
func BenchmarkStreamChunkSizes(b *testing.B) {
	for _, chunkSize := range []int{8, 64, 256, 1024} {
		chunkSize := chunkSize
		b.Run(fmt.Sprintf("chunk=%d", chunkSize), func(b *testing.B) {
			provider := benchutil.NewMockProvider(benchutil.LargeResponseText(2048))
			provider.ChunkSize = chunkSize
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
				ch := agent.RunStream(ctx, fmt.Sprintf("chunk-bench-%d-%d", chunkSize, i), "stream")
				for range ch {
				}
			}
		})
	}
}

// BenchmarkStreamConcurrent measures concurrent stream draining.
func BenchmarkStreamConcurrent(b *testing.B) {
	provider := benchutil.NewMockProvider(benchutil.LargeResponseText(512))
	provider.ChunkSize = 32
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
			ch := agent.RunStream(ctx, fmt.Sprintf("conc-stream-%d", i), "parallel stream")
			i++
			for range ch {
			}
		}
	})
}

// BenchmarkStreamResponseSizes shows how response size affects drain time.
func BenchmarkStreamResponseSizes(b *testing.B) {
	for _, size := range []int{128, 512, 2048, 8192} {
		size := size
		b.Run(fmt.Sprintf("bytes=%d", size), func(b *testing.B) {
			provider := benchutil.NewMockProvider(benchutil.LargeResponseText(size))
			provider.ChunkSize = 64
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
			b.SetBytes(int64(size))
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ch := agent.RunStream(ctx, fmt.Sprintf("size-bench-%d-%d", size, i), "stream size test")
				for range ch {
				}
			}
		})
	}
}
