package chainforge

import (
	"context"
	"fmt"
	"io"

	"github.com/lioarce01/chainforge/pkg/core"
)

// DebugEventKind identifies the stage of the agent loop a DebugEvent describes.
type DebugEventKind string

const (
	// DebugLLMRequest fires just before a Chat or ChatStream call.
	// DebugEvent.Messages contains the full message slice sent to the provider.
	DebugLLMRequest DebugEventKind = "llm_request"

	// DebugLLMResponse fires immediately after a successful Chat or ChatStream call.
	// DebugEvent.Response is set.
	DebugLLMResponse DebugEventKind = "llm_response"

	// DebugToolCall fires before each individual tool is invoked.
	// DebugEvent.ToolCall is set.
	DebugToolCall DebugEventKind = "tool_call"

	// DebugToolResult fires after each individual tool returns.
	// DebugEvent.ToolCall, DebugEvent.ToolOutput, and DebugEvent.ToolError are set.
	DebugToolResult DebugEventKind = "tool_result"
)

// DebugEvent carries the state of a single step in the agent loop.
type DebugEvent struct {
	Kind      DebugEventKind
	Iteration int

	// LLMRequest: full message slice sent to the provider.
	Messages []core.Message

	// LLMResponse: the provider response.
	Response *core.ChatResponse

	// ToolCall / ToolResult: the tool invocation.
	ToolCall   *core.ToolCall
	ToolOutput string
	ToolError  error
}

// DebugHandler is called synchronously at each significant step of the agent loop.
// It must not block. Use it for logging, recording, or printing during development.
type DebugHandler func(ctx context.Context, event DebugEvent)

// PrettyPrintDebugHandler returns a DebugHandler that writes a human-readable
// transcript of every agent loop step to w.
//
//	chainforge.WithDebugHandler(chainforge.PrettyPrintDebugHandler(os.Stderr))
func PrettyPrintDebugHandler(w io.Writer) DebugHandler {
	return func(_ context.Context, ev DebugEvent) {
		switch ev.Kind {
		case DebugLLMRequest:
			fmt.Fprintf(w, "[iter %d] → LLM  (%d messages)\n", ev.Iteration, len(ev.Messages))
		case DebugLLMResponse:
			if ev.Response != nil {
				fmt.Fprintf(w, "[iter %d] ← LLM  stop=%s  %q\n",
					ev.Iteration, ev.Response.StopReason,
					truncate(ev.Response.Message.Content, 120))
			}
		case DebugToolCall:
			if ev.ToolCall != nil {
				fmt.Fprintf(w, "[iter %d] ⚙  tool=%s  input=%s\n",
					ev.Iteration, ev.ToolCall.Name, truncate(ev.ToolCall.Input, 80))
			}
		case DebugToolResult:
			if ev.ToolError != nil {
				fmt.Fprintf(w, "[iter %d] ✗  tool=%s  err=%v\n",
					ev.Iteration, ev.ToolCall.Name, ev.ToolError)
			} else {
				fmt.Fprintf(w, "[iter %d] ✓  tool=%s  result=%s\n",
					ev.Iteration, ev.ToolCall.Name, truncate(ev.ToolOutput, 80))
			}
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
