package chainforge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
	mcppkg "github.com/lioarce01/chainforge/pkg/mcp"
	"github.com/lioarce01/chainforge/pkg/structured"
	"github.com/lioarce01/chainforge/pkg/tools"
)

// Agent runs the agentic loop: call LLM → dispatch tools → repeat.
type Agent struct {
	cfg        agentConfig
	toolMap    map[string]core.Tool
	toolDefs   []core.ToolDefinition // cached; rebuilt after MCP connect
	mcpClients []*mcppkg.Client
	mcpMu      sync.Mutex
	mcpDone    bool  // true only after a successful connect
	mcpErr     error // last connect error; cleared by ReconnectMCP
}

func newAgent(cfg agentConfig) *Agent {
	tm := make(map[string]core.Tool, len(cfg.tools))
	for _, t := range cfg.tools {
		tm[t.Definition().Name] = t
	}
	clients := make([]*mcppkg.Client, len(cfg.mcpServers))
	for i, s := range cfg.mcpServers {
		clients[i] = mcppkg.NewClient(s, cfg.logger)
	}
	a := &Agent{cfg: cfg, toolMap: tm, mcpClients: clients}
	a.toolDefs = a.buildToolDefs()
	return a
}

// connectMCP connects all configured MCP servers in parallel and merges their
// tools into the agent's toolMap. Called exactly once via sync.Once.
func (a *Agent) connectMCP(ctx context.Context) error {
	if len(a.mcpClients) == 0 {
		return nil
	}

	type result struct {
		tools []core.Tool
		err   error
	}

	ch := make(chan result, len(a.mcpClients))
	for _, cl := range a.mcpClients {
		go func(c *mcppkg.Client) {
			if err := c.Connect(ctx); err != nil {
				ch <- result{err: err}
				return
			}
			ch <- result{tools: c.CoreTools()}
		}(cl)
	}

	var errs []error
	for range a.mcpClients {
		r := <-ch
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		for _, t := range r.tools {
			name := t.Definition().Name
			if _, exists := a.toolMap[name]; exists {
				a.cfg.logger.Warn("mcp: tool name collision, MCP tool wins",
					slog.String("name", name))
			}
			a.toolMap[name] = t
		}
	}
	// Rebuild cached tool definitions after MCP tools are merged in.
	a.toolDefs = a.buildToolDefs()
	return errors.Join(errs...)
}

