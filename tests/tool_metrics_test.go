package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lioarce01/chainforge/pkg/middleware/metrics"
	"github.com/lioarce01/chainforge/pkg/tools"
)

func newMetricsRegistry(t *testing.T) (*metrics.ToolRegistry, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	tr, err := metrics.NewToolRegistry(reg)
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}
	return tr, reg
}

func TestToolMetricsCountOnCall(t *testing.T) {
	tr, reg := newMetricsRegistry(t)
	inner := tools.MustFunc("my-tool", "test", nil, func(ctx context.Context, input string) (string, error) {
		return "ok", nil
	})
	wrapped := tr.Wrap(inner)

	wrapped.Call(context.Background(), "{}")
	wrapped.Call(context.Background(), "{}")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	for _, f := range families {
		if f.GetName() == "chainforge_tool_calls_total" {
			for _, m := range f.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "status" && lp.GetValue() == "ok" {
						if v := m.GetCounter().GetValue(); v != 2 {
							t.Errorf("expected 2 ok calls, got %v", v)
						}
					}
				}
			}
			return
		}
	}
	t.Error("chainforge_tool_calls_total metric not found")
}

func TestToolMetricsDuration(t *testing.T) {
	tr, reg := newMetricsRegistry(t)
	inner := tools.MustFunc("slow", "test", nil, func(ctx context.Context, input string) (string, error) {
		return "result", nil
	})
	wrapped := tr.Wrap(inner)
	wrapped.Call(context.Background(), "{}")

	families, _ := reg.Gather()
	for _, f := range families {
		if f.GetName() == "chainforge_tool_duration_seconds" {
			for _, m := range f.GetMetric() {
				if m.GetHistogram().GetSampleCount() == 1 {
					return // found, duration was observed
				}
			}
		}
	}
	t.Error("chainforge_tool_duration_seconds not observed after call")
}

func TestToolMetricsErrorLabel(t *testing.T) {
	tr, reg := newMetricsRegistry(t)
	wantErr := errors.New("boom")
	inner := tools.MustFunc("fail", "test", nil, func(ctx context.Context, input string) (string, error) {
		return "", wantErr
	})
	wrapped := tr.Wrap(inner)
	wrapped.Call(context.Background(), "{}")

	families, _ := reg.Gather()
	for _, f := range families {
		if f.GetName() == "chainforge_tool_calls_total" {
			for _, m := range f.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "status" && lp.GetValue() == "error" {
						if v := m.GetCounter().GetValue(); v == 1 {
							return // found
						}
					}
				}
			}
		}
	}
	t.Error("expected status=error counter not found")
}

func TestToolMetricsNoConflict(t *testing.T) {
	// Two separate registries must not conflict.
	reg1 := prometheus.NewRegistry()
	reg2 := prometheus.NewRegistry()

	_, err1 := metrics.NewToolRegistry(reg1)
	_, err2 := metrics.NewToolRegistry(reg2)

	if err1 != nil {
		t.Errorf("registry 1 error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("registry 2 error: %v", err2)
	}
}
