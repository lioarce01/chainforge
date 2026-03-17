// Package logging provides slog-based observability wrappers for core interfaces.
// It has zero external dependencies beyond the Go standard library.
package logging

import (
	"context"
	"log/slog"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
)

// LoggedProvider wraps a core.Provider and emits structured log events for
// every Chat and ChatStream call.
type LoggedProvider struct {
	inner  core.Provider
	logger *slog.Logger
}

// NewLoggedProvider wraps p with slog-based logging using logger.
// If logger is nil, slog.Default() is used.
func NewLoggedProvider(p core.Provider, logger *slog.Logger) *LoggedProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggedProvider{inner: p, logger: logger}
}

// Name delegates to the wrapped provider.
func (l *LoggedProvider) Name() string { return l.inner.Name() }

// Chat logs before/after the underlying call and records latency.
func (l *LoggedProvider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	start := time.Now()
	l.logger.DebugContext(ctx, "provider.Chat: start",
		slog.String("provider", l.inner.Name()),
		slog.String("model", req.Model),
		slog.Int("messages", len(req.Messages)),
	)

	resp, err := l.inner.Chat(ctx, req)
	dur := time.Since(start)

	if err != nil {
		l.logger.ErrorContext(ctx, "provider.Chat: error",
			slog.String("provider", l.inner.Name()),
			slog.String("model", req.Model),
			slog.Duration("duration", dur),
			slog.String("error", err.Error()),
		)
		return resp, err
	}

	l.logger.InfoContext(ctx, "provider.Chat: done",
		slog.String("provider", l.inner.Name()),
		slog.String("model", req.Model),
		slog.Duration("duration", dur),
		slog.String("stop_reason", string(resp.StopReason)),
		slog.Int("input_tokens", resp.Usage.InputTokens),
		slog.Int("output_tokens", resp.Usage.OutputTokens),
	)
	return resp, nil
}

// ChatStream logs stream start and wraps the event channel to log completion.
func (l *LoggedProvider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	start := time.Now()
	l.logger.DebugContext(ctx, "provider.ChatStream: start",
		slog.String("provider", l.inner.Name()),
		slog.String("model", req.Model),
		slog.Int("messages", len(req.Messages)),
	)

	inner, err := l.inner.ChatStream(ctx, req)
	if err != nil {
		l.logger.ErrorContext(ctx, "provider.ChatStream: error",
			slog.String("provider", l.inner.Name()),
			slog.String("model", req.Model),
			slog.Duration("duration", time.Since(start)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	out := make(chan core.StreamEvent, cap(inner))
	go func() {
		defer close(out)
		var (
			totalText  int
			stopReason core.StopReason
		)
		for ev := range inner {
			switch ev.Type {
			case core.StreamEventText:
				totalText += len(ev.TextDelta)
			case core.StreamEventDone:
				stopReason = ev.StopReason
			case core.StreamEventError:
				l.logger.ErrorContext(ctx, "provider.ChatStream: stream error",
					slog.String("provider", l.inner.Name()),
					slog.Duration("duration", time.Since(start)),
					slog.String("error", ev.Error.Error()),
				)
			}
			out <- ev
		}
		l.logger.InfoContext(ctx, "provider.ChatStream: done",
			slog.String("provider", l.inner.Name()),
			slog.String("model", req.Model),
			slog.Duration("duration", time.Since(start)),
			slog.String("stop_reason", string(stopReason)),
			slog.Int("text_bytes", totalText),
		)
	}()
	return out, nil
}
