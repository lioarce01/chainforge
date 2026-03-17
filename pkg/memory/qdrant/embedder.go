package qdrant

import "context"

// Embedder converts text to a dense vector representation.
type Embedder interface {
	// Embed returns the embedding for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)
	// Dims returns the number of dimensions in the output vector.
	Dims() uint64
}
