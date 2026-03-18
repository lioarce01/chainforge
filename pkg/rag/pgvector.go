package rag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
	pgvectorpgx "github.com/pgvector/pgvector-go/pgx"

	"github.com/lioarce01/chainforge/pkg/core"
)

// PGVectorStore is a PostgreSQL+pgvector backed RAG store.
// It implements both Retriever and DocumentStorer, so a single instance
// plugs into both rag.NewIngestor and chainforge.WithRetriever.
//
//	store, _ := rag.NewPGVectorStore(dsn, embedder)
//	ingestor  := rag.NewIngestor(store)
//	agent, _  := chainforge.NewAgent(chainforge.WithRetriever(store, rag.WithTopK(5)))
type PGVectorStore struct {
	cfg      pgvConfig
	embedder core.Embedder
	pool     *pgxpool.Pool
	initOnce sync.Once
	initErr  error
}

// pgvConfig holds configuration for PGVectorStore.
type pgvConfig struct {
	dsn        string
	table      string
	schema     string
	maxConns   int32
	createHNSW bool // create HNSW index (better recall, slower build)
}

func defaultPGVConfig() pgvConfig {
	return pgvConfig{
		table:    "chainforge_rag_chunks",
		schema:   "public",
		maxConns: 10,
	}
}

// PGVectorOption configures a PGVectorStore.
type PGVectorOption func(*pgvConfig)

// PGVWithTable overrides the chunk table name (default: "chainforge_rag_chunks").
func PGVWithTable(name string) PGVectorOption {
	return func(c *pgvConfig) { c.table = name }
}

// PGVWithSchema overrides the schema (default: "public").
func PGVWithSchema(name string) PGVectorOption {
	return func(c *pgvConfig) { c.schema = name }
}

// PGVWithMaxConns sets the pool maximum connection count (default: 10).
func PGVWithMaxConns(n int32) PGVectorOption {
	return func(c *pgvConfig) { c.maxConns = n }
}

// PGVWithHNSW adds an HNSW index on the embedding column for faster ANN search.
// Recommended for collections > 100k rows. Adds ~10–30s to first-migration time.
func PGVWithHNSW() PGVectorOption {
	return func(c *pgvConfig) { c.createHNSW = true }
}

// NewPGVectorStore creates and connects a PGVectorStore.
// The schema is applied lazily on the first Store or Retrieve call.
//
//	store, err := rag.NewPGVectorStore(
//	    "postgres://user:pass@localhost:5432/mydb",
//	    embedder,
//	    rag.PGVWithTable("kb_chunks"),
//	)
func NewPGVectorStore(dsn string, embedder core.Embedder, opts ...PGVectorOption) (*PGVectorStore, error) {
	if dsn == "" {
		return nil, errors.New("pgvector: DSN is required")
	}
	if embedder == nil {
		return nil, errors.New("pgvector: embedder is required")
	}

	cfg := defaultPGVConfig()
	cfg.dsn = dsn
	for _, o := range opts {
		o(&cfg)
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgvector: parse DSN: %w", err)
	}
	poolCfg.MaxConns = cfg.maxConns
	// Register pgvector types on every new connection.
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgvectorpgx.RegisterTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("pgvector: connect: %w", err)
	}

	return &PGVectorStore{cfg: cfg, embedder: embedder, pool: pool}, nil
}

// Close releases the connection pool.
func (s *PGVectorStore) Close() error {
	s.pool.Close()
	return nil
}

// -- Retriever ---------------------------------------------------------------

// Retrieve performs cosine-similarity ANN search against sessionID's chunks.
// Implements rag.Retriever.
func (s *PGVectorStore) Retrieve(ctx context.Context, query string, topK int) ([]Document, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}

	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("pgvector: embed query: %w", err)
	}

	table := pgx.Identifier{s.cfg.schema, s.cfg.table}.Sanitize()
	sql := fmt.Sprintf(`
		SELECT id, content, source, metadata
		FROM %s
		ORDER BY embedding <=> $1
		LIMIT $2`, table)

	rows, err := s.pool.Query(ctx, sql, pgvector.NewVector(vec), topK)
	if err != nil {
		return nil, fmt.Errorf("pgvector: retrieve: %w", err)
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var (
			id       string
			content  string
			source   string
			metaRaw  []byte
		)
		if err := rows.Scan(&id, &content, &source, &metaRaw); err != nil {
			return nil, fmt.Errorf("pgvector: scan row: %w", err)
		}
		doc := Document{ID: id, Content: content, Source: source}
		if len(metaRaw) > 0 {
			json.Unmarshal(metaRaw, &doc.Metadata) //nolint:errcheck
		}
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgvector: rows error: %w", err)
	}
	return docs, nil
}

