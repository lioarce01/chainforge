package mcp

import (
	"net/url"
	"path/filepath"
)

// TransportKind identifies the connection mechanism for an MCP server.
type TransportKind int

const (
	// TransportStdio launches a subprocess and communicates via stdin/stdout.
	TransportStdio TransportKind = iota

	// TransportHTTP connects over Streamable HTTP — the modern MCP transport
	// used by Cursor, Claude Code, and other production MCP clients.
	// Replaces the deprecated SSE transport.
	TransportHTTP
)

// ServerConfig is a pure value type describing how to connect to one MCP server.
// No I/O happens here; use NewClient to establish a connection.
type ServerConfig struct {
	Name    string        // tool name prefix (derived from command/host if empty)
	Kind    TransportKind
	Command string   // stdio only: executable
	Args    []string // stdio only: arguments
	Env     []string // stdio only: extra environment variables (KEY=VALUE)
	URL     string   // HTTP only: server URL
}

// Stdio returns a config for a subprocess MCP server (stdin/stdout transport).
//
//	mcp.Stdio("npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp").WithName("fs")
func Stdio(command string, args ...string) ServerConfig {
	return ServerConfig{
		Kind:    TransportStdio,
		Command: command,
		Args:    args,
		Name:    deriveName(command, ""),
	}
}

// HTTP returns a config for a remote MCP server using Streamable HTTP transport.
// This is the modern transport used by Cursor, Claude Code, and hosted MCP services.
//
//	mcp.HTTP("https://api.example.com/mcp").WithName("myserver")
func HTTP(serverURL string) ServerConfig {
	return ServerConfig{
		Kind: TransportHTTP,
		URL:  serverURL,
		Name: deriveName("", serverURL),
	}
}

// WithName sets the server name used as a tool name prefix.
// e.g. server "fs" + tool "read_file" → "fs__read_file"
// Returns a copy; the original is unmodified.
func (s ServerConfig) WithName(name string) ServerConfig {
	s.Name = name
	return s
}

// WithEnv adds extra environment variables for stdio servers (e.g. "API_KEY=abc").
// Returns a copy; the original is unmodified.
func (s ServerConfig) WithEnv(env ...string) ServerConfig {
	s.Env = append(append([]string{}, s.Env...), env...)
	return s
}

// deriveName produces a readable default name from the command or URL.
func deriveName(command, rawURL string) string {
	if command != "" {
		return filepath.Base(command)
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "mcp"
	}
	return u.Host
}
