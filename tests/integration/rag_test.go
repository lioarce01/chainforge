//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/memory/qdrant"
	"github.com/lioarce01/chainforge/pkg/memory/qdrant/embedders"
	"github.com/lioarce01/chainforge/pkg/rag"
	"github.com/lioarce01/chainforge/pkg/rag/splitter"
)

func qdrantURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("QDRANT_URL")
	if u == "" {
		t.Skip("QDRANT_URL not set")
	}
	return u
}

func openAIKey(t *testing.T) string {
	t.Helper()
	k := os.Getenv("OPENAI_API_KEY")
	if k == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	return k
}

func TestOpenRouter_RAG_IngestAndRetrieve(t *testing.T) {
	url := qdrantURL(t)
	key := openAIKey(t)

	embedder := embedders.OpenAI(key)
	store, err := qdrant.New(
		qdrant.WithURL(url),
		qdrant.WithCollectionName("rag_integration_test"),
		qdrant.WithEmbedder(embedder),
	)
	if err != nil {
		t.Fatalf("qdrant.New: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Clear any previous test data.
	store.Clear(ctx, "integration-kb")

	// Ingest documents.
	qIngestor := rag.NewQdrantIngestor(store, embedder)
	ingestor := rag.NewIngestor(qIngestor)

	docs := []rag.Document{
		{ID: "doc1", Content: "The Eiffel Tower is located in Paris, France.", Source: "geo.txt"},
		{ID: "doc2", Content: "Go is a statically typed compiled language.", Source: "go.txt"},
		{ID: "doc3", Content: "The Great Wall of China stretches 21,196 km.", Source: "geo.txt"},
	}

	if err := ingestor.Ingest(ctx, "integration-kb", docs,
		rag.WithChunkSize(200),
		rag.WithSplitter(splitter.NewFixedSizeSplitter(200, 0)),
	); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// Retrieve.
	retriever := rag.NewQdrantRetriever(store, embedder, "integration-kb")
	results, err := retriever.Retrieve(ctx, "where is the Eiffel Tower?", 2)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}

	found := false
	for _, d := range results {
		if strings.Contains(d.Content, "Paris") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected result about Paris, got: %v", results)
	}
}

func TestOpenRouter_RAG_WithRetrieverOption(t *testing.T) {
	url := qdrantURL(t)
	key := openAIKey(t)
	orKey := openRouterKey(t)

	embedder := embedders.OpenAI(key)
	store, err := qdrant.New(
		qdrant.WithURL(url),
		qdrant.WithCollectionName("rag_integration_test2"),
		qdrant.WithEmbedder(embedder),
	)
	if err != nil {
		t.Fatalf("qdrant.New: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	store.Clear(ctx, "kb2")

	// Ingest a fact.
	qIngestor := rag.NewQdrantIngestor(store, embedder)
	ingestor := rag.NewIngestor(qIngestor)
	ingestor.Ingest(ctx, "kb2", []rag.Document{
		{ID: "fact1", Content: "The speed of light is approximately 299,792 km/s.", Source: "physics.txt"},
	})

	retriever := rag.NewQdrantRetriever(store, embedder, "kb2")
	agent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("Answer questions using only the provided context."),
		chainforge.WithRetriever(retriever, rag.WithTopK(3)),
		chainforge.WithModel(openRouterModel),
	)

	result, err := agent.Run(ctx, "rag-session", "What is the speed of light?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(result, "299") {
		t.Errorf("expected answer to mention 299,792, got: %q", result)
	}
	_ = orKey
}

func TestOpenRouter_RAG_RetrieverTool(t *testing.T) {
	url := qdrantURL(t)
	key := openAIKey(t)

	embedder := embedders.OpenAI(key)
	store, err := qdrant.New(
		qdrant.WithURL(url),
		qdrant.WithCollectionName("rag_integration_test3"),
		qdrant.WithEmbedder(embedder),
	)
	if err != nil {
		t.Fatalf("qdrant.New: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	store.Clear(ctx, "kb3")

	qIngestor := rag.NewQdrantIngestor(store, embedder)
	ingestor := rag.NewIngestor(qIngestor)
	ingestor.Ingest(ctx, "kb3", []rag.Document{
		{ID: "fact1", Content: "Mount Everest is 8,849 metres tall.", Source: "geography.txt"},
	})

	retriever := rag.NewQdrantRetriever(store, embedder, "kb3")
	tool := rag.NewRetrieverTool(retriever, rag.WithTopK(3))

	agent := newOpenRouterAgent(t,
		chainforge.WithTools(tool),
		chainforge.WithSystemPrompt("Use the retriever_tool to look up facts before answering."),
	)

	result, err := agent.Run(ctx, "rag-tool-session", "How tall is Mount Everest?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(result, "8,849") && !strings.Contains(result, "8849") {
		t.Errorf("expected answer about Everest height, got: %q", result)
	}
}
