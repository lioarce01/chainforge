package qdrant

import "errors"

var (
	// ErrNoEmbedder is returned by New when no embedder is configured.
	ErrNoEmbedder = errors.New("qdrant: embedder is required")

	// ErrEmptyText is returned when an empty string would be embedded.
	ErrEmptyText = errors.New("qdrant: cannot embed empty text")

	// ErrCollectionInit is returned when collection creation fails.
	ErrCollectionInit = errors.New("qdrant: collection initialisation failed")
)
