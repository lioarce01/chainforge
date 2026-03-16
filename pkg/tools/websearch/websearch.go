package websearch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/tools"
)

// SearchResult is a single search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// SearchBackend is the interface for search implementations.
type SearchBackend interface {
	Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error)
}

// WebSearchTool wraps a SearchBackend as a core.Tool.
type WebSearchTool struct {
	def     core.ToolDefinition
	backend SearchBackend
}

// New creates a new WebSearchTool with the given backend.
func New(backend SearchBackend) *WebSearchTool {
	schema := tools.NewSchema().
		Add("query", tools.Property{
			Type:        tools.TypeString,
			Description: "Search query",
		}, true).
		Add("max_results", tools.Property{
			Type:        tools.TypeInteger,
			Description: "Maximum number of results to return (default: 5)",
		}, false).
		MustBuild()

	return &WebSearchTool{
		def: core.ToolDefinition{
			Name:        "web_search",
			Description: "Search the web for current information. Returns titles, URLs, and snippets.",
			InputSchema: schema,
		},
		backend: backend,
	}
}

func (w *WebSearchTool) Definition() core.ToolDefinition { return w.def }

func (w *WebSearchTool) Call(ctx context.Context, input string) (string, error) {
	var req struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("websearch: invalid input: %w", err)
	}
	if req.Query == "" {
		return "", fmt.Errorf("websearch: query is required")
	}
	if req.MaxResults <= 0 {
		req.MaxResults = 5
	}

	results, err := w.backend.Search(ctx, req.Query, req.MaxResults)
	if err != nil {
		return "", fmt.Errorf("websearch: %w", err)
	}

	if len(results) == 0 {
		return "No results found.", nil
	}

	b, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("websearch: marshal results: %w", err)
	}
	return string(b), nil
}