// Close shuts down all MCP server connections. Call via defer after NewAgent.
func (a *Agent) Close() error {
	errs := make([]error, 0, len(a.mcpClients))
	for _, cl := range a.mcpClients {
		if err := cl.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Run executes the agent loop synchronously and returns the final text response.
// sessionID namespaces memory; use different IDs for independent conversations.
func (a *Agent) Run(ctx context.Context, sessionID, userMessage string) (string, error) {
	result, _, err := a.RunWithUsage(ctx, sessionID, userMessage)
	return result, err
}

// RunWithUsage is like Run but also returns the total token usage accumulated
// across all LLM calls in the agentic loop.
func (a *Agent) RunWithUsage(ctx context.Context, sessionID, userMessage string) (string, core.Usage, error) {
	if a.cfg.runTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.cfg.runTimeout)
		defer cancel()
	}

	start := time.Now()

	// Connect MCP servers (retryable on failure via ReconnectMCP).
	if err := a.ensureMCPConnected(ctx); err != nil {
		return "", core.Usage{}, fmt.Errorf("agent: MCP connect: %w", err)
	}

	// Inject session ID into context so middleware (tracing, logging) can correlate.
	ctx = core.WithSessionID(ctx, sessionID)

	// Load history from memory
	history, err := a.loadHistory(ctx, sessionID)
	if err != nil {
		return "", core.Usage{}, fmt.Errorf("agent: load history: %w", err)
	}

	// Append user message
	userMsg := core.Message{Role: core.RoleUser, Content: userMessage}
	history = append(history, userMsg)

	// Save user message to memory
	if a.cfg.memory != nil {
		if err := a.cfg.memory.Append(ctx, sessionID, userMsg); err != nil {
			return "", core.Usage{}, fmt.Errorf("agent: save user message: %w", err)
		}
	}

	// Prepend system message once — it will stay at history[0] throughout.
	history = a.prependSystem(history)
	var totalUsage core.Usage

	for i := 0; i < a.cfg.maxIterations; i++ {
		req := core.ChatRequest{
			Model:    a.cfg.model,
			Messages: history,
			Tools:    a.toolDefs,
			Options: core.ChatOptions{
				MaxTokens:    a.cfg.maxTokens,
				Temperature:  a.cfg.temperature,
				SystemPrompt: a.cfg.systemPrompt,
			},
		}

		a.cfg.logger.DebugContext(ctx, "agent: calling provider",
			slog.String("provider", a.cfg.provider.Name()),
			slog.String("model", a.cfg.model),
			slog.Int("iteration", i+1),
			slog.Int("messages", len(req.Messages)),
		)
		a.debug(ctx, DebugEvent{Kind: DebugLLMRequest, Iteration: i, Messages: req.Messages})

		resp, err := a.cfg.provider.Chat(ctx, req)
		if err != nil {
			return "", core.Usage{}, fmt.Errorf("agent: provider error: %w", err)
		}
		a.debug(ctx, DebugEvent{Kind: DebugLLMResponse, Iteration: i, Response: &resp})

		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens

		// Add assistant message to history
		history = append(history, resp.Message)
		if a.cfg.memory != nil {
			if err := a.cfg.memory.Append(ctx, sessionID, resp.Message); err != nil {
				return "", core.Usage{}, fmt.Errorf("agent: save assistant message: %w", err)
			}
		}

		switch resp.StopReason {
		case core.StopReasonEndTurn, core.StopReasonStop, core.StopReasonMaxTokens:
			a.cfg.logger.InfoContext(ctx, "agent: run complete",
				slog.String("session", sessionID),
				slog.Duration("duration", time.Since(start)),
				slog.Int("input_tokens", totalUsage.InputTokens),
				slog.Int("output_tokens", totalUsage.OutputTokens),
				slog.Int("iterations", i+1),
			)
			if len(a.cfg.outputSchema) > 0 {
				if err := structured.ValidateJSON(resp.Message.Content, a.cfg.outputSchema); err != nil {
					return "", totalUsage, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
				}
			}
			return resp.Message.Content, totalUsage, nil

		case core.StopReasonToolUse:
			if len(resp.Message.ToolCalls) == 0 {
				// Malformed response — treat as done
				return resp.Message.Content, totalUsage, nil
			}

			a.cfg.logger.DebugContext(ctx, "agent: dispatching tools",
				slog.Int("count", len(resp.Message.ToolCalls)),
			)

			toolMsgs, err := a.dispatchTools(ctx, resp.Message.ToolCalls)
			if err != nil {
				return "", core.Usage{}, err // only context cancellation propagates as hard error
			}

			history = append(history, toolMsgs...)
			if a.cfg.memory != nil {
				if err := a.cfg.memory.Append(ctx, sessionID, toolMsgs...); err != nil {
					return "", core.Usage{}, fmt.Errorf("agent: save tool messages: %w", err)
				}
			}

		default:
			// Unknown stop reason — treat as done
			return resp.Message.Content, totalUsage, nil
		}
	}

	a.cfg.logger.WarnContext(ctx, "agent: max iterations reached",
		slog.Int("max", a.cfg.maxIterations),
	)
	return "", core.Usage{}, core.ErrMaxIterations
}

