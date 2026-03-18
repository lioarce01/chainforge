package testutil

import (
	"context"
	"strings"
	"testing"

	"github.com/lioarce01/chainforge/pkg/core"
)

// AssertCallCount fails the test if p was not called exactly want times.
func AssertCallCount(t testing.TB, p *MockProvider, want int) {
	t.Helper()
	if got := p.CallCount(); got != want {
		t.Errorf("MockProvider: expected %d call(s), got %d", want, got)
	}
}

// AssertLastRequestContains fails if no message in the last request has the
// given role and contains substr in its content.
func AssertLastRequestContains(t testing.TB, p *MockProvider, role core.Role, substr string) {
	t.Helper()
	req := p.LastRequest()
	for _, m := range req.Messages {
		if m.Role == role && strings.Contains(m.Content, substr) {
			return
		}
	}
	t.Errorf("MockProvider: last request has no %s message containing %q", role, substr)
}

// AssertSessionContains fails if no message in sessionID contains substr.
func AssertSessionContains(t testing.TB, mem core.MemoryStore, sessionID, substr string) {
	t.Helper()
	msgs, err := mem.Get(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("AssertSessionContains: mem.Get: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m.Content, substr) {
			return
		}
	}
	t.Errorf("session %q: no message contains %q", sessionID, substr)
}

// AssertSessionLen fails if the number of messages in sessionID is not want.
func AssertSessionLen(t testing.TB, mem core.MemoryStore, sessionID string, want int) {
	t.Helper()
	msgs, err := mem.Get(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("AssertSessionLen: mem.Get: %v", err)
	}
	if got := len(msgs); got != want {
		t.Errorf("session %q: expected %d message(s), got %d", sessionID, want, got)
	}
}
