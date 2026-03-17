package tests

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
)

func TestProviderBuilder_NoWrappers(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("hi"))
	p := chainforge.NewProviderBuilder(mock).Build()
	if p == nil {
		t.Error("expected non-nil provider")
	}
	// Without any wrappers Build returns the base provider.
	resp, err := p.Chat(context.Background(), core.ChatRequest{Model: "mock"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Message.Content != "hi" {
		t.Errorf("Content = %q, want %q", resp.Message.Content, "hi")
	}
}

func TestProviderBuilder_WithRetry(t *testing.T) {
	// Provider that fails once then succeeds.
	mock := NewMockProvider(
		MockResponse{Err: core.ErrProviderError},
		EndTurnResponse("retried"),
	)
	p := chainforge.NewProviderBuilder(mock).WithRetry(2).Build()
	resp, err := p.Chat(context.Background(), core.ChatRequest{Model: "mock"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Message.Content != "retried" {
		t.Errorf("Content = %q, want %q", resp.Message.Content, "retried")
	}
}

func TestProviderBuilder_WithLogging(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mock := NewMockProvider(EndTurnResponse("logged"))
	p := chainforge.NewProviderBuilder(mock).WithLogging(logger).Build()
	_, err := p.Chat(context.Background(), core.ChatRequest{Model: "mock"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected log output after Chat call")
	}
}

func TestProviderBuilder_WithTracing_NoopWithoutTracer(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("traced"))
	p := chainforge.NewProviderBuilder(mock).WithTracing().Build()
	resp, err := p.Chat(context.Background(), core.ChatRequest{Model: "mock"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Message.Content != "traced" {
		t.Errorf("Content = %q, want %q", resp.Message.Content, "traced")
	}
}

func TestProviderBuilder_ChainedRetryAndLogging(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mock := NewMockProvider(
		MockResponse{Err: core.ErrProviderError},
		EndTurnResponse("ok"),
	)
	p := chainforge.NewProviderBuilder(mock).WithRetry(2).WithLogging(logger).Build()
	resp, err := p.Chat(context.Background(), core.ChatRequest{Model: "mock"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Message.Content != "ok" {
		t.Errorf("Content = %q, want %q", resp.Message.Content, "ok")
	}
	if buf.Len() == 0 {
		t.Error("expected log output")
	}
}

func TestProviderBuilder_BuildDeterministic(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("a"), EndTurnResponse("b"))
	b := chainforge.NewProviderBuilder(mock).WithRetry(1)
	p1 := b.Build()
	p2 := b.Build()
	if p1 == nil || p2 == nil {
		t.Error("expected non-nil providers")
	}
}

func TestProviderBuilder_NilLogger_NoPanic(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("ok"))
	p := chainforge.NewProviderBuilder(mock).WithLogging(nil).Build()
	_, err := p.Chat(context.Background(), core.ChatRequest{Model: "mock"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
}

func TestProviderBuilder_WithAgent(t *testing.T) {
	mock := NewMockProvider(EndTurnResponse("from builder"))
	p := chainforge.NewProviderBuilder(mock).WithRetry(1).Build()
	a, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	result, err := a.Run(context.Background(), "sess", "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "from builder" {
		t.Errorf("result = %q, want %q", result, "from builder")
	}
}
