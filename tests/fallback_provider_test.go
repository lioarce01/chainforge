package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/middleware/fallback"
)

func TestFallback_PrimarySucceeds(t *testing.T) {
	primary := NewMockProvider(EndTurnResponse("primary"))
	secondary := NewMockProvider(EndTurnResponse("secondary"))

	fp := fallback.New(primary, secondary)
	resp, err := fp.Chat(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Message.Content != "primary" {
		t.Errorf("expected 'primary', got %q", resp.Message.Content)
	}
	if secondary.CallCount() != 0 {
		t.Errorf("fallback should not be called when primary succeeds")
	}
}

func TestFallback_PrimaryFails_UsedFallback(t *testing.T) {
	primary := NewMockProvider(MockResponse{Err: fmt.Errorf("primary down")})
	secondary := NewMockProvider(EndTurnResponse("fallback response"))

	fp := fallback.New(primary, secondary)
	resp, err := fp.Chat(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Message.Content != "fallback response" {
		t.Errorf("expected 'fallback response', got %q", resp.Message.Content)
	}
}

func TestFallback_AllFail_ReturnsLastError(t *testing.T) {
	err1 := fmt.Errorf("error 1")
	err2 := fmt.Errorf("error 2")
	p1 := NewMockProvider(MockResponse{Err: err1})
	p2 := NewMockProvider(MockResponse{Err: err2})

	fp := fallback.New(p1, p2)
	_, err := fp.Chat(context.Background(), core.ChatRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, err2) {
		t.Errorf("expected last error %v, got %v", err2, err)
	}
}

func TestFallback_ChatStream_FallsBack(t *testing.T) {
	// Primary returns error on stream open
	primary := &errOnStreamProvider{err: fmt.Errorf("stream open error")}
	secondary := NewMockProvider(EndTurnResponse("streamed"))

	fp := fallback.New(primary, secondary)
	ch, err := fp.ChatStream(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for ev := range ch {
		if ev.Type == core.StreamEventText {
			got += ev.TextDelta
		}
	}
	if got != "streamed" {
		t.Errorf("expected 'streamed', got %q", got)
	}
}

func TestFallback_NameConcatenated(t *testing.T) {
	p1 := NewMockProvider(EndTurnResponse(""))
	p2 := NewMockProvider(EndTurnResponse(""))
	fp := fallback.New(p1, p2)
	if fp.Name() != "mock/mock" {
		t.Errorf("expected 'mock/mock', got %q", fp.Name())
	}
}

func TestProviderBuilder_WithFallback(t *testing.T) {
	primary := NewMockProvider(MockResponse{Err: fmt.Errorf("fail")})
	fb := NewMockProvider(EndTurnResponse("from fallback"))

	p := chainforge.NewProviderBuilder(primary).WithFallback(fb).Build()
	resp, err := p.Chat(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Message.Content != "from fallback" {
		t.Errorf("expected 'from fallback', got %q", resp.Message.Content)
	}
}

// errOnStreamProvider always errors on ChatStream open.
type errOnStreamProvider struct{ err error }

func (e *errOnStreamProvider) Name() string { return "err-stream" }
func (e *errOnStreamProvider) Chat(_ context.Context, _ core.ChatRequest) (core.ChatResponse, error) {
	return core.ChatResponse{}, e.err
}
func (e *errOnStreamProvider) ChatStream(_ context.Context, _ core.ChatRequest) (<-chan core.StreamEvent, error) {
	return nil, e.err
}
