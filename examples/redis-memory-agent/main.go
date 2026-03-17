// redis-memory-agent demonstrates conversation history stored in Redis with an
// optional sliding-window TTL. Each restart reconnects to the same session.
//
// Run:
//
//	REDIS_URL=redis://localhost:6379 \
//	  OPENROUTER_API_KEY=... MODEL=... \
//	  go run ./examples/redis-memory-agent/
//
// Set SESSION_TTL_HOURS to expire history automatically (e.g. SESSION_TTL_HOURS=24).
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/memory/redis"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
)

func main() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	provider := openai.NewWithBaseURL(
		os.Getenv("OPENROUTER_API_KEY"),
		"https://openrouter.ai/api/v1",
		"openrouter",
	)

	opts := []redis.Option{}
	if h := os.Getenv("SESSION_TTL_HOURS"); h != "" {
		hours, err := strconv.Atoi(h)
		if err != nil {
			log.Fatalf("invalid SESSION_TTL_HOURS: %v", err)
		}
		opts = append(opts, redis.WithTTL(time.Duration(hours)*time.Hour))
	}

	store, err := redis.NewFromURL(redisURL, opts...)
	if err != nil {
		log.Fatalf("create redis store: %v", err)
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

	const sessionID = "redis-demo-session"
	ctx := context.Background()

	fmt.Println("Redis memory agent ready. Type a message and press Enter. Ctrl+C to quit.")
	if os.Getenv("SESSION_TTL_HOURS") != "" {
		fmt.Printf("TTL: %s hours (sliding window — resets on each message).\n", os.Getenv("SESSION_TTL_HOURS"))
	}
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