// RetrieveBySession performs cosine-similarity search scoped to a single sessionID.
// Use this when you have multiple knowledge bases in the same table.
func (s *PGVectorStore) RetrieveBySession(ctx context.Context, sessionID, query string, topK int) ([]Document, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}

	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("pgvector: embed query: %w", err)
	}

	table := pgx.Identifier{s.cfg.schema, s.cfg.table}.Sanitize()
	sql := fmt.Sprintf(`
		SELECT id, content, source, metadata
		FROM %s
		WHERE session_id = $1
		ORDER BY embedding <=> $2
		LIMIT $3`, table)

	rows, err := s.pool.Query(ctx, sql, sessionID, pgvector.NewVector(vec), topK)
	if err != nil {
		return nil, fmt.Errorf("pgvector: retrieve by session: %w", err)
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var (
			id      string
			content string
			source  string
			metaRaw []byte
		)
		if err := rows.Scan(&id, &content, &source, &metaRaw); err != nil {
			return nil, fmt.Errorf("pgvector: scan row: %w", err)
		}
		doc := Document{ID: id, Content: content, Source: source}
		if len(metaRaw) > 0 {
			json.Unmarshal(metaRaw, &doc.Metadata) //nolint:errcheck
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// -- DocumentStorer ----------------------------------------------------------

// Store upserts document chunks with their embeddings.
// Implements rag.DocumentStorer for use with rag.NewIngestor.
func (s *PGVectorStore) Store(ctx context.Context, sessionID string, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}
	if err := s.ensureSchema(ctx); err != nil {
		return err
	}

	table := pgx.Identifier{s.cfg.schema, s.cfg.table}.Sanitize()
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (id, session_id, content, source, metadata, embedding)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			content   = EXCLUDED.content,
			source    = EXCLUDED.source,
			metadata  = EXCLUDED.metadata,
			embedding = EXCLUDED.embedding`, table)

	batch := &pgx.Batch{}
	for _, doc := range docs {
		vec, err := s.embedder.Embed(ctx, doc.Content)
		if err != nil {
			return fmt.Errorf("pgvector: embed %q: %w", doc.ID, err)
		}

		var metaJSON []byte
		if len(doc.Metadata) > 0 {
			metaJSON, _ = json.Marshal(doc.Metadata)
		}

		batch.Queue(insertSQL, doc.ID, sessionID, doc.Content, doc.Source, metaJSON, pgvector.NewVector(vec))
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range docs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("pgvector: batch upsert: %w", err)
		}
	}
	return br.Close()
}

// Clear deletes all chunks for a sessionID.
func (s *PGVectorStore) Clear(ctx context.Context, sessionID string) error {
	if err := s.ensureSchema(ctx); err != nil {
		return err
	}
	table := pgx.Identifier{s.cfg.schema, s.cfg.table}.Sanitize()
	_, err := s.pool.Exec(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE session_id = $1`, table),
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("pgvector: clear session %q: %w", sessionID, err)
	}
	return nil
}

// -- Schema migration --------------------------------------------------------

func (s *PGVectorStore) ensureSchema(ctx context.Context) error {
	s.initOnce.Do(func() {
		s.initErr = s.migrate(ctx)
	})
	return s.initErr
}

func (s *PGVectorStore) migrate(ctx context.Context) error {
	dims := s.embedder.Dims()
	table := pgx.Identifier{s.cfg.schema, s.cfg.table}.Sanitize()

	// Sanitized index names (no schema prefix allowed in CREATE INDEX IF NOT EXISTS).
	sessionIdxName := pgx.Identifier{fmt.Sprintf("idx_%s_%s_session", s.cfg.schema, s.cfg.table)}.Sanitize()

	ddl := fmt.Sprintf(`
		CREATE EXTENSION IF NOT EXISTS vector;

		CREATE TABLE IF NOT EXISTS %s (
			id         TEXT         PRIMARY KEY,
			session_id TEXT         NOT NULL,
			content    TEXT         NOT NULL,
			source     TEXT         NOT NULL DEFAULT '',
			metadata   JSONB,
			embedding  vector(%d),
			created_at TIMESTAMPTZ  DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS %s ON %s(session_id);
	`, table, dims, sessionIdxName, table)

	if _, err := s.pool.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("pgvector: migrate: %w", err)
	}

	if s.cfg.createHNSW {
		hnswIdxName := pgx.Identifier{fmt.Sprintf("idx_%s_%s_hnsw", s.cfg.schema, s.cfg.table)}.Sanitize()
		hnswDDL := fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s USING hnsw (embedding vector_cosine_ops);
		`, hnswIdxName, table)
		if _, err := s.pool.Exec(ctx, hnswDDL); err != nil {
			return fmt.Errorf("pgvector: migrate hnsw index: %w", err)
		}
	}

	return nil
}
