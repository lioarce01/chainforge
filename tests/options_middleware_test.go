package tests

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
)

// capturingHandler is a slog.Handler that records all log records.
type capturingHandler struct {
	buf *bytes.Buffer
}

func newCapturingHandler() (*capturingHandler, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return &capturingHandler{buf: buf}, buf
}

func (h *capturingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.buf.WriteString(r.Message + "\n")
	return nil
}
func (h *capturingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(_ string) slog.Handler      { return h }

func TestWithLogging_ProducesLogOutput(t *testing.T) {
	handler, buf := newCapturingHandler()
	logger := slog.New(handler)

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("hello"))),
		chainforge.WithModel("mock"),
		chainforge.WithLogging(logger),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	defer agent.Close()

	if _, err := agent.Run(context.Background(), "s1", "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("expected log output from WithLogging, got none")
	}
}

func TestWithLogging_NilLogger(t *testing.T) {
	// nil logger falls back to slog.Default() — should not panic.
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("ok"))),
		chainforge.WithModel("mock"),
		chainforge.WithLogging(nil),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	defer agent.Close()

	if _, err := agent.Run(context.Background(), "s1", "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestWithTracing_NoopWhenNotInitialized(t *testing.T) {
	// WithTracing() with no OTel provider configured uses the global noop tracer.
	// Must not panic or error.
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("traced"))),
		chainforge.WithModel("mock"),
		chainforge.WithTracing(),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	defer agent.Close()

	result, err := agent.Run(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "traced" {
		t.Fatalf("want traced, got %q", result)
	}
}

func TestWithTracing_BeforeProvider(t *testing.T) {
	// WithTracing listed before WithProvider — deferred wrapping must handle this.
	agent, err := chainforge.NewAgent(
		chainforge.WithTracing(),
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("ok"))),
		chainforge.WithModel("mock"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	defer agent.Close()

	if _, err := agent.Run(context.Background(), "s1", "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestWithLogging_Then_WithTracing(t *testing.T) {
	// Both options stacked — logging wraps first, then tracing wraps logging.
	handler, buf := newCapturingHandler()
	logger := slog.New(handler)

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("stacked"))),
		chainforge.WithModel("mock"),
		chainforge.WithLogging(logger),
		chainforge.WithTracing(),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	defer agent.Close()

	result, err := agent.Run(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "stacked" {
		t.Fatalf("want stacked, got %q", result)
	}
	if buf.Len() == 0 {
		t.Fatal("expected log output when both WithLogging and WithTracing used")
	}
}

func TestWithLogging_ProviderStillCalled(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("logged"))
	handler, _ := newCapturingHandler()

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(mock),
		chainforge.WithModel("mock"),
		chainforge.WithLogging(slog.New(handler)),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	defer agent.Close()

	if _, err := agent.Run(context.Background(), "s1", "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if mock.CallCount() != 1 {
		t.Fatalf("want provider called once, got %d", mock.CallCount())
	}
}

func TestWithTracing_Stream(t *testing.T) {
	// WithTracing must not break streaming.
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("stream ok"))),
		chainforge.WithModel("mock"),
		chainforge.WithTracing(),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	defer agent.Close()

	ch := agent.RunStream(context.Background(), "s1", "hi")
	var got string
	for ev := range ch {
		got += ev.TextDelta
	}
	if got != "stream ok" {
		t.Fatalf("want stream ok, got %q", got)
	}
}
