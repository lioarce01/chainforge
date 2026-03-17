// postgres-memory-agent demonstrates persistent conversation history stored in
// a PostgreSQL database. The schema is auto-migrated on first use.
//
// Run:
//
//	POSTGRES_DSN="postgres://user:pass@localhost:5432/mydb" \
//	  OPENROUTER_API_KEY=... MODEL=... \
//	  go run ./examples/postgres-memory-agent/
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/memory/postgres"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
)

func main() {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("POSTGRES_DSN is required")
	}

	provider := openai.NewWithBaseURL(
		os.Getenv("OPENROUTER_API_KEY"),
		"https://openrouter.ai/api/v1",
		"openrouter",
	)

	store, err := postgres.New(dsn)
	if err != nil {
		log.Fatalf("create postgres store: %v", err)
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

	const sessionID = "postgres-demo-session"
	ctx := context.Background()

	fmt.Println("PostgreSQL memory agent ready. Type a message and press Enter. Ctrl+C to quit.")
	fmt.Println("Tip: history persists in Postgres — stop and restart to verify.")
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
