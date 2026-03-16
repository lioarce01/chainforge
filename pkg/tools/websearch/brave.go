package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// BraveSearchBackend implements SearchBackend using the Brave Search API.
// Get a free API key at https://api.search.brave.com/
type BraveSearchBackend struct {
	apiKey     string
	httpClient *http.Client
}

// NewBrave creates a Brave Search backend with the given API key.
func NewBrave(apiKey string) *BraveSearchBackend {
	return &BraveSearchBackend{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *BraveSearchBackend) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	endpoint := "https://api.search.brave.com/res/v1/web/search"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("brave: build request: %w", err)
	}

	q := url.Values{}
	q.Set("q", query)
	q.Set("count", fmt.Sprintf("%d", maxResults))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", b.apiKey)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brave: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("brave: decode response: %w", err)
	}

	out := make([]SearchResult, 0, len(result.Web.Results))
	for _, r := range result.Web.Results {
		out = append(out, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
		})
	}
	return out, nil
}
