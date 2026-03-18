package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadFile loads a single file (.txt, .md, .json, or .html) as a Document.
// For HTML files the tags are stripped before returning the text.
func LoadFile(path string) ([]Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loader: read file %q: %w", path, err)
	}

	content := string(data)
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".html", ".htm":
		content = stripHTML(content)
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("loader: file %q is empty after processing", path)
	}

	return []Document{{
		ID:      filepath.Base(path),
		Content: content,
		Source:  path,
	}}, nil
}

// LoadDir loads all files in dir whose names match pattern (e.g. "*.txt", "*.md").
// Subdirectories are not traversed. Files that fail to load are skipped with an error logged.
func LoadDir(dir, pattern string) ([]Document, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("loader: read dir %q: %w", dir, err)
	}

	var docs []Document
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matched, err := filepath.Match(pattern, e.Name())
		if err != nil {
			return nil, fmt.Errorf("loader: invalid pattern %q: %w", pattern, err)
		}
		if !matched {
			continue
		}
		fileDocs, err := LoadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			// Skip files that can't be loaded; caller can check result length.
			continue
		}
		docs = append(docs, fileDocs...)
	}
	return docs, nil
}
