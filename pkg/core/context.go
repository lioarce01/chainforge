package core

import "context"

type contextKey string

const sessionCtxKey contextKey = "chainforge.session_id"

// WithSessionID returns a copy of ctx with the session ID attached.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionCtxKey, id)
}

// SessionIDFromContext returns the session ID stored in ctx, or "" if none.
func SessionIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(sessionCtxKey).(string)
	return v
}
