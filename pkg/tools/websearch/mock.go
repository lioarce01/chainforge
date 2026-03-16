package websearch

import (
	"context"
	"fmt"
)

// MockSearchBackend returns scripted results for testing.
type MockSearchBackend struct {
	Results []SearchResult
	Err     error
}

// NewMock creates a MockSearchBackend with the given results.
func NewMock(results ...SearchResult) *MockSearchBackend {
	return &MockSearchBackend{Results: results}
}

func (m *MockSearchBackend) Search(_ context.Context, query string, maxResults int) ([]SearchResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	results := m.Results
	if len(results) == 0 {
		results = []SearchResult{
			{
				Title:   fmt.Sprintf("Result for: %s", query),
				URL:     "https://example.com/result",
				Snippet: fmt.Sprintf("This is a mock search result for the query: %q", query),
			},
		}
	}
	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}
	return results, nil
}
