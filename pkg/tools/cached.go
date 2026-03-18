package tools

import (
	"context"
	"sync"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Compile-time guard
var _ core.Tool = (*CachedTool)(nil)

type cacheEntry struct {
	result   string
	err      error
	cachedAt time.Time
}

// CachedTool wraps a core.Tool and memoizes Call results by input JSON string.
// Errors are also cached to avoid repeated calls on bad input.
// Safe for concurrent use; read-heavy workloads benefit from the RWMutex fast path.
type CachedTool struct {
	inner core.Tool
	mu    sync.RWMutex
	cache map[string]cacheEntry
	ttl   time.Duration // 0 = never expires
}

// NewCachedTool wraps t with a result cache (no TTL — entries never expire).
func NewCachedTool(t core.Tool) *CachedTool {
	return &CachedTool{
		inner: t,
		cache: make(map[string]cacheEntry),
	}
}

// NewCachedToolWithTTL wraps t with a result cache that expires entries after ttl.
// A zero ttl is equivalent to NewCachedTool (no expiry).
func NewCachedToolWithTTL(t core.Tool, ttl time.Duration) *CachedTool {
	return &CachedTool{
		inner: t,
		cache: make(map[string]cacheEntry),
		ttl:   ttl,
	}
}

// Definition delegates to the wrapped tool.
func (c *CachedTool) Definition() core.ToolDefinition {
	return c.inner.Definition()
}

// Call returns the cached result for input if available and not expired;
// otherwise calls the inner tool, caches the result, and returns it.
// Uses a double-check pattern: read lock for the common cache-hit path,
// write lock only on a miss to ensure exactly-once inner invocation.
func (c *CachedTool) Call(ctx context.Context, input string) (string, error) {
	// Fast path: read lock — no blocking for concurrent cache hits.
	c.mu.RLock()
	if e, ok := c.cache[input]; ok {
		if c.ttl == 0 || time.Since(e.cachedAt) < c.ttl {
			c.mu.RUnlock()
			return e.result, e.err
		}
	}
	c.mu.RUnlock()

	// Miss or expired: acquire write lock and double-check before calling inner.
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.cache[input]; ok {
		if c.ttl == 0 || time.Since(e.cachedAt) < c.ttl {
			return e.result, e.err
		}
		// Still expired — fall through to re-invoke.
	}

	result, err := c.inner.Call(ctx, input)
	c.cache[input] = cacheEntry{result: result, err: err, cachedAt: time.Now()}
	return result, err
}

// InvalidateAll clears all cached entries.
func (c *CachedTool) InvalidateAll() {
	c.mu.Lock()
	c.cache = make(map[string]cacheEntry)
	c.mu.Unlock()
}
