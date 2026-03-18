package tests

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lioarce01/chainforge/pkg/tools"
)

func TestCachedTool_HitReturnsCached(t *testing.T) {
	var callCount atomic.Int32
	inner := tools.MustFunc("test", "test tool", nil, func(ctx context.Context, input string) (string, error) {
		callCount.Add(1)
		return "result", nil
	})
	cached := tools.NewCachedTool(inner)

	r1, err1 := cached.Call(context.Background(), `{"x":1}`)
	r2, err2 := cached.Call(context.Background(), `{"x":1}`)

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if r1 != "result" || r2 != "result" {
		t.Errorf("unexpected results: %q, %q", r1, r2)
	}
	if n := callCount.Load(); n != 1 {
		t.Errorf("inner called %d times, want 1", n)
	}
}

func TestCachedTool_MissCallsInner(t *testing.T) {
	var callCount atomic.Int32
	inner := tools.MustFunc("test", "test tool", nil, func(ctx context.Context, input string) (string, error) {
		callCount.Add(1)
		return "r-" + input, nil
	})
	cached := tools.NewCachedTool(inner)

	cached.Call(context.Background(), `{"x":1}`)
	cached.Call(context.Background(), `{"x":2}`)
	cached.Call(context.Background(), `{"x":3}`)

	if n := callCount.Load(); n != 3 {
		t.Errorf("inner called %d times, want 3", n)
	}
}

func TestCachedTool_ErrorIsCached(t *testing.T) {
	var callCount atomic.Int32
	wantErr := errors.New("boom")
	inner := tools.MustFunc("test", "test tool", nil, func(ctx context.Context, input string) (string, error) {
		callCount.Add(1)
		return "", wantErr
	})
	cached := tools.NewCachedTool(inner)

	_, err1 := cached.Call(context.Background(), `{}`)
	_, err2 := cached.Call(context.Background(), `{}`)

	if !errors.Is(err1, wantErr) || !errors.Is(err2, wantErr) {
		t.Errorf("expected wantErr, got %v, %v", err1, err2)
	}
	if n := callCount.Load(); n != 1 {
		t.Errorf("inner called %d times, want 1 (error should be cached)", n)
	}
}

func TestCachedTool_InvalidateAllFlushes(t *testing.T) {
	var callCount atomic.Int32
	inner := tools.MustFunc("test", "test tool", nil, func(ctx context.Context, input string) (string, error) {
		callCount.Add(1)
		return "result", nil
	})
	cached := tools.NewCachedTool(inner)

	cached.Call(context.Background(), `{}`)
	cached.InvalidateAll()
	cached.Call(context.Background(), `{}`)

	if n := callCount.Load(); n != 2 {
		t.Errorf("inner called %d times, want 2 (after invalidation)", n)
	}
}

func TestCachedTool_ConcurrentSafe(t *testing.T) {
	var callCount atomic.Int32
	inner := tools.MustFunc("test", "test tool", nil, func(ctx context.Context, input string) (string, error) {
		callCount.Add(1)
		return "result", nil
	})
	cached := tools.NewCachedTool(inner)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cached.Call(context.Background(), `{"same":"input"}`)
		}()
	}
	wg.Wait()

	if n := callCount.Load(); n != 1 {
		t.Errorf("inner called %d times, want exactly 1 (concurrent same input)", n)
	}
}

func TestCachedTool_DefinitionDelegates(t *testing.T) {
	inner := tools.MustFunc("my-tool", "my description", nil, func(ctx context.Context, input string) (string, error) {
		return "", nil
	})
	cached := tools.NewCachedTool(inner)

	def := cached.Definition()
	if def.Name != "my-tool" {
		t.Errorf("expected Name=%q, got %q", "my-tool", def.Name)
	}
	if def.Description != "my description" {
		t.Errorf("expected Description=%q, got %q", "my description", def.Description)
	}
}

// --- TTL tests ---

func TestCachedToolTTLExpiry(t *testing.T) {
	var callCount atomic.Int32
	inner := tools.MustFunc("test", "test", nil, func(ctx context.Context, input string) (string, error) {
		callCount.Add(1)
		return "result", nil
	})
	cached := tools.NewCachedToolWithTTL(inner, 50*time.Millisecond)

	cached.Call(context.Background(), `{}`)
	if n := callCount.Load(); n != 1 {
		t.Fatalf("expected 1 call, got %d", n)
	}

	time.Sleep(80 * time.Millisecond) // wait for TTL expiry

	cached.Call(context.Background(), `{}`)
	if n := callCount.Load(); n != 2 {
		t.Errorf("expected 2 calls after expiry, got %d", n)
	}
}

func TestCachedToolTTLNotExpired(t *testing.T) {
	var callCount atomic.Int32
	inner := tools.MustFunc("test", "test", nil, func(ctx context.Context, input string) (string, error) {
		callCount.Add(1)
		return "result", nil
	})
	cached := tools.NewCachedToolWithTTL(inner, 200*time.Millisecond)

	cached.Call(context.Background(), `{}`)
	cached.Call(context.Background(), `{}`) // should hit cache

	if n := callCount.Load(); n != 1 {
		t.Errorf("expected 1 call (cache hit), got %d", n)
	}
}

func TestCachedToolNoTTL(t *testing.T) {
	var callCount atomic.Int32
	inner := tools.MustFunc("test", "test", nil, func(ctx context.Context, input string) (string, error) {
		callCount.Add(1)
		return "result", nil
	})
	cached := tools.NewCachedTool(inner) // no TTL

	cached.Call(context.Background(), `{}`)
	time.Sleep(10 * time.Millisecond)
	cached.Call(context.Background(), `{}`)

	if n := callCount.Load(); n != 1 {
		t.Errorf("expected 1 call (no TTL = permanent cache), got %d", n)
	}
}
