package chainforge

import (
	"context"

	"github.com/lioarce01/chainforge/pkg/core"
)

// WithSessionID returns a copy of ctx with the given session ID attached.
// The value can be retrieved with SessionIDFromContext or core.SessionIDFromContext.
// The agent loop sets this automatically before calling the provider so that
// middleware (WithTracing, WithLogging) can correlate traces to sessions.
func WithSessionID(ctx context.Context, id string) context.Context {
	return core.WithSessionID(ctx, id)
}

// SessionIDFromContext returns the session ID stored in ctx, or "" if none.
func SessionIDFromContext(ctx context.Context) string {
	return core.SessionIDFromContext(ctx)
}
