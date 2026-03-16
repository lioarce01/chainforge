package core

import (
	"errors"
	"fmt"
)

// Sentinel errors — use errors.Is() to check.
var (
	ErrMaxIterations = errors.New("agent: max iterations reached")
	ErrToolNotFound  = errors.New("agent: tool not found")
	ErrProviderError = errors.New("agent: provider error")
	ErrNoProvider    = errors.New("agent: no provider configured")
	ErrNoModel       = errors.New("agent: no model configured")
)

// ToolError wraps a tool execution failure with the tool name.
type ToolError struct {
	ToolName string
	Err      error
}

func (e *ToolError) Error() string {
	return fmt.Sprintf("tool %q: %v", e.ToolName, e.Err)
}

func (e *ToolError) Unwrap() error { return e.Err }

// ProviderError wraps a provider failure with the provider name and HTTP status if applicable.
type ProviderError struct {
	Provider   string
	StatusCode int
	Err        error
}

func (e *ProviderError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("provider %q (status %d): %v", e.Provider, e.StatusCode, e.Err)
	}
	return fmt.Sprintf("provider %q: %v", e.Provider, e.Err)
}

func (e *ProviderError) Unwrap() error { return e.Err }
