package otel

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/lioarce01/chainforge/pkg/core"
)

// TracedProvider wraps a core.Provider with OpenTelemetry spans.
// It accepts a trace.Tracer so callers can inject a noop tracer in tests.
type TracedProvider struct {
	inner  core.Provider
	tracer trace.Tracer
}

// NewTracedProvider wraps p with OTel tracing using tracer.
func NewTracedProvider(p core.Provider, tracer trace.Tracer) *TracedProvider {
	return &TracedProvider{inner: p, tracer: tracer}
}

// Name delegates to the wrapped provider.
func (t *TracedProvider) Name() string { return t.inner.Name() }

// Chat creates a span that covers the full synchronous call.
func (t *TracedProvider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	ctx, span := t.tracer.Start(ctx, "chainforge.provider.chat",
		trace.WithAttributes(
			attribute.String("provider", t.inner.Name()),
			attribute.String("model", req.Model),
			attribute.Int("messages", len(req.Messages)),
		),
	)
	defer span.End()

	resp, err := t.inner.Chat(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return resp, err
	}

	span.SetAttributes(
		attribute.String("stop_reason", string(resp.StopReason)),
		attribute.Int("input_tokens", resp.Usage.InputTokens),
		attribute.Int("output_tokens", resp.Usage.OutputTokens),
	)
	span.SetStatus(codes.Ok, "")
	return resp, nil
}

// ChatStream creates a span that covers the full stream duration.
// The span is ended by an interceptor goroutine after all events are drained,
// capturing the true end-to-end streaming latency.
func (t *TracedProvider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	ctx, span := t.tracer.Start(ctx, "chainforge.provider.chat_stream",
		trace.WithAttributes(
			attribute.String("provider", t.inner.Name()),
			attribute.String("model", req.Model),
			attribute.Int("messages", len(req.Messages)),
		),
	)

	inner, err := t.inner.ChatStream(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return nil, err
	}

	out := make(chan core.StreamEvent, cap(inner))
	go func() {
		defer span.End()
		defer close(out)

		var (
			stopReason   core.StopReason
			inputTokens  int
			outputTokens int
		)

		for ev := range inner {
			switch ev.Type {
			case core.StreamEventDone:
				stopReason = ev.StopReason
				if ev.Usage != nil {
					inputTokens = ev.Usage.InputTokens
					outputTokens = ev.Usage.OutputTokens
				}
			case core.StreamEventError:
				if ev.Error != nil {
					span.RecordError(ev.Error)
					span.SetStatus(codes.Error, ev.Error.Error())
				}
			}
			out <- ev
		}

		span.SetAttributes(
			attribute.String("stop_reason", string(stopReason)),
			attribute.Int("input_tokens", inputTokens),
			attribute.Int("output_tokens", outputTokens),
		)
		if stopReason != "" && stopReason != core.StopReasonEndTurn {
			// Non-error but notable stop reason.
			span.SetStatus(codes.Ok, string(stopReason))
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}()

	return out, nil
}
