// Single agent example.
//
// Run with:
//
//	API_KEY=... BASE_URL=https://openrouter.ai/api/v1 MODEL=your/model go run ./examples/single-agent/
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
)

func main() {
	provider := openai.NewWithBaseURL(
		os.Getenv("API_KEY"),
		os.Getenv("BASE_URL"),
		"openai-compatible",
	)

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(provider),
		chainforge.WithModel(os.Getenv("MODEL")),
		chainforge.WithSystemPrompt("You are a helpful assistant."),
		chainforge.WithMemory(inmemory.New()),
	)
	if err != nil {
		log.Fatalf("create agent: %v", err)
	}

	ctx := context.Background()

	messages := []string{
		"What is the capital of France?",
		"And what is its population?",
	}

	for _, msg := range messages {
		fmt.Printf("User: %s\n", msg)
		result, err := agent.Run(ctx, "session-1", msg)
		if err != nil {
			log.Fatalf("agent error: %v", err)
		}
		fmt.Printf("Agent: %s\n\n", result)
	}
}
