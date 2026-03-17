package chainforge

import (
	"log/slog"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/middleware/fallback"
	"github.com/lioarce01/chainforge/pkg/middleware/logging"
	cfotel "github.com/lioarce01/chainforge/pkg/middleware/otel"
	"github.com/lioarce01/chainforge/pkg/middleware/ratelimit"
	"github.com/lioarce01/chainforge/pkg/middleware/retry"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/lioarce01/chainforge/pkg/middleware/metrics"
)

// ProviderBuilder composes middleware around a base core.Provider in an explicit,
// ordered manner. It is an alternative to the WithLogging / WithRetry / WithTracing
// agent options when you need fine-grained control over wrapper ordering or want to
// share a pre-built provider across multiple agents.
//
//	p := chainforge.NewProviderBuilder(anthropic.New(apiKey)).
//	    WithRetry(3).
//	    WithLogging(logger).
//	    Build()
//
//	agent, _ := chainforge.NewAgent(chainforge.WithProvider(p), chainforge.WithModel("claude-sonnet-4-6"))
type ProviderBuilder struct {
	base     core.Provider
	wrappers []func(core.Provider) core.Provider
}

// NewProviderBuilder creates a builder starting from base.
func NewProviderBuilder(base core.Provider) *ProviderBuilder {
	return &ProviderBuilder{base: base}
}

// WithRetry adds exponential-backoff retry middleware.
// maxAttempts is the total number of attempts (1 = no retry).
func (b *ProviderBuilder) WithRetry(maxAttempts int) *ProviderBuilder {
	b.wrappers = append(b.wrappers, func(p core.Provider) core.Provider {
		return retry.New(p, maxAttempts)
	})
	return b
}

// WithLogging adds slog-based request/response logging middleware.
// If logger is nil, slog.Default() is used.
func (b *ProviderBuilder) WithLogging(logger *slog.Logger) *ProviderBuilder {
	b.wrappers = append(b.wrappers, func(p core.Provider) core.Provider {
		return logging.NewLoggedProvider(p, logger)
	})
	return b
}

// WithTracing adds OpenTelemetry span tracing middleware.
// If InitTracerProvider has not been called, the global noop tracer is used — no error.
func (b *ProviderBuilder) WithTracing() *ProviderBuilder {
	b.wrappers = append(b.wrappers, func(p core.Provider) core.Provider {
		return cfotel.NewTracedProvider(p, cfotel.Tracer())
	})
	return b
}

// WithFallback adds a fallback chain: if the current provider fails, each fallback
// is tried in order until one succeeds.
func (b *ProviderBuilder) WithFallback(fallbacks ...core.Provider) *ProviderBuilder {
	b.wrappers = append(b.wrappers, func(p core.Provider) core.Provider {
		return fallback.New(p, fallbacks...)
	})
	return b
}

// WithRateLimit adds token-bucket rate limiting middleware.
// rps is the sustained request rate; burst is the maximum burst size.
func (b *ProviderBuilder) WithRateLimit(rps float64, burst int) *ProviderBuilder {
	b.wrappers = append(b.wrappers, func(p core.Provider) core.Provider {
		return ratelimit.New(p, rps, burst)
	})
	return b
}

// WithMetrics adds Prometheus metrics middleware. Registers metrics on reg.
// Panics if registration fails (use New directly for error handling).
func (b *ProviderBuilder) WithMetrics(reg prometheus.Registerer) *ProviderBuilder {
	b.wrappers = append(b.wrappers, func(p core.Provider) core.Provider {
		return metrics.MustNew(p, reg)
	})
	return b
}

// Build applies all registered wrappers in call order and returns the composed Provider.
// Build is deterministic and may be called multiple times; each call applies the same
// wrappers to the original base.
func (b *ProviderBuilder) Build() core.Provider {
	p := b.base
	for _, wrap := range b.wrappers {
		p = wrap(p)
	}
	return p
}
