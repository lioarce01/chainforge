// Package ratelimit provides a token-bucket rate limiting middleware for core.Provider.
package ratelimit

import (
	"context"

	"github.com/lioarce01/chainforge/pkg/core"
	"golang.org/x/time/rate"
)

// Compile-time guard
var _ core.Provider = (*Provider)(nil)

// Provider wraps a core.Provider with a token-bucket rate limiter.
// Calls block until a token is available or the context is cancelled.
type Provider struct {
	inner   core.Provider
	limiter *rate.Limiter
}

// New creates a rate-limited provider. rps is the sustained request rate
// (requests per second); burst is the maximum burst size.
func New(p core.Provider, rps float64, burst int) *Provider {
	return &Provider{
		inner:   p,
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}
}

func (r *Provider) Name() string { return r.inner.Name() }

// Chat waits for a rate-limit token before forwarding the call.
func (r *Provider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return core.ChatResponse{}, err
	}
	return r.inner.Chat(ctx, req)
}

// ChatStream waits for a rate-limit token before opening the stream.
func (r *Provider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return r.inner.ChatStream(ctx, req)
}
