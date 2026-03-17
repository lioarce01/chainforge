// Package retry provides a retry middleware for core.Provider.
// It retries transient errors (rate limits, network blips, server errors)
// with exponential backoff. Context cancellation and deadline errors are
// never retried.
package retry

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
)

const (
	defaultBaseDelay = 200 * time.Millisecond
	defaultMaxDelay  = 10 * time.Second
)

// Provider wraps a core.Provider with automatic retry on transient errors.
type Provider struct {
	inner      core.Provider
	maxAttempts int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// New wraps p with retry logic. maxAttempts is the total number of attempts
// (1 = no retry). Uses exponential backoff starting at 200ms, capped at 10s.
func New(p core.Provider, maxAttempts int) *Provider {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return &Provider{
		inner:       p,
		maxAttempts: maxAttempts,
		baseDelay:   defaultBaseDelay,
		maxDelay:    defaultMaxDelay,
	}
}

func (r *Provider) Name() string { return r.inner.Name() }

// Chat retries the underlying Chat call on transient errors.
func (r *Provider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	var lastErr error
	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		if attempt > 0 {
			if err := r.wait(ctx, attempt); err != nil {
				return core.ChatResponse{}, err
			}
		}
		resp, err := r.inner.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		if !isRetryable(err) {
			return core.ChatResponse{}, err
		}
		lastErr = err
	}
	return core.ChatResponse{}, lastErr
}

// ChatStream retries the stream open call on transient errors.
// Once the stream is open and events are flowing, errors mid-stream
// are not retried — the caller must handle StreamEventError.
func (r *Provider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	var lastErr error
	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		if attempt > 0 {
			if err := r.wait(ctx, attempt); err != nil {
				return nil, err
			}
		}
		ch, err := r.inner.ChatStream(ctx, req)
		if err == nil {
			return ch, nil
		}
		if !isRetryable(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// wait sleeps for the backoff duration or until ctx is cancelled.
func (r *Provider) wait(ctx context.Context, attempt int) error {
	delay := time.Duration(float64(r.baseDelay) * math.Pow(2, float64(attempt-1)))
	if delay > r.maxDelay {
		delay = r.maxDelay
	}
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// isRetryable returns true for errors that are worth retrying.
// Context cancellation and deadline exceeded are never retried.
func isRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}