// RunStream executes the agent loop and streams events.
// The final text is accumulated; tool calls are still dispatched synchronously.
// The returned channel is closed when done or on error.
func (a *Agent) RunStream(ctx context.Context, sessionID, userMessage string) <-chan core.StreamEvent {
	ch := make(chan core.StreamEvent, a.cfg.streamBufSize)
	go func() {
		defer close(ch)

		// send is a context-aware helper: returns false if ctx is done so the
		// goroutine can exit cleanly instead of blocking forever on a full channel.
		send := func(ev core.StreamEvent) bool {
			select {
			case ch <- ev:
				return true
			case <-ctx.Done():
				return false
			}
		}

		// Connect MCP servers (retryable on failure via ReconnectMCP).
		if err := a.ensureMCPConnected(ctx); err != nil {
			send(core.StreamEvent{
				Type:  core.StreamEventError,
				Error: fmt.Errorf("agent: MCP connect: %w", err),
			})
			return
		}

		// Inject session ID into context so middleware can correlate.
		ctx = core.WithSessionID(ctx, sessionID)

		history, err := a.loadHistory(ctx, sessionID)
		if err != nil {
			send(core.StreamEvent{Type: core.StreamEventError, Error: err})
			return
		}

		userMsg := core.Message{Role: core.RoleUser, Content: userMessage}
		history = append(history, userMsg)
		if a.cfg.memory != nil {
			_ = a.cfg.memory.Append(ctx, sessionID, userMsg)
		}

		// Prepend system message once — it will stay at history[0] throughout.
		history = a.prependSystem(history)
		var totalUsage core.Usage

		for i := 0; i < a.cfg.maxIterations; i++ {
			req := core.ChatRequest{
				Model:    a.cfg.model,
				Messages: history,
				Tools:    a.toolDefs,
				Options: core.ChatOptions{
					MaxTokens:    a.cfg.maxTokens,
					Temperature:  a.cfg.temperature,
					SystemPrompt: a.cfg.systemPrompt,
				},
			}

			stream, err := a.cfg.provider.ChatStream(ctx, req)
			if err != nil {
				send(core.StreamEvent{Type: core.StreamEventError, Error: err})
				return
			}

			var (
				sb         strings.Builder
				toolCalls  []core.ToolCall
				stopReason core.StopReason
				usage      core.Usage
			)

			for event := range stream {
				switch event.Type {
				case core.StreamEventText:
					sb.WriteString(event.TextDelta)
					if !send(event) {
						return
					}
				case core.StreamEventToolCall:
					if event.ToolCall != nil {
						toolCalls = append(toolCalls, *event.ToolCall)
					}
				case core.StreamEventDone:
					stopReason = event.StopReason
					if event.Usage != nil {
						usage = *event.Usage
					}
				case core.StreamEventError:
					send(event)
					return
				}
			}

			assistantMsg := core.Message{
				Role:      core.RoleAssistant,
				Content:   sb.String(),
				ToolCalls: toolCalls,
			}
			history = append(history, assistantMsg)
			totalUsage.InputTokens += usage.InputTokens
			totalUsage.OutputTokens += usage.OutputTokens
			if a.cfg.memory != nil {
				_ = a.cfg.memory.Append(ctx, sessionID, assistantMsg)
			}

			if stopReason != core.StopReasonToolUse || len(toolCalls) == 0 {
				send(core.StreamEvent{
					Type:       core.StreamEventDone,
					StopReason: stopReason,
					Usage:      &totalUsage,
				})
				return
			}

			toolMsgs, err := a.dispatchTools(ctx, toolCalls)
			if err != nil {
				send(core.StreamEvent{Type: core.StreamEventError, Error: err})
				return
			}
			history = append(history, toolMsgs...)
			if a.cfg.memory != nil {
				_ = a.cfg.memory.Append(ctx, sessionID, toolMsgs...)
			}
		}

		send(core.StreamEvent{Type: core.StreamEventError, Error: core.ErrMaxIterations})
	}()
	return ch
}

type toolResult struct {
	index int
	msg   core.Message
}

