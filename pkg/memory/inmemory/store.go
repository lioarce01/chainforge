package inmemory

import (
	"context"
	"sync"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Store is an in-memory MemoryStore that isolates history by session ID.
// Safe for concurrent use.
type Store struct {
	mu       sync.RWMutex
	sessions map[string][]core.Message
}

// New creates a new in-memory store.
func New() *Store {
	return &Store{sessions: make(map[string][]core.Message)}
}

func (s *Store) Get(_ context.Context, sessionID string) ([]core.Message, error) {
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
	return nil
}

func (s *Store) Clear(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}
