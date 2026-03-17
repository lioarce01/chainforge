package logging

import (
	"context"
	"log/slog"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
)

// LoggedMemoryStore wraps a core.MemoryStore and emits structured log events
// for every Get, Append, and Clear call.
type LoggedMemoryStore struct {
	inner  core.MemoryStore
	logger *slog.Logger
}

// NewLoggedMemoryStore wraps store with slog-based logging using logger.
// If logger is nil, slog.Default() is used.
func NewLoggedMemoryStore(store core.MemoryStore, logger *slog.Logger) *LoggedMemoryStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggedMemoryStore{inner: store, logger: logger}
}

// Get logs before/after the retrieval and records message count.
func (l *LoggedMemoryStore) Get(ctx context.Context, sessionID string) ([]core.Message, error) {
	start := time.Now()
	msgs, err := l.inner.Get(ctx, sessionID)
	dur := time.Since(start)

	if err != nil {
		l.logger.ErrorContext(ctx, "memory.Get: error",
			slog.String("session", sessionID),
			slog.Duration("duration", dur),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	l.logger.DebugContext(ctx, "memory.Get: done",
		slog.String("session", sessionID),
		slog.Int("messages", len(msgs)),
		slog.Duration("duration", dur),
	)
	return msgs, nil
}

// Append logs the number of messages appended.
func (l *LoggedMemoryStore) Append(ctx context.Context, sessionID string, msgs ...core.Message) error {
	start := time.Now()
	err := l.inner.Append(ctx, sessionID, msgs...)
	dur := time.Since(start)

	if err != nil {
		l.logger.ErrorContext(ctx, "memory.Append: error",
			slog.String("session", sessionID),
			slog.Int("count", len(msgs)),
			slog.Duration("duration", dur),
			slog.String("error", err.Error()),
		)
		return err
	}

	l.logger.DebugContext(ctx, "memory.Append: done",
		slog.String("session", sessionID),
		slog.Int("count", len(msgs)),
		slog.Duration("duration", dur),
	)
	return nil
}

// Clear logs the clear operation.
func (l *LoggedMemoryStore) Clear(ctx context.Context, sessionID string) error {
	start := time.Now()
	err := l.inner.Clear(ctx, sessionID)
	dur := time.Since(start)

	if err != nil {
		l.logger.ErrorContext(ctx, "memory.Clear: error",
			slog.String("session", sessionID),
			slog.Duration("duration", dur),
			slog.String("error", err.Error()),
		)
		return err
	}

	l.logger.InfoContext(ctx, "memory.Clear: done",
		slog.String("session", sessionID),
		slog.Duration("duration", dur),
	)
	return nil
}
