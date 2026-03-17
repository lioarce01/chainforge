package otel

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/lioarce01/chainforge/pkg/core"
)

// TracedMemoryStore wraps a core.MemoryStore with OpenTelemetry spans.
type TracedMemoryStore struct {
	inner  core.MemoryStore
	tracer trace.Tracer
}

// NewTracedMemoryStore wraps store with OTel tracing using tracer.
func NewTracedMemoryStore(store core.MemoryStore, tracer trace.Tracer) *TracedMemoryStore {
	return &TracedMemoryStore{inner: store, tracer: tracer}
}

// Get creates a span covering the history retrieval.
func (t *TracedMemoryStore) Get(ctx context.Context, sessionID string) ([]core.Message, error) {
	ctx, span := t.tracer.Start(ctx, "chainforge.memory.get",
		trace.WithAttributes(attribute.String("session_id", sessionID)),
	)
	defer span.End()

	msgs, err := t.inner.Get(ctx, sessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.Int("message_count", len(msgs)))
	span.SetStatus(codes.Ok, "")
	return msgs, nil
}

// Append creates a span covering the history append.
func (t *TracedMemoryStore) Append(ctx context.Context, sessionID string, msgs ...core.Message) error {
	ctx, span := t.tracer.Start(ctx, "chainforge.memory.append",
		trace.WithAttributes(
			attribute.String("session_id", sessionID),
			attribute.Int("message_count", len(msgs)),
		),
	)
	defer span.End()

	if err := t.inner.Append(ctx, sessionID, msgs...); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// Clear creates a span covering the session clear.
func (t *TracedMemoryStore) Clear(ctx context.Context, sessionID string) error {
	ctx, span := t.tracer.Start(ctx, "chainforge.memory.clear",
		trace.WithAttributes(attribute.String("session_id", sessionID)),
	)
	defer span.End()

	if err := t.inner.Clear(ctx, sessionID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
