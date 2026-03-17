// Package redis provides a MemoryStore backed by Redis Lists, with optional TTL.
package redis

import (
	"context"
	"encoding/json"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Compile-time guard: Store must satisfy core.MemoryStore.
var _ core.MemoryStore = (*Store)(nil)

// Store is a Redis-backed MemoryStore. Safe for concurrent use.
type Store struct {
	cfg    Config
	client *goredis.Client
}

// New creates a Redis-backed Store and validates the connection with a Ping.
func New(addr string, extra ...Option) (*Store, error) {
	if addr == "" {
		return nil, ErrNoAddr
	}
	opts := append([]Option{WithAddr(addr)}, extra...)
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("%w: %v", ErrConnect, err)
	}

	return &Store{cfg: cfg, client: client}, nil
}

// NewFromURL creates a Redis-backed Store from a redis:// URL.
func NewFromURL(rawURL string, extra ...Option) (*Store, error) {
	parsed, err := goredis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse URL: %w", err)
	}

	cfg := defaultConfig()
	for _, o := range extra {
		o(&cfg)
	}

	client := goredis.NewClient(parsed)
	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("%w: %v", ErrConnect, err)
	}

	// Override addr from parsed URL so config reflects reality.
	cfg.Addr = parsed.Addr
	cfg.DB = parsed.DB
	cfg.Password = parsed.Password
	return &Store{cfg: cfg, client: client}, nil
}

// Close releases the underlying Redis connection.
func (s *Store) Close() error {
	return s.client.Close()
}

// -- core.MemoryStore --------------------------------------------------------

// Get returns all messages for a session in FIFO order (oldest→newest).
func (s *Store) Get(ctx context.Context, sessionID string) ([]core.Message, error) {
	key := s.key(sessionID)
	vals, err := s.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: get session %q: %w", sessionID, err)
	}
	if len(vals) == 0 {
		return nil, nil
	}

	msgs := make([]core.Message, 0, len(vals))
	for _, v := range vals {
		var msg core.Message
		if err := json.Unmarshal([]byte(v), &msg); err != nil {
			return nil, fmt.Errorf("redis: unmarshal message_json: %w", err)
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// Append pushes one or more messages to the right of the session list.
// If TTL is configured, it is (re)set on every Append (sliding window).
func (s *Store) Append(ctx context.Context, sessionID string, msgs ...core.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	key := s.key(sessionID)
	pipe := s.client.Pipeline()

	vals := make([]interface{}, 0, len(msgs))
	for _, msg := range msgs {
		raw, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("redis: marshal message: %w", err)
		}
		vals = append(vals, string(raw))
	}
	pipe.RPush(ctx, key, vals...)
	if s.cfg.TTL > 0 {
		pipe.Expire(ctx, key, s.cfg.TTL)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis: append session %q: %w", sessionID, err)
	}
	return nil
}

// Clear deletes the session list.
func (s *Store) Clear(ctx context.Context, sessionID string) error {
	if err := s.client.Del(ctx, s.key(sessionID)).Err(); err != nil {
		return fmt.Errorf("redis: clear session %q: %w", sessionID, err)
	}
	return nil
}

// -- Helpers -----------------------------------------------------------------

func (s *Store) key(sessionID string) string {
	return fmt.Sprintf("%s:%s", s.cfg.KeyPrefix, sessionID)
}
