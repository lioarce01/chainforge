// Package sqlite provides a persistent MemoryStore backed by a SQLite database.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	_ "modernc.org/sqlite" // register "sqlite" driver

	"github.com/lioarce01/chainforge/pkg/core"
)

// Compile-time guard: Store must satisfy core.MemoryStore.
var _ core.MemoryStore = (*Store)(nil)

// validIdentifier only allows safe characters for SQL identifiers.
var validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// Store is a SQLite-backed MemoryStore. Safe for concurrent use.
type Store struct {
	cfg      Config
	db       *sql.DB
	initOnce sync.Once
	initErr  error
}

// New creates a SQLite-backed Store at the given path.
func New(path string, extra ...Option) (*Store, error) {
	opts := append([]Option{WithPath(path)}, extra...)
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	if !validIdentifier.MatchString(cfg.TableName) {
		return nil, fmt.Errorf("sqlite: invalid table name %q", cfg.TableName)
	}
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %q: %w", cfg.Path, err)
	}
	// SQLite allows only one writer at a time.
	db.SetMaxOpenConns(1)
	return &Store{cfg: cfg, db: db}, nil
}

// NewInMemory creates a Store backed by an in-process SQLite database.
// Useful for tests and development — data is lost when the Store is closed.
func NewInMemory(extra ...Option) (*Store, error) {
	return New(":memory:", extra...)
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB. Useful for raw queries in tests.
func (s *Store) DB() *sql.DB {
	return s.db
}

// -- core.MemoryStore --------------------------------------------------------

// Get returns all messages for a session in insertion order (oldest→newest).
func (s *Store) Get(ctx context.Context, sessionID string) ([]core.Message, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	query := fmt.Sprintf(
		`SELECT message_json FROM %s WHERE session_id = ? ORDER BY id ASC`,
		s.cfg.TableName,
	)
	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get session %q: %w", sessionID, err)
	}
	defer rows.Close()

	var msgs []core.Message
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("sqlite: scan row: %w", err)
		}
		var msg core.Message
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal message_json: %w", err)
		}
		msgs = append(msgs, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: rows error: %w", err)
	}
	if len(msgs) == 0 {
		return nil, nil
	}
	return msgs, nil
}

// Append inserts one or more messages for a session in a single transaction.
func (s *Store) Append(ctx context.Context, sessionID string, msgs ...core.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	if err := s.ensureSchema(ctx); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(
		`INSERT INTO %s (session_id, message_json) VALUES (?, ?)`,
		s.cfg.TableName,
	))
	if err != nil {
		return fmt.Errorf("sqlite: prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, msg := range msgs {
		raw, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("sqlite: marshal message: %w", err)
		}
		if _, err := stmt.ExecContext(ctx, sessionID, string(raw)); err != nil {
			return fmt.Errorf("sqlite: insert message: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit: %w", err)
	}
	return nil
}

// Clear deletes all messages for a session.
func (s *Store) Clear(ctx context.Context, sessionID string) error {
	if err := s.ensureSchema(ctx); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE session_id = ?`, s.cfg.TableName),
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: clear session %q: %w", sessionID, err)
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
	ms := s.cfg.BusyTimeout.Milliseconds()
	pragmas := fmt.Sprintf(`
		PRAGMA journal_mode=WAL;
		PRAGMA busy_timeout=%d;
	`, ms)
	if _, err := s.db.ExecContext(ctx, pragmas); err != nil {
		return fmt.Errorf("%w: pragma: %v", ErrMigration, err)
	}

	ddl := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id   TEXT NOT NULL,
			message_json TEXT NOT NULL,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_%s_session
			ON %s(session_id, id);
	`, s.cfg.TableName, s.cfg.TableName, s.cfg.TableName)

	if _, err := s.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("%w: create table: %v", ErrMigration, err)
	}
	return nil
}
