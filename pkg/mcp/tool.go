package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lioarce01/chainforge/pkg/core"
)

// mcpTool wraps a single remote MCP tool as a core.Tool.
type mcpTool struct {
	client     *Client
	remoteName string             // original tool name on the server (unqualified)
	def        core.ToolDefinition
}

func (t *mcpTool) Definition() core.ToolDefinition {
	return t.def
}

func (t *mcpTool) Call(ctx context.Context, input string) (string, error) {
	var args map[string]any
	if input != "" && input != "{}" {
		if err := json.Unmarshal([]byte(input), &args); err != nil {
			return "", fmt.Errorf("mcp tool %s: invalid input JSON: %w", t.def.Name, err)
		}
	}
	return t.client.CallTool(ctx, t.remoteName, args)
}

// qualifiedName produces a unique, LLM-friendly tool name.
// e.g. server "fs" + tool "read_file" → "fs__read_file"
func qualifiedName(serverName, toolName string) string {
	if serverName == "" {
		return toolName
	}
	return serverName + "__" + toolName
}
