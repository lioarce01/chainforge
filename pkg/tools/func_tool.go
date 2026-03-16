package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lioarce01/chainforge/pkg/core"
)

// FuncTool wraps an arbitrary function as a core.Tool.
// The function receives the raw JSON input string and returns a string result.
type FuncTool struct {
	def core.ToolDefinition
	fn  func(ctx context.Context, input string) (string, error)
}

// Func creates a new FuncTool.
// schema must be a valid JSON Schema object (use Schema.Build() to create one).
func Func(name, description string, schema json.RawMessage, fn func(ctx context.Context, input string) (string, error)) (*FuncTool, error) {
	if name == "" {
		return nil, fmt.Errorf("tools: tool name cannot be empty")
	}
	if fn == nil {
		return nil, fmt.Errorf("tools: tool function cannot be nil")
	}
	return &FuncTool{
		def: core.ToolDefinition{
			Name:        name,
			Description: description,
			InputSchema: schema,
		},
		fn: fn,
	}, nil
}

// MustFunc is like Func but panics on error (use in tests / package init).
func MustFunc(name, description string, schema json.RawMessage, fn func(ctx context.Context, input string) (string, error)) *FuncTool {
	t, err := Func(name, description, schema, fn)
	if err != nil {
		panic(err)
	}
	return t
}

func (t *FuncTool) Definition() core.ToolDefinition { return t.def }

func (t *FuncTool) Call(ctx context.Context, input string) (string, error) {
	return t.fn(ctx, input)
}
