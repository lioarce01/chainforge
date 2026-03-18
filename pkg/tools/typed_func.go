package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// TypedFunc creates a Tool from a strongly-typed handler function.
// TInput must be a struct. The JSON schema is generated automatically from
// the struct's field tags (see SchemaFromStruct). The handler receives a
// parsed TInput value — no manual JSON unmarshalling required.
//
//	type SearchInput struct {
//	    Query string `json:"query" cf:"required,description=The search query"`
//	    Limit int    `json:"limit" cf:"description=Max results"`
//	}
//
//	tool, err := tools.TypedFunc[SearchInput]("web_search", "Search the web",
//	    func(ctx context.Context, in SearchInput) (string, error) {
//	        return search(in.Query, in.Limit)
//	    })
func TypedFunc[TInput any](
	name, description string,
	fn func(ctx context.Context, in TInput) (string, error),
) (*FuncTool, error) {
	schema, err := SchemaFromStruct[TInput]()
	if err != nil {
		return nil, fmt.Errorf("tools.TypedFunc %q: %w", name, err)
	}
	return Func(name, description, schema, func(ctx context.Context, raw string) (string, error) {
		var in TInput
		if err := json.Unmarshal([]byte(raw), &in); err != nil {
			return "", fmt.Errorf("tool %q: invalid input: %w", name, err)
		}
		return fn(ctx, in)
	})
}

// MustTypedFunc is like TypedFunc but panics on schema generation errors.
// Use in package-level var declarations or test setup where a non-struct
// TInput is a programmer error.
func MustTypedFunc[TInput any](
	name, description string,
	fn func(ctx context.Context, in TInput) (string, error),
) *FuncTool {
	t, err := TypedFunc[TInput](name, description, fn)
	if err != nil {
		panic(err)
	}
	return t
}
