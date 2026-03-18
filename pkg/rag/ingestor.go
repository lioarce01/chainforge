package rag

import (
	"context"
	"fmt"

	"github.com/lioarce01/chainforge/pkg/rag/splitter"
)

// IngestOption configures a single Ingest call.
type IngestOption func(*ingestOptions)

type ingestOptions struct {
	chunkSize    int
	chunkOverlap int
	splitter     splitter.Splitter
}

func defaultIngestOptions() ingestOptions {
	return ingestOptions{
		chunkSize:    512,
		chunkOverlap: 64,
	}
}

// WithChunkSize sets the target chunk size in runes (default: 512).
func WithChunkSize(n int) IngestOption {
	return func(o *ingestOptions) { o.chunkSize = n }
}

// WithChunkOverlap sets the overlap between adjacent chunks in runes (default: 64).
func WithChunkOverlap(n int) IngestOption {
	return func(o *ingestOptions) { o.chunkOverlap = n }
}

// WithSplitter overrides the default FixedSizeSplitter with a custom one.
func WithSplitter(s splitter.Splitter) IngestOption {
	return func(o *ingestOptions) { o.splitter = s }
}

// DocumentStorer stores chunked documents (implemented by QdrantIngestor and others).
type DocumentStorer interface {
	Store(ctx context.Context, sessionID string, docs []Document) error
}

// Ingestor splits documents into chunks and stores them via a DocumentStorer.
type Ingestor struct {
	storer DocumentStorer
}

// NewIngestor creates an Ingestor backed by storer.
func NewIngestor(storer DocumentStorer) *Ingestor {
	return &Ingestor{storer: storer}
}

// Ingest splits docs into chunks and stores them under sessionID.
// sessionID acts as a namespace (e.g. "kb", "project-docs").
func (ing *Ingestor) Ingest(ctx context.Context, sessionID string, docs []Document, opts ...IngestOption) error {
	o := defaultIngestOptions()
	for _, opt := range opts {
		opt(&o)
	}

	s := o.splitter
	if s == nil {
		s = splitter.NewFixedSizeSplitter(o.chunkSize, o.chunkOverlap)
	}

	var chunks []Document
	for _, doc := range docs {
		parts := s.Split(doc.Content)
		if len(parts) == 0 {
			continue
		}
		for i, part := range parts {
			chunk := Document{
				ID:      fmt.Sprintf("%s:chunk:%d", doc.ID, i),
				Content: part,
				Source:  doc.Source,
				Metadata: copyMetadata(doc.Metadata),
			}
			chunks = append(chunks, chunk)
		}
	}

	if len(chunks) == 0 {
		return nil
	}

	return ing.storer.Store(ctx, sessionID, chunks)
}

func copyMetadata(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
