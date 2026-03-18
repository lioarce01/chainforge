package inmemory

import (
	"context"
	"sync"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Store is an in-memory MemoryStore that isolates history by session ID.
// Safe for concurrent use.
type Store struct {
	mu        sync.RWMutex
	sessions  map[string][]core.Message
	expiresAt map[string]time.Time // zero value = no TTL for that session
	cfg       config
}

// New creates a new in-memory store.
func New(opts ...Option) *Store {
	cfg := config{}
	for _, o := range opts {
		o(&cfg)
	}
	return &Store{
		sessions:  make(map[string][]core.Message),
		expiresAt: make(map[string]time.Time),
		cfg:       cfg,
	}
}

func (s *Store) Get(_ context.Context, sessionID string) ([]core.Message, error) {
	// Check expiry under write lock so we can clear atomically.
	if s.cfg.ttl > 0 {
		s.mu.Lock()
		if exp, ok := s.expiresAt[sessionID]; ok && !exp.IsZero() && time.Now().After(exp) {
			delete(s.sessions, sessionID)
			delete(s.expiresAt, sessionID)
			s.mu.Unlock()
			return nil, nil
		}
		s.mu.Unlock()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.sessions[sessionID]
	if len(msgs) == 0 {
		return nil, nil
	}
	out := make([]core.Message, len(msgs))
	copy(out, msgs)
	return out, nil
}

func (s *Store) Append(_ context.Context, sessionID string, msgs ...core.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[sessionID] = append(s.sessions[sessionID], msgs...)

	// Enforce max messages: keep only the most recent n.
	if s.cfg.maxMessages > 0 && len(s.sessions[sessionID]) > s.cfg.maxMessages {
		excess := len(s.sessions[sessionID]) - s.cfg.maxMessages
		s.sessions[sessionID] = s.sessions[sessionID][excess:]
	}

	// Reset TTL on every append (sliding window).
	if s.cfg.ttl > 0 {
		s.expiresAt[sessionID] = time.Now().Add(s.cfg.ttl)
	}

	return nil
}

func (s *Store) Clear(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	delete(s.expiresAt, sessionID)
	return nil
}
