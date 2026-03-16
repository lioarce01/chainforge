package chainforge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Agent runs the agentic loop: call LLM → dispatch tools → repeat.
type Agent struct {
	cfg     agentConfig
	toolMap map[string]core.Tool
}

func newAgent(cfg agentConfig) *Agent {
	tm := make(map[string]core.Tool, len(cfg.tools))
	for _, t := range cfg.tools {
		tm[t.Definition().Name] = t
	}
	return &Agent{cfg: cfg, toolMap: tm}
}

// Run executes the agent loop synchronously and returns the final text response.
// sessionID namespaces memory; use different IDs for independent conversations.
func (a *Agent) Run(ctx context.Context, sessionID, userMessage string) (string, error) {
	start := time.Now()

	// Load history from memory
	history, err := a.loadHistory(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("agent: load history: %w", err)
	}

	// Append user message
	userMsg := core.Message{Role: core.RoleUser, Content: userMessage}
	history = append(history, userMsg)

	// Save user message to memory
	if a.cfg.memory != nil {
		if err := a.cfg.memory.Append(ctx, sessionID, userMsg); err != nil {
			return "", fmt.Errorf("agent: save user message: %w", err)
		}
	}

	toolDefs := a.buildToolDefs()
	var totalUsage core.Usage

	for i := 0; i < a.cfg.maxIterations; i++ {
		req := core.ChatRequest{
			Model:    a.cfg.model,
			Messages: a.prependSystem(history),
			Tools:    toolDefs,
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

		resp, err := a.cfg.provider.Chat(ctx, req)
		if err != nil {
			return "", fmt.Errorf("agent: provider error: %w", err)
		}

		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens

		// Add assistant message to history
		history = append(history, resp.Message)
		if a.cfg.memory != nil {
			if err := a.cfg.memory.Append(ctx, sessionID, resp.Message); err != nil {
				return "", fmt.Errorf("agent: save assistant message: %w", err)
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
			return resp.Message.Content, nil

		case core.StopReasonToolUse:
			if len(resp.Message.ToolCalls) == 0 {
				// Malformed response — treat as done
				return resp.Message.Content, nil
			}

			a.cfg.logger.DebugContext(ctx, "agent: dispatching tools",
				slog.Int("count", len(resp.Message.ToolCalls)),
			)

			toolMsgs, err := a.dispatchTools(ctx, resp.Message.ToolCalls)
			if err != nil {
				return "", err // only context cancellation propagates as hard error
			}

			history = append(history, toolMsgs...)
			if a.cfg.memory != nil {
				if err := a.cfg.memory.Append(ctx, sessionID, toolMsgs...); err != nil {
					return "", fmt.Errorf("agent: save tool messages: %w", err)
				}
			}

		default:
			// Unknown stop reason — treat as done
			return resp.Message.Content, nil
		}
	}

	a.cfg.logger.WarnContext(ctx, "agent: max iterations reached",
		slog.Int("max", a.cfg.maxIterations),
	)
	return "", core.ErrMaxIterations
}

// RunStream executes the agent loop and streams events.
// The final text is accumulated; tool calls are still dispatched synchronously.
// The returned channel is closed when done or on error.
func (a *Agent) RunStream(ctx context.Context, sessionID, userMessage string) <-chan core.StreamEvent {
	ch := make(chan core.StreamEvent, 16)
	go func() {
		defer close(ch)

		history, err := a.loadHistory(ctx, sessionID)
		if err != nil {
			ch <- core.StreamEvent{Type: core.StreamEventError, Error: err}
			return
		}

		userMsg := core.Message{Role: core.RoleUser, Content: userMessage}
		history = append(history, userMsg)
		if a.cfg.memory != nil {
			_ = a.cfg.memory.Append(ctx, sessionID, userMsg)
		}

		toolDefs := a.buildToolDefs()

		for i := 0; i < a.cfg.maxIterations; i++ {
			req := core.ChatRequest{
				Model:    a.cfg.model,
				Messages: a.prependSystem(history),
				Tools:    toolDefs,
				Options: core.ChatOptions{
					MaxTokens:    a.cfg.maxTokens,
					Temperature:  a.cfg.temperature,
					SystemPrompt: a.cfg.systemPrompt,
				},
			}

			stream, err := a.cfg.provider.ChatStream(ctx, req)
			if err != nil {
				ch <- core.StreamEvent{Type: core.StreamEventError, Error: err}
				return
			}

			var (
				textContent string
				toolCalls   []core.ToolCall
				stopReason  core.StopReason
				usage       core.Usage
			)

			for event := range stream {
				switch event.Type {
				case core.StreamEventText:
					textContent += event.TextDelta
					ch <- event
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
					ch <- event
					return
				}
			}

			assistantMsg := core.Message{
				Role:      core.RoleAssistant,
				Content:   textContent,
				ToolCalls: toolCalls,
			}
			history = append(history, assistantMsg)
			_ = usage // usage tracked per iteration
			if a.cfg.memory != nil {
				_ = a.cfg.memory.Append(ctx, sessionID, assistantMsg)
			}

			if stopReason != core.StopReasonToolUse || len(toolCalls) == 0 {
				ch <- core.StreamEvent{
					Type:       core.StreamEventDone,
					StopReason: stopReason,
				}
				return
			}

			toolMsgs, err := a.dispatchTools(ctx, toolCalls)
			if err != nil {
				ch <- core.StreamEvent{Type: core.StreamEventError, Error: err}
				return
			}
			history = append(history, toolMsgs...)
			if a.cfg.memory != nil {
				_ = a.cfg.memory.Append(ctx, sessionID, toolMsgs...)
			}
		}

		ch <- core.StreamEvent{Type: core.StreamEventError, Error: core.ErrMaxIterations}
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
	results := make(chan toolResult, len(toolCalls))
	var wg sync.WaitGroup

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, call core.ToolCall) {
			defer wg.Done()

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

	output, err := tool.Call(ctx, tc.Input)
	if err != nil {
		return "", &core.ToolError{ToolName: tc.Name, Err: err}
	}
	return output, nil
}

// loadHistory fetches history from memory (returns nil if no memory store).
func (a *Agent) loadHistory(ctx context.Context, sessionID string) ([]core.Message, error) {
	if a.cfg.memory == nil {
		return nil, nil
	}
	return a.cfg.memory.Get(ctx, sessionID)
}

// prependSystem ensures the system prompt is the first message if configured.
func (a *Agent) prependSystem(history []core.Message) []core.Message {
	if a.cfg.systemPrompt == "" {
		return history
	}
	// Check if already present (avoids duplication on repeated calls)
	if len(history) > 0 && history[0].Role == core.RoleSystem {
		return history
	}
	sys := core.Message{Role: core.RoleSystem, Content: a.cfg.systemPrompt}
	return append([]core.Message{sys}, history...)
}

// buildToolDefs extracts ToolDefinition from all registered tools.
func (a *Agent) buildToolDefs() []core.ToolDefinition {
	if len(a.cfg.tools) == 0 {
		return nil
	}
	defs := make([]core.ToolDefinition, len(a.cfg.tools))
	for i, t := range a.cfg.tools {
		defs[i] = t.Definition()
	}
	return defs
}

// parseToolInput is a helper to unmarshal JSON tool input into a typed struct.
func parseToolInput[T any](input string) (T, error) {
	var v T
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return v, fmt.Errorf("invalid tool input: %w", err)
	}
	return v, nil
}
