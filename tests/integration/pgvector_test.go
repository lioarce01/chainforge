//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/lioarce01/chainforge/pkg/memory/qdrant/embedders"
	"github.com/lioarce01/chainforge/pkg/rag"
)

// pgvectorDSN returns PGVECTOR_DSN or skips the test.
func pgvectorDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("PGVECTOR_DSN")
	if dsn == "" {
		t.Skip("PGVECTOR_DSN not set — skipping pgvector integration tests")
	}
	return dsn
}

// openAIEmbedder returns an OpenAI embedder or skips if no API key.
func openAIEmbedder(t *testing.T) *embedders.OpenAIEmbedder {
	t.Helper()
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set — skipping pgvector integration tests")
	}
	return embedders.NewOpenAIEmbedder(key, "text-embedding-3-small")
}

// TestPGVector_StoreAndRetrieve ingests two documents and retrieves the more relevant one.
func TestPGVector_StoreAndRetrieve(t *testing.T) {
	ctx := context.Background()
	dsn := pgvectorDSN(t)
	embedder := openAIEmbedder(t)

	store, err := rag.NewPGVectorStore(dsn, embedder,
		rag.PGVWithTable(fmt.Sprintf("test_chunks_%d", os.Getpid())),
	)
	if err != nil {
		t.Fatalf("NewPGVectorStore: %v", err)
	}
	defer func() {
		store.Clear(ctx, "test-session")
		store.Close()
	}()

	docs := []rag.Document{
		{ID: "doc1", Content: "Go is a statically typed compiled programming language.", Source: "go.txt"},
		{ID: "doc2", Content: "Python is a dynamically typed interpreted scripting language.", Source: "python.txt"},
	}

	if err := store.Store(ctx, "test-session", docs); err != nil {
		t.Fatalf("Store: %v", err)
	}

	results, err := store.RetrieveBySession(ctx, "test-session", "compiled statically typed language", 1)
	if err != nil {
		t.Fatalf("RetrieveBySession: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].ID != "doc1" {
		t.Errorf("expected doc1 to be most relevant, got %s", results[0].ID)
	}
}

// TestPGVector_GlobalRetrieve verifies Retrieve (no session filter) returns results across sessions.
func TestPGVector_GlobalRetrieve(t *testing.T) {
	ctx := context.Background()
	dsn := pgvectorDSN(t)
	embedder := openAIEmbedder(t)

	table := fmt.Sprintf("test_global_%d", os.Getpid())
	store, err := rag.NewPGVectorStore(dsn, embedder, rag.PGVWithTable(table))
	if err != nil {
		t.Fatalf("NewPGVectorStore: %v", err)
	}
	defer store.Close()

	_ = store.Store(ctx, "session-a", []rag.Document{
		{ID: "a1", Content: "Rust provides memory safety without garbage collection.", Source: "rust.txt"},
	})
	_ = store.Store(ctx, "session-b", []rag.Document{
		{ID: "b1", Content: "Java uses a garbage collector for memory management.", Source: "java.txt"},
	})
	defer func() {
		store.Clear(ctx, "session-a")
		store.Clear(ctx, "session-b")
	}()

	results, err := store.Retrieve(ctx, "memory safety without GC", 2)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results from global retrieve")
	}
}

// TestPGVector_UpsertDeduplication verifies that re-storing a document with the same ID updates it.
func TestPGVector_UpsertDeduplication(t *testing.T) {
	ctx := context.Background()
	dsn := pgvectorDSN(t)
	embedder := openAIEmbedder(t)

	table := fmt.Sprintf("test_upsert_%d", os.Getpid())
	store, err := rag.NewPGVectorStore(dsn, embedder, rag.PGVWithTable(table))
	if err != nil {
		t.Fatalf("NewPGVectorStore: %v", err)
	}
	defer func() {
		store.Clear(ctx, "upsert-session")
		store.Close()
	}()

	doc := rag.Document{ID: "dedup1", Content: "original content", Source: "orig.txt"}
	if err := store.Store(ctx, "upsert-session", []rag.Document{doc}); err != nil {
		t.Fatalf("first Store: %v", err)
	}

	doc.Content = "updated content"
	if err := store.Store(ctx, "upsert-session", []rag.Document{doc}); err != nil {
		t.Fatalf("second Store: %v", err)
	}

	results, err := store.RetrieveBySession(ctx, "upsert-session", "updated content", 5)
	if err != nil {
		t.Fatalf("Retrieve after upsert: %v", err)
	}
	// Should only have one record with the updated content.
	count := 0
	for _, r := range results {
		if r.ID == "dedup1" {
			count++
			if !strings.Contains(r.Content, "updated") {
				t.Errorf("expected updated content, got %q", r.Content)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected 1 record for dedup1, got %d", count)
	}
}
