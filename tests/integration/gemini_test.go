//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/providers/gemini"
)

func geminiKey(t *testing.T) string {
	t.Helper()
	k := os.Getenv("GEMINI_API_KEY")
	if k == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	return k
}

func TestGemini_Chat_Roundtrip(t *testing.T) {
	p, err := gemini.NewFlash(geminiKey(t))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := p.Chat(context.Background(), core.ChatRequest{
		Model: "gemini-2.0-flash",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Say 'hello' in exactly one word."},
		},
		Options: core.ChatOptions{MaxTokens: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Message.Content == "" {
		t.Error("expected non-empty response")
	}
}

func TestGemini_ChatStream_DrainEvents(t *testing.T) {
	p, err := gemini.NewFlash(geminiKey(t))
	if err != nil {
		t.Fatal(err)
	}

	ch, err := p.ChatStream(context.Background(), core.ChatRequest{
		Model: "gemini-2.0-flash",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Count to 3."},
		},
		Options: core.ChatOptions{MaxTokens: 50},
	})
	if err != nil {
		t.Fatal(err)
	}

	var gotDone bool
	for ev := range ch {
		if ev.Type == core.StreamEventError {
			t.Fatalf("stream error: %v", ev.Error)
		}
		if ev.Type == core.StreamEventDone {
			gotDone = true
		}
	}
	if !gotDone {
		t.Error("expected Done event in stream")
	}
}

func TestGemini_WithGemini_AgentOption(t *testing.T) {
	agent, err := chainforge.NewAgent(
		chainforge.WithGemini(geminiKey(t), "gemini-2.0-flash"),
		chainforge.WithMaxTokens(50),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	result, err := agent.Run(context.Background(), "sess-gemini", "Say 'hi'.")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}
