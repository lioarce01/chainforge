package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Compile-time guard
var _ core.Tool = (*MetricsTool)(nil)

// MetricsTool wraps a core.Tool and records per-tool Prometheus metrics.
type MetricsTool struct {
	inner    core.Tool
	calls    *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// ToolRegistry holds shared metric vectors for wrapping multiple tools on a
// single prometheus.Registerer without duplicate registration errors.
type ToolRegistry struct {
	calls    *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewToolRegistry registers the tool metric families on reg and returns a
// ToolRegistry that can wrap individual tools via Wrap.
// Returns an error if metric registration fails (e.g. duplicate registration).
func NewToolRegistry(reg prometheus.Registerer) (*ToolRegistry, error) {
	calls := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chainforge_tool_calls_total",
		Help: "Total tool calls, partitioned by tool name and status (ok|error).",
	}, []string{"tool", "status"})

	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "chainforge_tool_duration_seconds",
		Help:    "Tool call latency in seconds.",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"tool"})

	for _, c := range []prometheus.Collector{calls, duration} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}

	return &ToolRegistry{calls: calls, duration: duration}, nil
}

// Wrap returns a MetricsTool that records metrics using the shared registry vectors.
func (r *ToolRegistry) Wrap(t core.Tool) *MetricsTool {
	return &MetricsTool{
		inner:    t,
		calls:    r.calls,
		duration: r.duration,
	}
}

// NewMetricsTool wraps a single tool with dedicated metric vectors registered on reg.
// For wrapping many tools prefer NewToolRegistry to share the vectors.
func NewMetricsTool(t core.Tool, reg prometheus.Registerer) (*MetricsTool, error) {
	reg2, err := NewToolRegistry(reg)
	if err != nil {
		return nil, err
	}
	return reg2.Wrap(t), nil
}

// Definition delegates to the wrapped tool.
func (m *MetricsTool) Definition() core.ToolDefinition {
	return m.inner.Definition()
}

// Call records duration and call count (with ok/error status) around the inner call.
func (m *MetricsTool) Call(ctx context.Context, input string) (string, error) {
	name := m.inner.Definition().Name
	start := time.Now()

	result, err := m.inner.Call(ctx, input)

	m.duration.WithLabelValues(name).Observe(time.Since(start).Seconds())
	if err != nil {
		m.calls.WithLabelValues(name, "error").Inc()
	} else {
		m.calls.WithLabelValues(name, "ok").Inc()
	}

	return result, err
}
