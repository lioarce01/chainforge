// sqlite-memory-agent demonstrates persistent conversation history stored in a
// local SQLite file. No infrastructure required — pure Go, zero cgo.
//
// Run:
//
//	OPENROUTER_API_KEY=... MODEL=... go run ./examples/sqlite-memory-agent/
//
// Messages persist across restarts in ./chat.db. Delete the file to reset.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/memory/sqlite"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
)

func main() {
	provider := openai.NewWithBaseURL(
		os.Getenv("OPENROUTER_API_KEY"),
		"https://openrouter.ai/api/v1",
		"openrouter",
	)

	store, err := sqlite.New("./chat.db")
	if err != nil {
		log.Fatalf("create sqlite store: %v", err)
	}
	defer store.Close()

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(provider),
		chainforge.WithModel(os.Getenv("MODEL")),
		chainforge.WithSystemPrompt("You are a helpful assistant with persistent memory."),
		chainforge.WithMemory(store),
	)
	if err != nil {
		log.Fatalf("create agent: %v", err)
	}

	const sessionID = "sqlite-demo-session"
	ctx := context.Background()

	fmt.Println("SQLite memory agent ready. Type a message and press Enter. Ctrl+C to quit.")
	fmt.Println("Tip: history persists in ./chat.db — stop and restart to verify.")
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
