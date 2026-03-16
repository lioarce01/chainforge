// mcp-agent demonstrates connecting a remote MCP server to a chainforge agent via OpenRouter.
//
// Run with:
//
//	OPENROUTER_API_KEY=... MCP_URL=https://docs.langchain.com/mcp MODEL=openrouter/hunter-alpha go run ./examples/mcp-agent/
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/mcp"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
)

const openRouterBaseURL = "https://openrouter.ai/api/v1"

func main() {
	ctx := context.Background()

	mcpURL := os.Getenv("MCP_URL")
	if mcpURL == "" {
		slog.Error("MCP_URL environment variable is required")
		os.Exit(1)
	}

	provider := openai.NewWithBaseURL(os.Getenv("OPENROUTER_API_KEY"), openRouterBaseURL, "openrouter")

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(provider),
		chainforge.WithModel(os.Getenv("MODEL")),
		chainforge.WithSystemPrompt("You are a helpful assistant. Use the available tools to answer questions."),
		chainforge.WithMCPServer(mcp.HTTP(mcpURL).WithName("remote")),
		chainforge.WithMaxIterations(5),
	)
	if err != nil {
		slog.Error("failed to create agent", "error", err)
		os.Exit(1)
	}
	defer agent.Close()

	result, err := agent.Run(ctx, "mcp-session", "What is LangChain and what are its main components?")
	if err != nil {
		slog.Error("agent run failed", "error", err)
		os.Exit(1)
	}

	fmt.Println(result)
}
