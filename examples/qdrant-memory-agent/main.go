// qdrant-memory-agent demonstrates persistent, semantically-searchable memory
// backed by a Qdrant vector database.
//
// Run with local Qdrant + OpenAI embeddings:
//
//	docker run -p 6334:6334 qdrant/qdrant
//	OPENROUTER_API_KEY=... OPENAI_API_KEY=sk-... MODEL=... go run ./examples/qdrant-memory-agent/
//
// Or with Qdrant Cloud:
//
//	QDRANT_URL=https://xyz.cloud.qdrant.io:6334 QDRANT_API_KEY=... \
//	  OPENAI_API_KEY=sk-... OPENROUTER_API_KEY=... MODEL=... \
//	  go run ./examples/qdrant-memory-agent/
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	chainforge "github.com/lioarce01/chainforge"
	qdrantmem "github.com/lioarce01/chainforge/pkg/memory/qdrant"
	"github.com/lioarce01/chainforge/pkg/memory/qdrant/embedders"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
)

func main() {
	// --- LLM provider (OpenRouter or any OpenAI-compatible endpoint) ---
	provider := openai.NewWithBaseURL(
		os.Getenv("OPENROUTER_API_KEY"),
		"https://openrouter.ai/api/v1",
		"openrouter",
	)

	// --- Qdrant memory store ---
	qdrantURL := os.Getenv("QDRANT_URL")
	if qdrantURL == "" {
		qdrantURL = "localhost:6334"
	}

	storeOpts := []qdrantmem.Option{
		qdrantmem.WithURL(qdrantURL),
		qdrantmem.WithEmbedder(embedders.OpenAI(os.Getenv("OPENAI_API_KEY"))),
		qdrantmem.WithCollectionName("qdrant_memory_agent_demo"),
		qdrantmem.WithTopK(10),
	}
	if key := os.Getenv("QDRANT_API_KEY"); key != "" {
		storeOpts = append(storeOpts, qdrantmem.WithAPIKey(key))
	}

	store, err := qdrantmem.New(storeOpts...)
	if err != nil {
		log.Fatalf("create qdrant store: %v", err)
	}
	defer store.Close()

	// --- Agent ---
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(provider),
		chainforge.WithModel(os.Getenv("MODEL")),
		chainforge.WithSystemPrompt("You are a helpful assistant with persistent memory. Remember details the user shares."),
		chainforge.WithMemory(store),
	)
	if err != nil {
		log.Fatalf("create agent: %v", err)
	}

	const sessionID = "qdrant-demo-session"
	ctx := context.Background()

	fmt.Println("Qdrant memory agent ready. Type a message and press Enter. Ctrl+C to quit.")
	fmt.Println("Tip: messages persist across restarts — try stopping and restarting the program.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		reply, err := agent.Run(ctx, sessionID, input)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}
		fmt.Printf("Agent: %s\n\n", reply)
	}
}
