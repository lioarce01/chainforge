package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

// HTMLLoader loads an HTML file and strips all tags, returning clean text.
type HTMLLoader struct {
	Path string
}

// NewHTMLLoader creates an HTMLLoader for the given file path.
func NewHTMLLoader(path string) *HTMLLoader {
	return &HTMLLoader{Path: path}
}

// Load reads the HTML file, strips all tags, and returns a single Document.
func (l *HTMLLoader) Load() ([]Document, error) {
	return LoadHTMLFile(l.Path)
}

// LoadHTMLFile loads a single HTML file, strips tags, and returns a Document.
func LoadHTMLFile(path string) ([]Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("loader: open html %q: %w", path, err)
	}
	defer f.Close()

	doc, err := html.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("loader: parse html %q: %w", path, err)
	}

	var sb strings.Builder
	extractText(doc, &sb)
	content := strings.TrimSpace(sb.String())
	if content == "" {
		return nil, fmt.Errorf("loader: no text found in %q", path)
	}

	return []Document{{
		ID:      filepath.Base(path),
		Content: content,
		Source:  path,
	}}, nil
}

// stripHTML parses an HTML string and returns the visible text content.
func stripHTML(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		// Fall back to raw string on parse failure.
		return htmlStr
	}
	var sb strings.Builder
	extractText(doc, &sb)
	return strings.TrimSpace(sb.String())
}

// extractText walks the HTML node tree and writes text node content to sb.
// Script and style elements are skipped entirely.
func extractText(n *html.Node, sb *strings.Builder) {
	if n == nil {
		return
	}
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "head":
			return // skip these subtrees
		}
	}
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			if sb.Len() > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(text)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, sb)
	}
}
