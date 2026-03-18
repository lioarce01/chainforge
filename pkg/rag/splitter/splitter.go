// Package splitter provides text splitters for chunking documents before embedding.
package splitter

// Splitter splits a text string into a slice of chunks.
type Splitter interface {
	Split(text string) []string
}
