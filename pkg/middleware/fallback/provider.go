// Package fallback provides a provider that tries multiple providers in order,
// returning the first successful response.
package fallback

import (
	"context"
	"strings"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Compile-time guard
var _ core.Provider = (*Provider)(nil)

// Provider tries providers[0] first; on error falls through to providers[1], etc.
type Provider struct {
	providers []core.Provider
}

// New creates a fallback chain: primary is tried first, then each fallback in order.
func New(primary core.Provider, fallbacks ...core.Provider) *Provider {
	all := make([]core.Provider, 0, 1+len(fallbacks))
	all = append(all, primary)
	all = append(all, fallbacks...)
	return &Provider{providers: all}
}

// Name returns all provider names joined by "/".
func (f *Provider) Name() string {
	names := make([]string, len(f.providers))
	for i, p := range f.providers {
		names[i] = p.Name()
	}
	return strings.Join(names, "/")
}

// Chat tries each provider in order and returns the first success.
// If all fail, the last error is returned.
func (f *Provider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	var lastErr error
	for _, p := range f.providers {
		resp, err := p.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return core.ChatResponse{}, lastErr
}

// ChatStream tries each provider in order for stream-open; returns the first success.
// Mid-stream errors are not retried — the caller must handle StreamEventError.
func (f *Provider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	var lastErr error
	for _, p := range f.providers {
		ch, err := p.ChatStream(ctx, req)
		if err == nil {
			return ch, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
