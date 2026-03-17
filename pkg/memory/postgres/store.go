// Package postgres provides a persistent MemoryStore backed by PostgreSQL.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Compile-time guard: Store must satisfy core.MemoryStore.
var _ core.MemoryStore = (*Store)(nil)

// Store is a PostgreSQL-backed MemoryStore. Safe for concurrent use.
type Store struct {
	cfg      Config
	pool     *pgxpool.Pool
	initOnce sync.Once
	initErr  error
}

// New creates a PostgreSQL-backed Store using the given DSN.
func New(dsn string, extra ...Option) (*Store, error) {
	if dsn == "" {
		return nil, ErrNoDSN
	}
	opts := append([]Option{WithDSN(dsn)}, extra...)
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("%w: parse DSN: %v", ErrConnect, err)
	}
	poolCfg.MaxConns = cfg.MaxConns

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnect, err)
	}

	return &Store{cfg: cfg, pool: pool}, nil
}

// Close releases the underlying connection pool.
func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

// -- core.MemoryStore --------------------------------------------------------

// Get returns all messages for a session in insertion order (oldest→newest).
func (s *Store) Get(ctx context.Context, sessionID string) ([]core.Message, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	table := pgx.Identifier{s.cfg.SchemaName, s.cfg.TableName}.Sanitize()
	query := fmt.Sprintf(
		`SELECT message_json FROM %s WHERE session_id = $1 ORDER BY id ASC`,
		table,
	)
	rows, err := s.pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("postgres: get session %q: %w", sessionID, err)
	}
	defer rows.Close()

	var msgs []core.Message
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("postgres: scan row: %w", err)
		}
		var msg core.Message
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			return nil, fmt.Errorf("postgres: unmarshal message_json: %w", err)
		}
		msgs = append(msgs, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: rows error: %w", err)
	}
	if len(msgs) == 0 {
		return nil, nil
	}
	return msgs, nil
}

// Append inserts one or more messages for a session using a pgx Batch.
func (s *Store) Append(ctx context.Context, sessionID string, msgs ...core.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	if err := s.ensureSchema(ctx); err != nil {
		return err
	}

	table := pgx.Identifier{s.cfg.SchemaName, s.cfg.TableName}.Sanitize()
	query := fmt.Sprintf(
		`INSERT INTO %s (session_id, message_json) VALUES ($1, $2)`,
		table,
	)

	batch := &pgx.Batch{}
	for _, msg := range msgs {
		raw, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("postgres: marshal message: %w", err)
		}
		batch.Queue(query, sessionID, string(raw))
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range msgs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("postgres: batch insert: %w", err)
		}
	}
	return br.Close()
}

// Clear deletes all messages for a session.
func (s *Store) Clear(ctx context.Context, sessionID string) error {
	if err := s.ensureSchema(ctx); err != nil {
		return err
	}
	table := pgx.Identifier{s.cfg.SchemaName, s.cfg.TableName}.Sanitize()
	_, err := s.pool.Exec(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE session_id = $1`, table),
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("postgres: clear session %q: %w", sessionID, err)
	}
	return nil
}

// -- Schema init -------------------------------------------------------------

func (s *Store) ensureSchema(ctx context.Context) error {
	s.initOnce.Do(func() {
		s.initErr = s.migrate(ctx)
	})
	return s.initErr
}

func (s *Store) migrate(ctx context.Context) error {
	table := pgx.Identifier{s.cfg.SchemaName, s.cfg.TableName}.Sanitize()
	// Index name cannot be schema-qualified in CREATE INDEX IF NOT EXISTS.
	idxName := fmt.Sprintf("idx_%s_%s_session", s.cfg.SchemaName, s.cfg.TableName)

	ddl := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id           BIGSERIAL PRIMARY KEY,
			session_id   TEXT        NOT NULL,
			message_json TEXT        NOT NULL,
			created_at   TIMESTAMPTZ DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS %s
			ON %s(session_id, id);
	`, table, pgx.Identifier{idxName}.Sanitize(), table)

	if _, err := s.pool.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("%w: %v", ErrMigration, err)
	}
	return nil
}
