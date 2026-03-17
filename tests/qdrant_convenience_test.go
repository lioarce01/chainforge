package tests

import (
	"testing"

	"github.com/lioarce01/chainforge/pkg/memory/qdrant"
)

func TestNewWithOpenAI_NonNilStore(t *testing.T) {
	store, err := qdrant.NewWithOpenAI("localhost:6334", "", "sk-fake")
	if err != nil {
		t.Fatalf("NewWithOpenAI: %v", err)
	}
	if store == nil {
		t.Error("expected non-nil store")
	}
}

func TestNewWithOpenAI_EmptyQdrantAPIKey(t *testing.T) {
	// Empty qdrantAPIKey is valid for local Qdrant instances.
	store, err := qdrant.NewWithOpenAI("localhost:6334", "", "sk-fake")
	if err != nil {
		t.Fatalf("NewWithOpenAI with empty qdrantAPIKey: %v", err)
	}
	if store == nil {
		t.Error("expected non-nil store")
	}
}

func TestNewWithOpenAI_ExtraOptions(t *testing.T) {
	store, err := qdrant.NewWithOpenAI(
		"localhost:6334", "", "sk-fake",
		qdrant.WithCollectionName("my_collection"),
		qdrant.WithTopK(5),
	)
	if err != nil {
		t.Fatalf("NewWithOpenAI with extra options: %v", err)
	}
	if store == nil {
		t.Error("expected non-nil store")
	}
}

func TestNewWithOllama_NonNilStore(t *testing.T) {
	store, err := qdrant.NewWithOllama("localhost:6334", "http://localhost:11434", "nomic-embed-text", 768)
	if err != nil {
		t.Fatalf("NewWithOllama: %v", err)
	}
	if store == nil {
		t.Error("expected non-nil store")
	}
}

func TestNewWithOllama_ZeroDims(t *testing.T) {
	// Zero dims is accepted at construction; errors surface on first embed call.
	store, err := qdrant.NewWithOllama("localhost:6334", "http://localhost:11434", "nomic-embed-text", 0)
	if err != nil {
		t.Fatalf("NewWithOllama with zero dims: %v", err)
	}
	if store == nil {
		t.Error("expected non-nil store")
	}
}
