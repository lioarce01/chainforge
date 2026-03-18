package rag

import (
	"context"
	"fmt"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/qdrant"
)

// QdrantRetriever implements Retriever using a qdrant.Store for semantic search.
type QdrantRetriever struct {
	store     *qdrant.Store
	embedder  core.Embedder
	sessionID string
}

// NewQdrantRetriever creates a Retriever that searches a Qdrant collection.
// sessionID is the namespace used when documents were ingested (e.g. "kb").
func NewQdrantRetriever(store *qdrant.Store, embedder core.Embedder, sessionID string) *QdrantRetriever {
	return &QdrantRetriever{
		store:     store,
		embedder:  embedder,
		sessionID: sessionID,
	}
}

// Retrieve searches the Qdrant store for documents similar to query.
func (r *QdrantRetriever) Retrieve(ctx context.Context, query string, topK int) ([]Document, error) {
	msgs, err := r.store.Search(ctx, r.sessionID, query, uint64(topK))
	if err != nil {
		return nil, fmt.Errorf("qdrant retriever: %w", err)
	}
	docs := make([]Document, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Content == "" {
			continue
		}
		docs = append(docs, Document{
			ID:      msg.ToolCallID, // used as document ID when stored
			Content: msg.Content,
			Source:  msg.Name,
		})
	}
	return docs, nil
}

// QdrantIngestor implements DocumentStorer using a qdrant.Store.
type QdrantIngestor struct {
	store    *qdrant.Store
	embedder core.Embedder
}

// NewQdrantIngestor creates an ingestor that stores document chunks in Qdrant.
func NewQdrantIngestor(store *qdrant.Store, embedder core.Embedder) *QdrantIngestor {
	return &QdrantIngestor{store: store, embedder: embedder}
}

// Store persists chunks in Qdrant under sessionID.
// Each chunk is stored as a core.Message with Content = chunk text and Name = chunk source.
func (qi *QdrantIngestor) Store(ctx context.Context, sessionID string, docs []Document) error {
	msgs := make([]core.Message, 0, len(docs))
	for _, d := range docs {
		msgs = append(msgs, core.Message{
			Role:       core.RoleUser,
			Content:    d.Content,
			Name:       d.Source,
			ToolCallID: d.ID,
		})
	}
	return qi.store.Append(ctx, sessionID, msgs...)
}
