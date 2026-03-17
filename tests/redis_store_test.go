package tests

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/redis"
)

func redisAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	return addr
}

func TestRedis_ErrNoAddr(t *testing.T) {
	_, err := redis.New("")
	if !errors.Is(err, redis.ErrNoAddr) {
		t.Fatalf("want ErrNoAddr, got %v", err)
	}
}

func TestRedis_Roundtrip(t *testing.T) {
	store, err := redis.New(redisAddr(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	t.Cleanup(func() { store.Clear(ctx, "redis-roundtrip") }) //nolint:errcheck

	err = store.Append(ctx, "redis-roundtrip",
		core.Message{Role: core.RoleUser, Content: "hello"},
		core.Message{Role: core.RoleAssistant, Content: "world"},
	)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	msgs, err := store.Get(ctx, "redis-roundtrip")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("want 2, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" || msgs[1].Content != "world" {
		t.Fatalf("unexpected messages: %v", msgs)
	}
}

func TestRedis_Clear(t *testing.T) {
	store, err := redis.New(redisAddr(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	t.Cleanup(func() { store.Clear(ctx, "redis-clear") }) //nolint:errcheck

	_ = store.Append(ctx, "redis-clear", core.Message{Role: core.RoleUser, Content: "hello"})

	if err := store.Clear(ctx, "redis-clear"); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	msgs, err := store.Get(ctx, "redis-clear")
	if err != nil {
		t.Fatalf("Get after Clear: %v", err)
	}
	if msgs != nil {
		t.Fatalf("want nil after Clear, got %v", msgs)
	}
}

func TestRedis_WithTTLAccepted(t *testing.T) {
	store, err := redis.New(redisAddr(t), redis.WithTTL(24*time.Hour))
	if err != nil {
		t.Fatalf("New with TTL: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	t.Cleanup(func() { store.Clear(ctx, "redis-ttl") }) //nolint:errcheck

	if err := store.Append(ctx, "redis-ttl", core.Message{Role: core.RoleUser, Content: "ttl test"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	msgs, err := store.Get(ctx, "redis-ttl")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("want 1, got %d", len(msgs))
	}
}

func TestRedis_NewFromURL(t *testing.T) {
	addr := redisAddr(t)
	store, err := redis.NewFromURL("redis://" + addr)
	if err != nil {
		t.Fatalf("NewFromURL: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	t.Cleanup(func() { store.Clear(ctx, "redis-url") }) //nolint:errcheck

	_ = store.Append(ctx, "redis-url", core.Message{Role: core.RoleUser, Content: "from url"})
	msgs, err := store.Get(ctx, "redis-url")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "from url" {
		t.Fatalf("unexpected: %v", msgs)
	}
}

func TestRedis_KeyPrefixIsolation(t *testing.T) {
	addr := redisAddr(t)
	storeA, err := redis.New(addr, redis.WithKeyPrefix("prefix-a"))
	if err != nil {
		t.Fatalf("New A: %v", err)
	}
	storeB, err := redis.New(addr, redis.WithKeyPrefix("prefix-b"))
	if err != nil {
		t.Fatalf("New B: %v", err)
	}
	t.Cleanup(func() {
		storeA.Close()
		storeB.Close()
	})

	ctx := context.Background()
	t.Cleanup(func() {
		storeA.Clear(ctx, "same-session") //nolint:errcheck
		storeB.Clear(ctx, "same-session") //nolint:errcheck
	})

	_ = storeA.Append(ctx, "same-session", core.Message{Role: core.RoleUser, Content: "from A"})
	_ = storeB.Append(ctx, "same-session", core.Message{Role: core.RoleUser, Content: "from B"})

	msgsA, _ := storeA.Get(ctx, "same-session")
	msgsB, _ := storeB.Get(ctx, "same-session")

	if len(msgsA) != 1 || msgsA[0].Content != "from A" {
		t.Fatalf("prefix A isolation broken: %v", msgsA)
	}
	if len(msgsB) != 1 || msgsB[0].Content != "from B" {
		t.Fatalf("prefix B isolation broken: %v", msgsB)
	}
}
