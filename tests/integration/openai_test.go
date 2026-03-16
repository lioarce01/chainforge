//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
	"github.com/lioarce01/chainforge/pkg/tools/calculator"
)

func TestOpenAIBasic(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(openai.New(apiKey)),
		chainforge.WithModel("gpt-4o-mini"),
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

func TestOpenAIWithCalculator(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(openai.New(apiKey)),
		chainforge.WithModel("gpt-4o-mini"),
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
