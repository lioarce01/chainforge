package embedders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OllamaEmbedder embeds text using the Ollama /api/embed endpoint.
// It uses the standard net/http client — no additional dependencies.
type OllamaEmbedder struct {
	baseURL string
	model   string
	dims    uint64
	client  *http.Client
}

// Ollama creates an embedder that calls Ollama's /api/embed endpoint.
// baseURL is e.g. "http://localhost:11434", model is e.g. "nomic-embed-text".
func Ollama(baseURL, model string, dims uint64) *OllamaEmbedder {
	return &OllamaEmbedder{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		dims:    dims,
		client:  &http.Client{},
	}
}

type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(ollamaEmbedRequest{Model: e.model, Input: text})
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embedder: HTTP %d: %s", resp.StatusCode, string(b))
	}

	var out ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama embedder: decode: %w", err)
	}
	if len(out.Embeddings) == 0 {
		return nil, fmt.Errorf("ollama embedder: empty embeddings in response")
	}
	return out.Embeddings[0], nil
}

func (e *OllamaEmbedder) Dims() uint64 { return e.dims }
