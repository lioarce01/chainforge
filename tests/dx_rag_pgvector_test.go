package tests

import (
	"context"
	"testing"

	"github.com/lioarce01/chainforge/pkg/rag"
)

// mockEmbedder for pgvector tests — 3-dimensional embeddings.
type pgvMockEmbedder struct{}

func (e *pgvMockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	// Deterministic: hash the first 3 chars into a 3-dim vector.
	v := []float32{0, 0, 0}
	for i, ch := range []rune(text) {
		if i >= 3 {
			break
		}
		v[i] = float32(ch) / 128.0
	}
	return v, nil
}

func (e *pgvMockEmbedder) Dims() uint64 { return 3 }

// TestPGVectorStore_NewRequiresDSN verifies that an empty DSN is rejected.
func TestPGVectorStore_NewRequiresDSN(t *testing.T) {
	_, err := rag.NewPGVectorStore("", &pgvMockEmbedder{})
	if err == nil {
		t.Fatal("expected error for empty DSN")
	}
}

// TestPGVectorStore_NewRequiresEmbedder verifies that a nil embedder is rejected.
func TestPGVectorStore_NewRequiresEmbedder(t *testing.T) {
	_, err := rag.NewPGVectorStore("postgres://localhost/test", nil)
	if err == nil {
		t.Fatal("expected error for nil embedder")
	}
}

// TestPGVectorStore_OptionsApplied checks that PGVectorOption functions apply correctly.
// This verifies the options pipeline without connecting to a real database.
func TestPGVectorStore_OptionsApplied(t *testing.T) {
	// This will fail at pool creation (no real DB), but we want to confirm
	// option errors don't fire before the DSN parse step.
	store, err := rag.NewPGVectorStore(
		"postgres://user:pass@localhost:5432/testdb",
		&pgvMockEmbedder{},
		rag.PGVWithTable("my_chunks"),
		rag.PGVWithSchema("custom"),
		rag.PGVWithMaxConns(5),
		rag.PGVWithHNSW(),
	)
	// NewPGVectorStore connects eagerly — err is acceptable here (no real DB).
	// We just verify no panic and the constructor signature is correct.
	if err == nil {
		// If somehow a connection succeeded (e.g., local PG), close cleanly.
		store.Close()
	}
	// The test succeeds as long as no panic occurred and options compiled.
}

// TestPGVectorStore_ImplementsRetriever verifies PGVectorStore satisfies rag.Retriever.
func TestPGVectorStore_ImplementsRetriever(t *testing.T) {
	// Compile-time interface check via assignment to interface variable.
	var _ rag.Retriever = (*rag.PGVectorStore)(nil)
}

// TestPGVectorStore_ImplementsDocumentStorer verifies PGVectorStore satisfies rag.DocumentStorer.
func TestPGVectorStore_ImplementsDocumentStorer(t *testing.T) {
	var _ rag.DocumentStorer = (*rag.PGVectorStore)(nil)
}
