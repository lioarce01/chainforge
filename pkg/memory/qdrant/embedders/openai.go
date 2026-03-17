package embedders

import (
	"context"
	"fmt"

	gogpt "github.com/sashabaranov/go-openai"
)

const defaultOpenAIDims = uint64(1536)

// OpenAIEmbedder embeds text using the OpenAI embeddings API.
type OpenAIEmbedder struct {
	client *gogpt.Client
	model  gogpt.EmbeddingModel
	dims   uint64
}

// OpenAI creates an embedder using text-embedding-3-small (1536 dims).
func OpenAI(apiKey string) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		client: gogpt.NewClient(apiKey),
		model:  gogpt.SmallEmbedding3,
		dims:   defaultOpenAIDims,
	}
}

// OpenAIWithModel creates an embedder with a custom model and dimension count.
func OpenAIWithModel(apiKey string, model gogpt.EmbeddingModel, dims uint64) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		client: gogpt.NewClient(apiKey),
		model:  model,
		dims:   dims,
	}
}

// OpenAIWithBaseURL creates an embedder against a custom base URL (Azure, proxies).
func OpenAIWithBaseURL(apiKey, baseURL string, model gogpt.EmbeddingModel, dims uint64) *OpenAIEmbedder {
	cfg := gogpt.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	return &OpenAIEmbedder{
		client: gogpt.NewClientWithConfig(cfg),
		model:  model,
		dims:   dims,
	}
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := e.client.CreateEmbeddings(ctx, gogpt.EmbeddingRequest{
		Input: []string{text},
		Model: gogpt.EmbeddingModel(e.model),
	})
	if err != nil {
		return nil, fmt.Errorf("openai embedder: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("openai embedder: empty response")
	}
	return resp.Data[0].Embedding, nil
}

func (e *OpenAIEmbedder) Dims() uint64 { return e.dims }
