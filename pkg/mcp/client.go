package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpproto "github.com/mark3labs/mcp-go/mcp"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Client manages the lifecycle of a single MCP server connection.
type Client struct {
	cfg    ServerConfig
	logger *slog.Logger
	mu     sync.Mutex
	inner  *mcpclient.Client
	tools  []mcpTool
	closed bool
}

// NewClient creates a new (unconnected) MCP client for the given server.
func NewClient(cfg ServerConfig, logger *slog.Logger) *Client {
	return &Client{cfg: cfg, logger: logger}
}

// Connect dials the MCP server, negotiates capabilities, and loads available tools.
// Idempotent: calling Connect more than once is a no-op after the first success.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.inner != nil || c.closed {
		return nil
	}

	inner, err := c.dial(ctx)
	if err != nil {
		return fmt.Errorf("mcp: connect %q: %w", c.cfg.Name, err)
	}

	// Negotiate MCP session capabilities.
	if _, err = inner.Initialize(ctx, mcpproto.InitializeRequest{
		Params: mcpproto.InitializeParams{
			ProtocolVersion: mcpproto.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcpproto.Implementation{
				Name:    "chainforge",
				Version: "1.0.0",
			},
		},
	}); err != nil {
		_ = inner.Close()
		return fmt.Errorf("mcp: initialize %q: %w", c.cfg.Name, err)
	}

	// Discover available tools.
	toolsResult, err := inner.ListTools(ctx, mcpproto.ListToolsRequest{})
	if err != nil {
		_ = inner.Close()
		return fmt.Errorf("mcp: list tools %q: %w", c.cfg.Name, err)
	}

	tools := make([]mcpTool, 0, len(toolsResult.Tools))
	for _, t := range toolsResult.Tools {
		schema, merr := json.Marshal(t.InputSchema)
		if merr != nil {
			schema = json.RawMessage(`{}`)
		}
		tools = append(tools, mcpTool{
			client:     c,
			remoteName: t.Name,
			def: core.ToolDefinition{
				Name:        qualifiedName(c.cfg.Name, t.Name),
				Description: t.Description,
				InputSchema: schema,
			},
		})
	}

	c.inner = inner
	c.tools = tools
	c.logger.Info("mcp: connected",
		slog.String("server", c.cfg.Name),
		slog.Int("tools", len(tools)),
	)
	return nil
}

// CoreTools returns the tools exposed by this server as core.Tool instances.
// Must be called after Connect.
func (c *Client) CoreTools() []core.Tool {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]core.Tool, len(c.tools))
	for i := range c.tools {
		result[i] = &c.tools[i]
	}
	return result
}

// CallTool invokes a named tool on the remote MCP server.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	c.mu.Lock()
	inner := c.inner
	c.mu.Unlock()

	if inner == nil {
		return "", fmt.Errorf("mcp: server %q not connected", c.cfg.Name)
	}

	result, err := inner.CallTool(ctx, mcpproto.CallToolRequest{
		Params: mcpproto.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
	if err != nil {
		return "", fmt.Errorf("mcp: call tool %s: %w", name, err)
	}

	if result.IsError {
		return "", fmt.Errorf("mcp: tool %s returned error: %s", name, contentToString(result.Content))
	}

	return contentToString(result.Content), nil
}

// Close shuts down the MCP server connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.inner == nil {
		c.closed = true
		return nil
	}
	c.closed = true
	return c.inner.Close()
}

// dial creates and starts the underlying transport-specific client.
// Stdio auto-starts on creation; StreamableHTTP requires an explicit Start call.
func (c *Client) dial(ctx context.Context) (*mcpclient.Client, error) {
	switch c.cfg.Kind {
	case TransportStdio:
		cl, err := mcpclient.NewStdioMCPClient(c.cfg.Command, c.cfg.Env, c.cfg.Args...)
		if err != nil {
			return nil, err
		}
		return cl, nil

	case TransportHTTP:
		cl, err := mcpclient.NewStreamableHttpClient(c.cfg.URL)
		if err != nil {
			return nil, err
		}
		if err := cl.Start(ctx); err != nil {
			_ = cl.Close()
			return nil, err
		}
		return cl, nil

	default:
		return nil, fmt.Errorf("mcp: unknown transport kind: %d", c.cfg.Kind)
	}
}

// rawContent is a generic shape for unmarshaling MCP content blocks.
// We marshal each content item to JSON and unmarshal here so we don't
// depend on the concrete mcp-go types or their pointer/value semantics.
type rawContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	Resource *struct {
		URI      string `json:"uri,omitempty"`
		Text     string `json:"text,omitempty"`
		MIMEType string `json:"mimeType,omitempty"`
	} `json:"resource,omitempty"`
}

// contentToString converts MCP tool result content blocks to a plain string.
func contentToString(content []mcpproto.Content) string {
	var parts []string
	for _, item := range content {
		raw, err := json.Marshal(item)
		if err != nil {
			continue
		}
		var rc rawContent
		if err := json.Unmarshal(raw, &rc); err != nil {
			continue
		}
		switch rc.Type {
		case "text":
			if rc.Text != "" {
				parts = append(parts, rc.Text)
			}
		case "image":
			if decoded, err := base64.StdEncoding.DecodeString(rc.Data); err == nil {
				parts = append(parts, fmt.Sprintf("[image: %s, %d bytes]", rc.MIMEType, len(decoded)))
			} else {
				parts = append(parts, fmt.Sprintf("[image: %s]", rc.MIMEType))
			}
		case "audio":
			if decoded, err := base64.StdEncoding.DecodeString(rc.Data); err == nil {
				parts = append(parts, fmt.Sprintf("[audio: %s, %d bytes]", rc.MIMEType, len(decoded)))
			} else {
				parts = append(parts, fmt.Sprintf("[audio: %s]", rc.MIMEType))
			}
		case "resource":
			if rc.Resource != nil {
				if rc.Resource.Text != "" {
					parts = append(parts, rc.Resource.Text)
				} else if rc.Resource.URI != "" {
					parts = append(parts, fmt.Sprintf("[resource: %s]", rc.Resource.URI))
				}
			}
		default:
			parts = append(parts, fmt.Sprintf("[content type %q]", rc.Type))
		}
	}
	return strings.Join(parts, "\n")
}
