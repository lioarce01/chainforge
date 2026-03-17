// Package metrics provides a Prometheus metrics middleware for core.Provider.
package metrics

import (
	"context"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/prometheus/client_golang/prometheus"
)

// Compile-time guard
var _ core.Provider = (*Provider)(nil)

// Provider wraps a core.Provider and records Prometheus metrics for every call.
type Provider struct {
	inner    core.Provider
	requests *prometheus.CounterVec
	latency  *prometheus.HistogramVec
	tokens   *prometheus.CounterVec
}

// New registers Prometheus metrics on reg and returns a Provider wrapping p.
// Returns an error if metric registration fails (e.g. duplicate registration).
func New(p core.Provider, reg prometheus.Registerer) (*Provider, error) {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chainforge_provider_requests_total",
		Help: "Total provider requests, partitioned by provider name and status (ok|error).",
	}, []string{"provider", "status"})

	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "chainforge_provider_request_duration_seconds",
		Help:    "Provider request latency in seconds.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"provider"})

	tokens := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chainforge_provider_tokens_total",
		Help: "Total tokens consumed, partitioned by provider name and token type (input|output).",
	}, []string{"provider", "token_type"})

	for _, c := range []prometheus.Collector{requests, latency, tokens} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}

	return &Provider{
		inner:    p,
		requests: requests,
		latency:  latency,
		tokens:   tokens,
	}, nil
}

// MustNew is like New but panics on registration error.
func MustNew(p core.Provider, reg prometheus.Registerer) *Provider {
	m, err := New(p, reg)
	if err != nil {
		panic(err)
	}
	return m
}

func (m *Provider) Name() string { return m.inner.Name() }

// Chat records request count, latency, and token usage.
func (m *Provider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	start := time.Now()
	resp, err := m.inner.Chat(ctx, req)
	dur := time.Since(start)

	m.latency.WithLabelValues(m.inner.Name()).Observe(dur.Seconds())

	if err != nil {
		m.requests.WithLabelValues(m.inner.Name(), "error").Inc()
		return resp, err
	}

	m.requests.WithLabelValues(m.inner.Name(), "ok").Inc()
	m.tokens.WithLabelValues(m.inner.Name(), "input").Add(float64(resp.Usage.InputTokens))
	m.tokens.WithLabelValues(m.inner.Name(), "output").Add(float64(resp.Usage.OutputTokens))
	return resp, nil
}

// ChatStream records request count, latency, and token usage after the stream closes.
// Latency covers the full stream duration (open → last event).
func (m *Provider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	start := time.Now()
	inner, err := m.inner.ChatStream(ctx, req)
	if err != nil {
		m.requests.WithLabelValues(m.inner.Name(), "error").Inc()
		return nil, err
	}

	out := make(chan core.StreamEvent, max(cap(inner), 16))
	go func() {
		defer close(out)
		var totalUsage core.Usage
		for ev := range inner {
			if ev.Type == core.StreamEventDone && ev.Usage != nil {
				totalUsage = *ev.Usage
			}
			out <- ev
		}
		dur := time.Since(start)
		m.latency.WithLabelValues(m.inner.Name()).Observe(dur.Seconds())
		m.requests.WithLabelValues(m.inner.Name(), "ok").Inc()
		m.tokens.WithLabelValues(m.inner.Name(), "input").Add(float64(totalUsage.InputTokens))
		m.tokens.WithLabelValues(m.inner.Name(), "output").Add(float64(totalUsage.OutputTokens))
	}()

	return out, nil
}
