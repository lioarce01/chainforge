// Package rag provides Retrieval-Augmented Generation (RAG) primitives:
// document loading, chunking, embedding, storage, and retrieval.
package rag

// Document is a piece of text with metadata, used as the unit of retrieval.
type Document struct {
	// ID uniquely identifies the document (e.g. filename + chunk index).
	ID string
	// Content is the text content of the document chunk.
	Content string
	// Source is the origin of the document (e.g. file path, URL).
	Source string
	// Metadata holds arbitrary key-value pairs for filtering or display.
	Metadata map[string]any
}
