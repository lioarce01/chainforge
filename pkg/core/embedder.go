package core

import "context"

// Embedder converts text to a dense vector representation.
// Implementations are provided in pkg/memory/qdrant/embedders.
type Embedder interface {
	// Embed returns the embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)
	// Dims returns the number of dimensions in the output vector.
	Dims() uint64
}