// dispatchTools runs all tool calls concurrently and collects results in original order.
// Tool execution errors are returned as tool result messages (non-fatal).
// Context cancellation is the only hard failure.
func (a *Agent) dispatchTools(ctx context.Context, toolCalls []core.ToolCall) ([]core.Message, error) {
	// Fast path: skip goroutine/channel/WaitGroup overhead for a single tool call.
	if len(toolCalls) == 1 {
		tc := toolCalls[0]
		toolCtx, cancel := context.WithTimeout(ctx, a.cfg.toolTimeout)
		defer cancel()

		output, err := a.callTool(toolCtx, tc)
		msg := core.Message{
			Role:       core.RoleTool,
			Content:    output,
			ToolCallID: tc.ID,
			Name:       tc.Name,
		}
		if err != nil {
			a.cfg.logger.WarnContext(ctx, "agent: tool error",
				slog.String("tool", tc.Name),
				slog.String("error", err.Error()),
			)
			msg.Content = fmt.Sprintf("error: %v", err)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return []core.Message{msg}, nil
	}

	results := make(chan toolResult, len(toolCalls))
	var wg sync.WaitGroup

	// Optional semaphore to cap concurrent goroutines (WithToolConcurrency).
	var sem chan struct{}
	if a.cfg.toolConcurrency > 0 {
		sem = make(chan struct{}, a.cfg.toolConcurrency)
	}

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, call core.ToolCall) {
			defer wg.Done()

			// Acquire semaphore slot if concurrency is bounded.
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					results <- toolResult{index: idx, msg: core.Message{
						Role:       core.RoleTool,
						Content:    fmt.Sprintf("error: %v", ctx.Err()),
						ToolCallID: call.ID,
						Name:       call.Name,
					}}
					return
				}
			}

			toolCtx, cancel := context.WithTimeout(ctx, a.cfg.toolTimeout)
			defer cancel()

			output, err := a.callTool(toolCtx, call)
			msg := core.Message{
				Role:       core.RoleTool,
				Content:    output,
				ToolCallID: call.ID,
				Name:       call.Name,
			}
			if err != nil {
				// Non-fatal: feed error as tool result so the LLM can handle it
				a.cfg.logger.WarnContext(ctx, "agent: tool error",
					slog.String("tool", call.Name),
					slog.String("error", err.Error()),
				)
				msg.Content = fmt.Sprintf("error: %v", err)
			}
			results <- toolResult{index: idx, msg: msg}
		}(i, tc)
	}

	// Close results after all goroutines finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and reorder by original index
	ordered := make([]core.Message, len(toolCalls))
	for r := range results {
		// Check for context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		ordered[r.index] = r.msg
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return ordered, nil
}

// callTool looks up and invokes a tool by name.
func (a *Agent) callTool(ctx context.Context, tc core.ToolCall) (string, error) {
	tool, ok := a.toolMap[tc.Name]
	if !ok {
		a.cfg.logger.WarnContext(ctx, "agent: unknown tool", slog.String("name", tc.Name))
		return "", &core.ToolError{ToolName: tc.Name, Err: core.ErrToolNotFound}
	}

	a.cfg.logger.DebugContext(ctx, "agent: calling tool",
		slog.String("name", tc.Name),
		slog.String("input", tc.Input),
	)
	a.debug(ctx, DebugEvent{Kind: DebugToolCall, ToolCall: &tc})

	output, err := tool.Call(ctx, tc.Input)
	a.debug(ctx, DebugEvent{Kind: DebugToolResult, ToolCall: &tc, ToolOutput: output, ToolError: err})
	if err != nil {
		return "", &core.ToolError{ToolName: tc.Name, Err: err}
	}
	return output, nil
}

// debug fires the debug handler if one is configured. Zero overhead when not set.
func (a *Agent) debug(ctx context.Context, ev DebugEvent) {
	if a.cfg.debugHandler != nil {
		a.cfg.debugHandler(ctx, ev)
	}
}

// loadHistory fetches history from memory (returns nil if no memory store).
// If WithMaxHistory is set and history exceeds the limit:
//   - WithHistorySummarizer: old messages are summarized into one compact message.
//   - Otherwise: oldest messages are dropped.
func (a *Agent) loadHistory(ctx context.Context, sessionID string) ([]core.Message, error) {
	if a.cfg.memory == nil {
		return nil, nil
	}
	msgs, err := a.cfg.memory.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if a.cfg.maxHistory > 0 && len(msgs) > a.cfg.maxHistory {
		if a.cfg.historySummarizer != nil {
			msgs, err = a.summarizeHistory(ctx, sessionID, msgs)
			if err != nil {
				return nil, err
			}
		} else {
			msgs = msgs[len(msgs)-a.cfg.maxHistory:]
		}
	}
	return msgs, nil
}

