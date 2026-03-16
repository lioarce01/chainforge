//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/providers/anthropic"
	"github.com/lioarce01/chainforge/pkg/tools/calculator"
)

func TestAnthropicBasic(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(anthropic.New(apiKey)),
		chainforge.WithModel("claude-haiku-4-5-20251001"),
		chainforge.WithSystemPrompt("You are a helpful assistant. Be concise."),
		chainforge.WithMaxIterations(3),
	)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), "test-session", "Say 'hello world' and nothing else.")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	t.Logf("Response: %s", result)
}

func TestAnthropicWithCalculator(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(anthropic.New(apiKey)),
		chainforge.WithModel("claude-haiku-4-5-20251001"),
		chainforge.WithSystemPrompt("You are a math assistant. Always use the calculator tool."),
		chainforge.WithTools(calculator.New()),
		chainforge.WithMaxIterations(5),
	)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), "calc-session", "What is 2^10?")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	t.Logf("Response: %s", result)
}
