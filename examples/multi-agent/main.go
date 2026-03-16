// Multi-agent example — demonstrates sequential and parallel orchestration.
//
// Run with:
//
//	API_KEY=... BASE_URL=https://openrouter.ai/api/v1 MODEL=your/model go run ./examples/multi-agent/
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/orchestrator"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
)

func newAgent(systemPrompt string) *chainforge.Agent {
	provider := openai.NewWithBaseURL(
		os.Getenv("API_KEY"),
		os.Getenv("BASE_URL"),
		"openai-compatible",
	)
	return chainforge.MustNewAgent(
		chainforge.WithProvider(provider),
		chainforge.WithModel(os.Getenv("MODEL")),
		chainforge.WithSystemPrompt(systemPrompt),
	)
}

func main() {
	ctx := context.Background()

	// Sequential: output of each step feeds into the next
	fmt.Println("=== Sequential ===")
	result, err := orchestrator.Sequential(ctx, "seq",
		"Go programming language",
		orchestrator.StepOf("research", newAgent("Summarize the given topic in 2 sentences."), "Research: {{.input}}"),
		orchestrator.StepOf("write", newAgent("Expand the summary into a short paragraph."), "Expand: {{.previous}}"),
	)
	if err != nil {
		log.Fatalf("sequential: %v", err)
	}
	fmt.Printf("%s\n\n", result)

	// Parallel: all branches run concurrently
	fmt.Println("=== Parallel ===")
	results, err := orchestrator.Parallel(ctx, "par",
		orchestrator.FanOf("pros", newAgent("List 3 advantages in bullet points."), "Go programming language"),
		orchestrator.FanOf("cons", newAgent("List 3 disadvantages in bullet points."), "Go programming language"),
	)
	if err != nil {
		log.Fatalf("parallel: %v", err)
	}
	for _, r := range results {
		if r.Error != nil {
			fmt.Printf("[%s] error: %v\n", r.Name, r.Error)
			continue
		}
		fmt.Printf("[%s]\n%s\n\n", r.Name, r.Output)
	}
}
