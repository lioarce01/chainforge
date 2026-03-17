package qdrant

import (
	"github.com/lioarce01/chainforge/pkg/memory/qdrant/embedders"
)

// NewWithOpenAI creates a Qdrant Store pre-configured with an OpenAI embedder.
// url is the Qdrant server address (e.g. "localhost:6334" or "https://xyz.cloud.qdrant.io:6334").
// qdrantAPIKey is the Qdrant Cloud API key (pass "" for local instances).
// openaiAPIKey is the OpenAI API key used for text-embedding-3-small (1536 dims).
// Optional extra options (e.g. WithCollectionName, WithTopK) are applied last.
//
// Note: grpc.NewClient is lazy — construction succeeds even for invalid addresses.
// Connection errors surface on the first Get or Append call.
func NewWithOpenAI(url, qdrantAPIKey, openaiAPIKey string, extra ...Option) (*Store, error) {
	opts := []Option{
		WithURL(url),
		WithAPIKey(qdrantAPIKey),
		WithEmbedder(embedders.OpenAI(openaiAPIKey)),
	}
	opts = append(opts, extra...)
	return New(opts...)
}

// NewWithOllama creates a Qdrant Store pre-configured with an Ollama embedder.
// url is the Qdrant server address.
// ollamaBaseURL is the Ollama base URL (e.g. "http://localhost:11434").
// model is the embedding model name (e.g. "nomic-embed-text").
// dims is the embedding dimension count for the chosen model.
// Optional extra options are applied last.
func NewWithOllama(url, ollamaBaseURL, model string, dims uint64, extra ...Option) (*Store, error) {
	opts := []Option{
		WithURL(url),
		WithEmbedder(embedders.Ollama(ollamaBaseURL, model, dims)),
	}
	opts = append(opts, extra...)
	return New(opts...)
}
