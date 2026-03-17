package bench_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
)

// BenchmarkInMemoryAppend measures in-memory store Append throughput.
func BenchmarkInMemoryAppend(b *testing.B) {
	store := inmemory.New()
	ctx := context.Background()
	msg := core.Message{Role: core.RoleUser, Content: "benchmark message content"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := store.Append(ctx, "bench-session", msg); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkInMemoryGet measures Get throughput as history grows.
func BenchmarkInMemoryGet(b *testing.B) {
	store := inmemory.New()
	ctx := context.Background()

	// Pre-populate with 100 messages
	for i := 0; i < 100; i++ {
		_ = store.Append(ctx, "bench-get", core.Message{
			Role:    core.RoleUser,
			Content: fmt.Sprintf("message %d with some content", i),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := store.Get(ctx, "bench-get")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkInMemoryGetGrowingHistory measures how Get scales with history size.
func BenchmarkInMemoryGetGrowingHistory(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		size := size
		b.Run(fmt.Sprintf("messages=%d", size), func(b *testing.B) {
			store := inmemory.New()
			ctx := context.Background()
			for i := 0; i < size; i++ {
				_ = store.Append(ctx, "session", core.Message{
					Role:    core.RoleAssistant,
					Content: fmt.Sprintf("message content number %d", i),
				})
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := store.Get(ctx, "session")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkInMemoryConcurrentSessions measures performance with many sessions.
func BenchmarkInMemoryConcurrentSessions(b *testing.B) {
	store := inmemory.New()
	ctx := context.Background()
	msg := core.Message{Role: core.RoleUser, Content: "concurrent session message"}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sid := fmt.Sprintf("concurrent-session-%d", i%100)
			i++
			if err := store.Append(ctx, sid, msg); err != nil {
				b.Error(err)
			}
		}
	})
}

// BenchmarkQdrantAppend requires a running Qdrant instance.
// Skipped automatically if QDRANT_URL is not set.
func BenchmarkQdrantAppend(b *testing.B) {
	qdrantURL := os.Getenv("QDRANT_URL")
	if qdrantURL == "" {
		b.Skip("QDRANT_URL not set — skipping Qdrant benchmarks")
	}
	b.Logf("Qdrant benchmarks would run against %s (implementation pending)", qdrantURL)
	b.Skip("Qdrant benchmark not yet wired (add qdrant store init here)")
}

// BenchmarkQdrantGet requires a running Qdrant instance.
func BenchmarkQdrantGet(b *testing.B) {
	qdrantURL := os.Getenv("QDRANT_URL")
	if qdrantURL == "" {
		b.Skip("QDRANT_URL not set — skipping Qdrant benchmarks")
	}
	b.Skip("Qdrant benchmark not yet wired (add qdrant store init here)")
}
