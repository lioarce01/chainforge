package tools

import (
	"context"
	"sync"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Compile-time guard
var _ core.Tool = (*CachedTool)(nil)

type cacheEntry struct {
	result string
	err    error
}

// CachedTool wraps a core.Tool and memoizes Call results by input JSON string.
// Errors are also cached to avoid repeated calls on bad input.
// Safe for concurrent use.
type CachedTool struct {
	inner core.Tool
	mu    sync.Mutex
	cache map[string]cacheEntry
}

// NewCachedTool wraps t with a result cache.
func NewCachedTool(t core.Tool) *CachedTool {
	return &CachedTool{
		inner: t,
		cache: make(map[string]cacheEntry),
	}
}

// Definition delegates to the wrapped tool.
func (c *CachedTool) Definition() core.ToolDefinition {
	return c.inner.Definition()
}

// Call returns the cached result for input if available; otherwise calls the
// inner tool, caches the result, and returns it.
// The lock is held for the duration of the inner call to ensure exactly-once
// semantics when concurrent goroutines call with the same input.
func (c *CachedTool) Call(ctx context.Context, input string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.cache[input]; ok {
		return e.result, e.err
	}

	result, err := c.inner.Call(ctx, input)
	c.cache[input] = cacheEntry{result: result, err: err}
	return result, err
}

// InvalidateAll clears all cached entries.
func (c *CachedTool) InvalidateAll() {
	c.mu.Lock()
	c.cache = make(map[string]cacheEntry)
	c.mu.Unlock()
}
