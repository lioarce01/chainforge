package tests

import (
	"context"
	"fmt"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/middleware/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetrics_RequestsCounterOK(t *testing.T) {
	reg := prometheus.NewRegistry()
	mock := NewMockProvider(EndTurnResponse("ok"))
	mp, err := metrics.New(mock, reg)
	if err != nil {
		t.Fatal(err)
	}

	mp.Chat(context.Background(), core.ChatRequest{})

	counter, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	_ = counter

	// Find requests_total{status="ok"}
	families, _ := reg.Gather()
	for _, fam := range families {
		if fam.GetName() == "chainforge_provider_requests_total" {
			for _, m := range fam.GetMetric() {
				for _, lbl := range m.GetLabel() {
					if lbl.GetName() == "status" && lbl.GetValue() == "ok" {
						if got := m.GetCounter().GetValue(); got != 1 {
							t.Errorf("expected requests_total{ok}=1, got %v", got)
						}
						return
					}
				}
			}
		}
	}
	t.Error("requests_total{status=ok} metric not found")
}

func TestMetrics_RequestsCounterError(t *testing.T) {
	reg := prometheus.NewRegistry()
	mock := NewMockProvider(MockResponse{Err: fmt.Errorf("fail")})
	mp, err := metrics.New(mock, reg)
	if err != nil {
		t.Fatal(err)
	}

	mp.Chat(context.Background(), core.ChatRequest{})

	families, _ := reg.Gather()
	for _, fam := range families {
		if fam.GetName() == "chainforge_provider_requests_total" {
			for _, m := range fam.GetMetric() {
				for _, lbl := range m.GetLabel() {
					if lbl.GetName() == "status" && lbl.GetValue() == "error" {
						if got := m.GetCounter().GetValue(); got != 1 {
							t.Errorf("expected requests_total{error}=1, got %v", got)
						}
						return
					}
				}
			}
		}
	}
	t.Error("requests_total{status=error} metric not found")
}

func TestMetrics_LatencyHistogramRecorded(t *testing.T) {
	reg := prometheus.NewRegistry()
	mock := NewMockProvider(EndTurnResponse("ok"))
	mp, _ := metrics.New(mock, reg)

	mp.Chat(context.Background(), core.ChatRequest{})

	families, _ := reg.Gather()
	for _, fam := range families {
		if fam.GetName() == "chainforge_provider_request_duration_seconds" {
			for _, m := range fam.GetMetric() {
				if m.GetHistogram().GetSampleCount() == 1 {
					return
				}
			}
		}
	}
	t.Error("latency histogram sample count != 1 after one call")
}

func TestMetrics_TokensAccumulated(t *testing.T) {
	reg := prometheus.NewRegistry()
	mock := NewMockProvider(EndTurnResponse("ok")) // Usage{InputTokens:10, OutputTokens:5}
	mp, _ := metrics.New(mock, reg)

	mp.Chat(context.Background(), core.ChatRequest{})

	families, _ := reg.Gather()
	inputFound, outputFound := false, false
	for _, fam := range families {
		if fam.GetName() == "chainforge_provider_tokens_total" {
			for _, m := range fam.GetMetric() {
				for _, lbl := range m.GetLabel() {
					if lbl.GetName() == "token_type" {
						switch lbl.GetValue() {
						case "input":
							if v := m.GetCounter().GetValue(); v != 10 {
								t.Errorf("input tokens: expected 10, got %v", v)
							}
							inputFound = true
						case "output":
							if v := m.GetCounter().GetValue(); v != 5 {
								t.Errorf("output tokens: expected 5, got %v", v)
							}
							outputFound = true
						}
					}
				}
			}
		}
	}
	if !inputFound || !outputFound {
		t.Error("token counters not found in metrics")
	}
}

func TestMetrics_ChatStream_RecordsOnDone(t *testing.T) {
	reg := prometheus.NewRegistry()
	mock := NewMockProvider(EndTurnResponse("hello")) // Usage{10,5}
	mp, _ := metrics.New(mock, reg)

	ch, err := mp.ChatStream(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	// Drain stream
	for range ch {
	}

	families, _ := reg.Gather()
	for _, fam := range families {
		if fam.GetName() == "chainforge_provider_tokens_total" {
			for _, m := range fam.GetMetric() {
				for _, lbl := range m.GetLabel() {
					if lbl.GetName() == "token_type" && lbl.GetValue() == "input" {
						if v := m.GetCounter().GetValue(); v != 10 {
							t.Errorf("stream: input tokens expected 10, got %v", v)
						}
						return
					}
				}
			}
		}
	}
	t.Error("stream token metrics not recorded after channel closes")
}

func TestProviderBuilder_WithMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	mock := NewMockProvider(EndTurnResponse("ok"))
	p := chainforge.NewProviderBuilder(mock).WithMetrics(reg).Build()

	_, err := p.Chat(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}

	// Verify a metric was registered by checking gather works
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) == 0 {
		t.Error("expected metrics to be registered")
	}
	_ = testutil.ToFloat64 // ensure import is used
}
