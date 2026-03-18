package inmemory_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
)

func msg(role core.Role, content string) core.Message {
	return core.Message{Role: role, Content: content}
}

// TestInMemoryBasic verifies existing behaviour: get/append/clear round-trip.
func TestInMemoryBasic(t *testing.T) {
	s := inmemory.New()
	ctx := context.Background()

	msgs, err := s.Get(ctx, "s1")
	if err != nil || len(msgs) != 0 {
		t.Fatalf("empty get: err=%v msgs=%v", err, msgs)
	}

	if err := s.Append(ctx, "s1", msg(core.RoleUser, "hello")); err != nil {
		t.Fatalf("append: %v", err)
	}

	msgs, err = s.Get(ctx, "s1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "hello" {
		t.Errorf("unexpected msgs: %v", msgs)
	}

	if err := s.Clear(ctx, "s1"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	msgs, _ = s.Get(ctx, "s1")
	if len(msgs) != 0 {
		t.Errorf("expected empty after clear, got %v", msgs)
	}
}

// TestInMemoryTTLExpiry verifies that expired sessions return nil on Get.
func TestInMemoryTTLExpiry(t *testing.T) {
	s := inmemory.New(inmemory.WithTTL(50 * time.Millisecond))
	ctx := context.Background()

	if err := s.Append(ctx, "sess", msg(core.RoleUser, "hi")); err != nil {
		t.Fatalf("append: %v", err)
	}

	// Should still be there immediately.
	msgs, _ := s.Get(ctx, "sess")
	if len(msgs) == 0 {
		t.Fatal("expected messages before TTL expiry")
	}

	time.Sleep(80 * time.Millisecond) // wait for expiry

	msgs, err := s.Get(ctx, "sess")
	if err != nil {
		t.Fatalf("get after expiry: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected nil after TTL expiry, got %v", msgs)
	}
}

// TestInMemoryMaxMessages verifies that only the most recent n messages are kept.
func TestInMemoryMaxMessages(t *testing.T) {
	const max = 3
	s := inmemory.New(inmemory.WithMaxMessages(max))
	ctx := context.Background()

	for i := 0; i < max+2; i++ {
		_ = s.Append(ctx, "sess", msg(core.RoleUser, string(rune('A'+i))))
	}

	msgs, err := s.Get(ctx, "sess")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(msgs) != max {
		t.Errorf("expected %d messages, got %d", max, len(msgs))
	}
	// First kept message should be 'C' (oldest dropped: A, B).
	if msgs[0].Content != "C" {
		t.Errorf("expected oldest kept = %q, got %q", "C", msgs[0].Content)
	}
	if msgs[max-1].Content != "E" {
		t.Errorf("expected newest = %q, got %q", "E", msgs[max-1].Content)
	}
}

// TestInMemoryConcurrentTTL verifies no races during concurrent get/append on expiring sessions.
func TestInMemoryConcurrentTTL(t *testing.T) {
	s := inmemory.New(inmemory.WithTTL(20 * time.Millisecond))
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = s.Append(ctx, "shared", msg(core.RoleUser, "x"))
		}()
		go func() {
			defer wg.Done()
			_, _ = s.Get(ctx, "shared")
		}()
	}
	wg.Wait()
}
