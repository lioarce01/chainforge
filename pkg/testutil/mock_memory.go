package testutil

import (
	"context"
	"sync"

	"github.com/lioarce01/chainforge/pkg/core"
)

// MapMemoryStore is an in-memory MemoryStore backed by a plain map.
// It is safe for concurrent use and records all operations for inspection in tests.
//
//	mem := testutil.NewMapMemory()
//	agent, _ := chainforge.NewAgent(
//	    chainforge.WithMemory(mem),
//	    ...
//	)
type MapMemoryStore struct {
	mu       sync.RWMutex
	sessions map[string][]core.Message
	appends  int
	clears   int
}

// NewMapMemory creates an empty MapMemoryStore.
func NewMapMemory() *MapMemoryStore {
	return &MapMemoryStore{sessions: make(map[string][]core.Message)}
}

func (m *MapMemoryStore) Get(_ context.Context, sessionID string) ([]core.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs := m.sessions[sessionID]
	if len(msgs) == 0 {
		return nil, nil
	}
	out := make([]core.Message, len(msgs))
	copy(out, msgs)
	return out, nil
}

func (m *MapMemoryStore) Append(_ context.Context, sessionID string, msgs ...core.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionID] = append(m.sessions[sessionID], msgs...)
	m.appends++
	return nil
}

func (m *MapMemoryStore) Clear(_ context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
	m.clears++
	return nil
}

// AppendCount returns how many total Append calls have been made.
func (m *MapMemoryStore) AppendCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.appends
}

// ClearCount returns how many total Clear calls have been made.
func (m *MapMemoryStore) ClearCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clears
}

// SessionIDs returns all session IDs that currently have messages.
func (m *MapMemoryStore) SessionIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}
