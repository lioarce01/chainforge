// Websearch agent example — demonstrates tool calling with a web search tool.
// Uses DuckDuckGo, no search API key required.
//
// Run with:
//
//	API_KEY=... BASE_URL=https://openrouter.ai/api/v1 MODEL=your/model go run ./examples/websearch-agent/
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
	"github.com/lioarce01/chainforge/pkg/tools/websearch"
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
		chainforge.WithSystemPrompt("You are a research assistant. Use the web_search tool to find information before answering. Cite your sources."),
		chainforge.WithTools(websearch.New(websearch.NewDuckDuckGo())),
		chainforge.WithMemory(inmemory.New()),
		chainforge.WithMaxIterations(5),
	)
	if err != nil {
		log.Fatalf("create agent: %v", err)
	}

	ctx := context.Background()

	questions := []string{
		"What is the Go programming language?",
		"What are its main use cases?",
	}

	for _, q := range questions {
		fmt.Printf("User: %s\n", q)
		result, err := agent.Run(ctx, "research-session", q)
		if err != nil {
			log.Fatalf("agent error: %v", err)
		}
		fmt.Printf("Agent: %s\n\n", result)
	}
}