// summarizeHistory condenses messages that overflow maxHistory into a single
// summary message. It keeps (maxHistory-1) recent messages and prepends one
// summary message, for a total of maxHistory messages. The compressed history
// is persisted back to the memory store.
func (a *Agent) summarizeHistory(ctx context.Context, sessionID string, msgs []core.Message) ([]core.Message, error) {
	keep := a.cfg.maxHistory - 1
	if keep < 0 {
		keep = 0
	}
	toSummarize := msgs[:len(msgs)-keep]
	recent := msgs[len(msgs)-keep:]

	// Build a plain-text prompt of the messages to compress.
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation history concisely, preserving key facts and decisions:\n\n")
	for _, m := range toSummarize {
		sb.WriteString(string(m.Role))
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}

	summary, err := a.cfg.historySummarizer.Run(ctx, sessionID+":summarizer", sb.String())
	if err != nil {
		return nil, fmt.Errorf("history summarizer: %w", err)
	}

	summaryMsg := core.Message{
		Role:    core.RoleUser,
		Content: "[Summary: " + summary + "]",
	}

	compressed := append([]core.Message{summaryMsg}, recent...)

	// Persist the compressed history back so future runs start from the summary.
	if err := a.cfg.memory.Clear(ctx, sessionID); err != nil {
		return nil, fmt.Errorf("history summarizer clear: %w", err)
	}
	if err := a.cfg.memory.Append(ctx, sessionID, compressed...); err != nil {
		return nil, fmt.Errorf("history summarizer append: %w", err)
	}

	return compressed, nil
}

// prependSystem ensures the system prompt is the first message if configured.
// If WithStructuredOutput is set, a JSON schema hint is appended to the prompt.
func (a *Agent) prependSystem(history []core.Message) []core.Message {
	prompt := a.cfg.systemPrompt
	if len(a.cfg.outputSchema) > 0 {
		hint := "Respond only with valid JSON matching the following schema: " + string(a.cfg.outputSchema)
		if prompt == "" {
			prompt = hint
		} else {
			prompt = prompt + "\n\n" + hint
		}
	}
	if prompt == "" {
		return history
	}
	// Check if already present (avoids duplication on repeated calls)
	if len(history) > 0 && history[0].Role == core.RoleSystem {
		return history
	}
	sys := core.Message{Role: core.RoleSystem, Content: prompt}
	return append([]core.Message{sys}, history...)
}

// buildToolDefs extracts ToolDefinition from all registered tools (static + MCP).
// Results are sorted by name for deterministic LLM prompt ordering.
func (a *Agent) buildToolDefs() []core.ToolDefinition {
	if len(a.toolMap) == 0 {
		return nil
	}
	defs := make([]core.ToolDefinition, 0, len(a.toolMap))
	for _, t := range a.toolMap {
		defs = append(defs, t.Definition())
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}

// parseToolInput is a helper to unmarshal JSON tool input into a typed struct.
// Delegates to the public tools.ParseInput so users can call it directly.
func parseToolInput[T any](input string) (T, error) {
	return tools.ParseInput[T](input)
}

// WarmMCP pre-connects all configured MCP servers before the first Run call.
// Returns nil immediately if no MCP servers are configured.
// Calling WarmMCP is optional — Run and RunStream also connect lazily — but
// pre-connecting eliminates the latency spike on the first request.
func (a *Agent) WarmMCP(ctx context.Context) error {
	return a.ensureMCPConnected(ctx)
}

// ensureMCPConnected connects MCP servers if not already successfully connected.
// Safe for concurrent use; only the first caller runs connectMCP — unless
// ReconnectMCP has been called to reset the state after a failure.
func (a *Agent) ensureMCPConnected(ctx context.Context) error {
	a.mcpMu.Lock()
	defer a.mcpMu.Unlock()
	if a.mcpDone {
		return nil // already connected successfully
	}
	a.mcpErr = a.connectMCP(ctx)
	if a.mcpErr == nil {
		a.mcpDone = true
	}
	return a.mcpErr
}

// ReconnectMCP resets the connection state and re-attempts connecting all
// configured MCP servers. Use this to recover from a transient network failure
// without creating a new Agent.
func (a *Agent) ReconnectMCP(ctx context.Context) error {
	a.mcpMu.Lock()
	a.mcpDone = false
	a.mcpErr = nil
	a.mcpMu.Unlock()
	return a.ensureMCPConnected(ctx)
}
