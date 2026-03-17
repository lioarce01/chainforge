// Package benchutil provides test helpers for benchmarks and load tests.
package benchutil

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
)

// MockProvider is a configurable provider for benchmarks.
// It simulates realistic streaming without real network I/O.
type MockProvider struct {
	// ResponseText is the full text returned per request.
	ResponseText string
	// Latency is an artificial delay before the first chunk.
	Latency time.Duration
	// ChunkSize controls how many bytes per stream chunk (0 = one chunk).
	ChunkSize int
	// InjectError makes every call return this error.
	InjectError error
	// CallCount tracks total invocations atomically.
	callCount atomic.Int64
}

// NewMockProvider returns a MockProvider with sensible defaults.
func NewMockProvider(responseText string) *MockProvider {
	return &MockProvider{
		ResponseText: responseText,
		ChunkSize:    32,
	}
}

func (m *MockProvider) Name() string { return "mock-bench" }

// CallCount returns the total number of Chat/ChatStream calls made.
func (m *MockProvider) CallCount() int64 { return m.callCount.Load() }

// Reset zeroes the call counter.
func (m *MockProvider) Reset() { m.callCount.Store(0) }

func (m *MockProvider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	m.callCount.Add(1)
	if m.InjectError != nil {
		return core.ChatResponse{}, m.InjectError
	}
	if m.Latency > 0 {
		select {
		case <-time.After(m.Latency):
		case <-ctx.Done():
			return core.ChatResponse{}, ctx.Err()
		}
	}
	return core.ChatResponse{
		Message:    core.Message{Role: core.RoleAssistant, Content: m.ResponseText},
		StopReason: core.StopReasonEndTurn,
		Usage: core.Usage{
			InputTokens:  countTokens(req.Messages),
			OutputTokens: len(m.ResponseText) / 4,
		},
	}, nil
}

func (m *MockProvider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	m.callCount.Add(1)
	if m.InjectError != nil {
		return nil, m.InjectError
	}
	ch := make(chan core.StreamEvent, 32)
	go func() {
		defer close(ch)
		if m.Latency > 0 {
			select {
			case <-time.After(m.Latency):
			case <-ctx.Done():
				ch <- core.StreamEvent{Type: core.StreamEventError, Error: ctx.Err()}
				return
			}
		}
		text := m.ResponseText
		size := m.ChunkSize
		if size <= 0 {
			size = len(text)
		}
		if size == 0 {
			size = 1
		}
		chunks := splitChunks(text, size)
		for _, chunk := range chunks {
			select {
			case <-ctx.Done():
				ch <- core.StreamEvent{Type: core.StreamEventError, Error: ctx.Err()}
				return
			case ch <- core.StreamEvent{Type: core.StreamEventText, TextDelta: chunk}:
			}
		}
		usage := core.Usage{
			InputTokens:  countTokens(req.Messages),
			OutputTokens: len(text) / 4,
		}
		ch <- core.StreamEvent{
			Type:       core.StreamEventDone,
			StopReason: core.StopReasonEndTurn,
			Usage:      &usage,
		}
	}()
	return ch, nil
}

// splitChunks splits s into chunks of at most size bytes.
func splitChunks(s string, size int) []string {
	if s == "" {
		return nil
	}
	var chunks []string
	for len(s) > 0 {
		n := size
		if n > len(s) {
			n = len(s)
		}
		chunks = append(chunks, s[:n])
		s = s[n:]
	}
	return chunks
}

// countTokens is a rough approximation: 1 token ≈ 4 characters.
func countTokens(msgs []core.Message) int {
	var total int
	for _, m := range msgs {
		total += len(m.Content) / 4
		for _, tc := range m.ToolCalls {
			total += len(tc.Input) / 4
		}
	}
	return total
}

// MockToolProvider extends MockProvider with configurable tool-call responses.
// First call returns a tool_use stop; second call returns end_turn.
type MockToolProvider struct {
	MockProvider
	ToolName  string
	ToolInput string
	toolCalls atomic.Int64
}

func NewMockToolProvider(toolName, toolInput, finalResponse string) *MockToolProvider {
	return &MockToolProvider{
		MockProvider: MockProvider{ResponseText: finalResponse, ChunkSize: 32},
		ToolName:     toolName,
		ToolInput:    toolInput,
	}
}

func (m *MockToolProvider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	m.callCount.Add(1)
	if m.InjectError != nil {
		return core.ChatResponse{}, m.InjectError
	}
	n := m.toolCalls.Add(1)
	if n == 1 {
		return core.ChatResponse{
			Message: core.Message{
				Role: core.RoleAssistant,
				ToolCalls: []core.ToolCall{{
					ID:    fmt.Sprintf("tool_%d", n),
					Name:  m.ToolName,
					Input: m.ToolInput,
				}},
			},
			StopReason: core.StopReasonToolUse,
			Usage:      core.Usage{InputTokens: 20, OutputTokens: 15},
		}, nil
	}
	return core.ChatResponse{
		Message:    core.Message{Role: core.RoleAssistant, Content: m.ResponseText},
		StopReason: core.StopReasonEndTurn,
		Usage:      core.Usage{InputTokens: 30, OutputTokens: 10},
	}, nil
}

func (m *MockToolProvider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	resp, err := m.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan core.StreamEvent, 4)
	go func() {
		defer close(ch)
		if resp.StopReason == core.StopReasonToolUse {
			for _, tc := range resp.Message.ToolCalls {
				tcc := tc
				ch <- core.StreamEvent{Type: core.StreamEventToolCall, ToolCall: &tcc}
			}
		} else if resp.Message.Content != "" {
			chunks := splitChunks(resp.Message.Content, 32)
			for _, c := range chunks {
				ch <- core.StreamEvent{Type: core.StreamEventText, TextDelta: c}
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

// LargeResponseText generates a realistic response of approximately n bytes.
func LargeResponseText(n int) string {
	const sentence = "The quick brown fox jumps over the lazy dog. "
	return strings.Repeat(sentence, n/len(sentence)+1)[:n]
}
