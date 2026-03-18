// Package testutil provides testing helpers for code built on top of chainforge.
// Import it in your _test.go files to get a scriptable provider, a fake memory
// store, and assertion helpers — without copying internal test utilities.
package testutil

import (
	"context"
	"fmt"
	"sync"

	"github.com/lioarce01/chainforge/pkg/core"
)

// MockResponse is a scripted response for MockProvider.
type MockResponse struct {
	Response core.ChatResponse
	Err      error
}

// Call records a single captured provider call.
type Call struct {
	Request core.ChatRequest
}

// MockProvider is a deterministic, scriptable provider for unit tests.
// Responses are returned in order; once exhausted the last response repeats.
//
//	p := testutil.NewMockProvider(
//	    testutil.EndTurnResponse("Hello!"),
//	    testutil.ToolUseResponse(core.ToolCall{Name: "search", Input: `{"query":"go"}`}),
//	    testutil.EndTurnResponse("Done."),
//	)
type MockProvider struct {
	mu        sync.Mutex
	responses []MockResponse
	idx       int
	calls     []Call
}

// NewMockProvider creates a MockProvider with the given scripted responses.
func NewMockProvider(responses ...MockResponse) *MockProvider {
	return &MockProvider{responses: responses}
}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, Call{Request: req})

	if len(m.responses) == 0 {
		return core.ChatResponse{}, fmt.Errorf("mock: no responses scripted")
	}

	i := m.idx
	if i >= len(m.responses) {
		i = len(m.responses) - 1
	} else {
		m.idx++
	}

	r := m.responses[i]
	return r.Response, r.Err
}

// ChatStream delegates to Chat and emits text + tool-call events, mirroring
// what a real streaming provider would produce.
func (m *MockProvider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	resp, err := m.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan core.StreamEvent, 2+len(resp.Message.ToolCalls))
	go func() {
		defer close(ch)
		if resp.Message.Content != "" {
			ch <- core.StreamEvent{Type: core.StreamEventText, TextDelta: resp.Message.Content}
		}
		for i := range resp.Message.ToolCalls {
			ch <- core.StreamEvent{
				Type:     core.StreamEventToolCall,
				ToolCall: &resp.Message.ToolCalls[i],
			}
		}
		ch <- core.StreamEvent{
			Type:       core.StreamEventDone,
			StopReason: resp.StopReason,
			Usage:      &resp.Usage,
		}
	}()
	return ch, nil
}

// Calls returns a copy of all recorded provider calls.
func (m *MockProvider) Calls() []Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Call, len(m.calls))
	copy(out, m.calls)
	return out
}

// CallCount returns how many times Chat was called.
func (m *MockProvider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// Reset clears call history and resets the response index.
func (m *MockProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
	m.idx = 0
}

// LastRequest returns the most recent ChatRequest, or panics if no calls were made.
func (m *MockProvider) LastRequest() core.ChatRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		panic("testutil: MockProvider.LastRequest: no calls recorded")
	}
	return m.calls[len(m.calls)-1].Request
}

// EndTurnResponse builds a scripted text response that ends the agent loop.
func EndTurnResponse(text string) MockResponse {
	return MockResponse{
		Response: core.ChatResponse{
			Message:    core.Message{Role: core.RoleAssistant, Content: text},
			StopReason: core.StopReasonEndTurn,
			Usage:      core.Usage{InputTokens: 10, OutputTokens: 5},
		},
	}
}

// ToolUseResponse builds a scripted response that triggers one or more tool calls.
func ToolUseResponse(toolCalls ...core.ToolCall) MockResponse {
	return MockResponse{
		Response: core.ChatResponse{
			Message: core.Message{
				Role:      core.RoleAssistant,
				ToolCalls: toolCalls,
			},
			StopReason: core.StopReasonToolUse,
			Usage:      core.Usage{InputTokens: 20, OutputTokens: 10},
		},
	}
}

// ErrorResponse builds a scripted provider error.
func ErrorResponse(err error) MockResponse {
	return MockResponse{Err: err}
}
