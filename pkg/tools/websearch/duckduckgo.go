package websearch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DuckDuckGoBackend implements SearchBackend using DuckDuckGo HTML search.
// No API key required.
type DuckDuckGoBackend struct {
	httpClient *http.Client
}

// NewDuckDuckGo creates a DuckDuckGo search backend. No API key needed.
func NewDuckDuckGo() *DuckDuckGoBackend {
	return &DuckDuckGoBackend{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

var (
	// Match result links: <a class="result__a" href="...">title</a>
	reTitle   = regexp.MustCompile(`class="result__a"[^>]*href="([^"]+)"[^>]*>([^<]+)<`)
	// Match snippets: <a class="result__snippet"...>text</a>
	reSnippet = regexp.MustCompile(`class="result__snippet"[^>]*>([^<]+(?:<[^>]+>[^<]*</[^>]+>[^<]*)*)<`)
	reTag     = regexp.MustCompile(`<[^>]+>`)
)

func (d *DuckDuckGoBackend) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://html.duckduckgo.com/html/", strings.NewReader(
		url.Values{"q": {query}}.Encode(),
	))
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; chainforge/1.0)")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: read body: %w", err)
	}
	html := string(body)

	titles := reTitle.FindAllStringSubmatch(html, maxResults)
	snippets := reSnippet.FindAllStringSubmatch(html, maxResults)

	var out []SearchResult
	for i, t := range titles {
		if i >= maxResults {
			break
		}
		rawURL := t[1]
		title := strings.TrimSpace(t[2])

		// DDG wraps URLs — extract the actual URL from uddg= param
		if strings.Contains(rawURL, "uddg=") {
			if parsed, err := url.ParseQuery(strings.TrimPrefix(rawURL, "/l/?")); err == nil {
				if u := parsed.Get("uddg"); u != "" {
					rawURL, _ = url.QueryUnescape(u)
				}
			}
		}

		snippet := ""
		if i < len(snippets) {
			snippet = strings.TrimSpace(reTag.ReplaceAllString(snippets[i][1], ""))
		}

		if title == "" || rawURL == "" {
			continue
		}

		out = append(out, SearchResult{
			Title:   title,
			URL:     rawURL,
			Snippet: snippet,
		})
	}

	if len(out) == 0 {
		return []SearchResult{{
			Snippet: fmt.Sprintf("No results found for: %q", query),
		}}, nil
	}

	return out, nil
}
