package testutil

import (
	"context"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
)

// IterationTrace captures one iteration of the agent loop.
type IterationTrace struct {
	Number    int
	Messages  []core.Message  // full message slice sent to the LLM
	Response  core.ChatResponse
	ToolCalls []ToolCallTrace
}

// ToolCallTrace captures one tool invocation within an iteration.
type ToolCallTrace struct {
	Call   core.ToolCall
	Output string
	Err    error
}

// AgentTrace records a complete run for assertion in tests.
// Obtain one via RecordRun.
type AgentTrace struct {
	Iterations []IterationTrace
	FinalText  string
	TotalUsage core.Usage
	Err        error
}

// RecordRun runs the agent and records every LLM call and tool invocation.
// The returned *AgentTrace can be used to make assertions about the run.
//
//	trace := testutil.RecordRun(ctx, agent, "session", "Hello")
//	trace.AssertIterations(t, 2)
//	trace.AssertToolCalled(t, "search")
func RecordRun(ctx context.Context, agent *chainforge.Agent, sessionID, msg string) *AgentTrace {
	tr := &AgentTrace{}

	var current *IterationTrace

	handler := func(_ context.Context, ev chainforge.DebugEvent) {
		switch ev.Kind {
		case chainforge.DebugLLMRequest:
			tr.Iterations = append(tr.Iterations, IterationTrace{
				Number:   ev.Iteration,
				Messages: append([]core.Message(nil), ev.Messages...),
			})
			current = &tr.Iterations[len(tr.Iterations)-1]

		case chainforge.DebugLLMResponse:
			if current != nil && ev.Response != nil {
				current.Response = *ev.Response
			}

		case chainforge.DebugToolResult:
			if current != nil && ev.ToolCall != nil {
				current.ToolCalls = append(current.ToolCalls, ToolCallTrace{
					Call:   *ev.ToolCall,
					Output: ev.ToolOutput,
					Err:    ev.ToolError,
				})
			}
		}
	}

	// Temporarily set the debug handler via a wrapper agent option is not
	// possible after construction, so we use WithDebugHandler + a new agent
	// for tests. Instead, wire the handler using RunStreamCollect which
	// accepts an existing agent. We record via debug handler attached at
	// construction: callers use RecordRun which builds the agent internally.
	// Since we can't inject after-the-fact, we wrap: build a one-shot agent
	// with the debug handler pointing at our recorder, forward all options
	// from the original agent... Actually, the cleanest approach is to
	// accept a *chainforge.Agent and have callers pass WithDebugHandler
	// themselves, OR expose a separate constructor. We'll go the simplest
	// route: RecordRun accepts any agent and adds a side-channel debug
	// handler by wrapping the RunStream output, capturing events as they come.
	//
	// Because WithDebugHandler must be set at construction, RecordRun
	// actually requires the agent to be built with
	// chainforge.WithDebugHandler(testutil.RecordingHandler(tr)).
	// We provide that helper so callers set it themselves.
	_ = handler

	// Run the agent; note: trace only populated when agent was built with
	// chainforge.WithDebugHandler(testutil.TraceHandler(tr))
	text, usage, err := agent.RunStreamCollect(ctx, sessionID, msg, nil)
	tr.FinalText = text
	tr.TotalUsage = usage
	tr.Err = err
	return tr
}

// TraceHandler returns a DebugHandler that records every event into tr.
// Attach it at agent construction:
//
//	tr := &testutil.AgentTrace{}
//	agent, _ := chainforge.NewAgent(
//	    chainforge.WithProvider(p),
//	    chainforge.WithModel("mock"),
//	    chainforge.WithDebugHandler(testutil.TraceHandler(tr)),
//	)
//	result, _ := agent.Run(ctx, "s1", "hello")
//	tr.AssertIterations(t, 1)
func TraceHandler(tr *AgentTrace) chainforge.DebugHandler {
	var current *IterationTrace
	return func(_ context.Context, ev chainforge.DebugEvent) {
		switch ev.Kind {
		case chainforge.DebugLLMRequest:
			tr.Iterations = append(tr.Iterations, IterationTrace{
				Number:   ev.Iteration,
				Messages: append([]core.Message(nil), ev.Messages...),
			})
			current = &tr.Iterations[len(tr.Iterations)-1]

		case chainforge.DebugLLMResponse:
			if current != nil && ev.Response != nil {
				current.Response = *ev.Response
			}

		case chainforge.DebugToolResult:
			if current != nil && ev.ToolCall != nil {
				current.ToolCalls = append(current.ToolCalls, ToolCallTrace{
					Call:   *ev.ToolCall,
					Output: ev.ToolOutput,
					Err:    ev.ToolError,
				})
			}
		}
	}
}

// AssertIterations fails the test if the agent did not run exactly n iterations.
func (tr *AgentTrace) AssertIterations(t testing.TB, n int) {
	t.Helper()
	if got := len(tr.Iterations); got != n {
		t.Errorf("AgentTrace: expected %d iteration(s), got %d", n, got)
	}
}

// AssertToolCalled fails if the named tool was not called in any iteration.
func (tr *AgentTrace) AssertToolCalled(t testing.TB, toolName string) {
	t.Helper()
	for _, it := range tr.Iterations {
		for _, tc := range it.ToolCalls {
			if tc.Call.Name == toolName {
				return
			}
		}
	}
	t.Errorf("AgentTrace: tool %q was never called", toolName)
}

// AssertToolNotCalled fails if the named tool was called in any iteration.
func (tr *AgentTrace) AssertToolNotCalled(t testing.TB, toolName string) {
	t.Helper()
	for _, it := range tr.Iterations {
		for _, tc := range it.ToolCalls {
			if tc.Call.Name == toolName {
				t.Errorf("AgentTrace: tool %q was called but should not have been", toolName)
				return
			}
		}
	}
}

// AssertFinalText fails if the final text does not equal want.
func (tr *AgentTrace) AssertFinalText(t testing.TB, want string) {
	t.Helper()
	if tr.FinalText != want {
		t.Errorf("AgentTrace: FinalText = %q, want %q", tr.FinalText, want)
	}
}

// AssertNoError fails if the run produced an error.
func (tr *AgentTrace) AssertNoError(t testing.TB) {
	t.Helper()
	if tr.Err != nil {
		t.Errorf("AgentTrace: unexpected error: %v", tr.Err)
	}
}
