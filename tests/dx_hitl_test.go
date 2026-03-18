package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/hitl"
	"github.com/lioarce01/chainforge/pkg/testutil"
	"github.com/lioarce01/chainforge/pkg/tools"
)

// makeTool creates a simple FuncTool for use in HITL tests.
func makeTool(name string, fn func(context.Context, string) (string, error)) core.Tool {
	schema, _ := json.Marshal(map[string]any{"type": "object", "properties": map[string]any{}})
	t, _ := tools.Func(name, name+" tool", json.RawMessage(schema), fn)
	return t
}

// --- Gateway unit tests ---

func TestFuncGateway_ApprovesWhenFnReturnsTrue(t *testing.T) {
	gw := hitl.NewFuncGateway(func(_ context.Context, req hitl.ApprovalRequest) (hitl.ApprovalResponse, error) {
		return hitl.ApprovalResponse{Approved: true}, nil
	})
	resp, err := gw.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "search"})
	if err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	if !resp.Approved {
		t.Error("expected Approved=true")
	}
}

func TestFuncGateway_RejectsAndUsesOverride(t *testing.T) {
	gw := hitl.NewFuncGateway(func(_ context.Context, _ hitl.ApprovalRequest) (hitl.ApprovalResponse, error) {
		return hitl.ApprovalResponse{Approved: false, Override: "not allowed"}, nil
	})
	resp, err := gw.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "delete"})
	if err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	if resp.Approved {
		t.Error("expected Approved=false")
	}
	if resp.Override != "not allowed" {
		t.Errorf("Override = %q, want %q", resp.Override, "not allowed")
	}
}

func TestAlwaysApprove_NeverBlocks(t *testing.T) {
	for _, name := range []string{"send_email", "delete_file", "drop_table"} {
		resp, err := hitl.AlwaysApprove.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: name})
		if err != nil {
			t.Fatalf("%s: RequestApproval error: %v", name, err)
		}
		if !resp.Approved {
			t.Errorf("%s: expected Approved=true from AlwaysApprove", name)
		}
	}
}

func TestOnlyTools_InterceptsOnlyNamedTools(t *testing.T) {
	var intercepted []string
	inner := hitl.NewFuncGateway(func(_ context.Context, req hitl.ApprovalRequest) (hitl.ApprovalResponse, error) {
		intercepted = append(intercepted, req.ToolName)
		return hitl.ApprovalResponse{Approved: true}, nil
	})
	gw := hitl.OnlyTools(inner, "send_email", "delete_file")

	// Named tool — should go through inner.
	gw.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "send_email"})
	if len(intercepted) != 1 || intercepted[0] != "send_email" {
		t.Errorf("expected send_email intercepted, got %v", intercepted)
	}
}

func TestOnlyTools_PassesThroughNonMatchingTools(t *testing.T) {
	innerCalled := false
	inner := hitl.NewFuncGateway(func(_ context.Context, _ hitl.ApprovalRequest) (hitl.ApprovalResponse, error) {
		innerCalled = true
		return hitl.ApprovalResponse{Approved: true}, nil
	})
	gw := hitl.OnlyTools(inner, "send_email")

	resp, err := gw.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "read_file"})
	if err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	if !resp.Approved {
		t.Error("non-matching tool should be auto-approved")
	}
	if innerCalled {
		t.Error("inner should not be called for non-matching tool")
	}
}

func TestExcludeTools_SkipsNamedTools(t *testing.T) {
	innerCalled := false
	inner := hitl.NewFuncGateway(func(_ context.Context, _ hitl.ApprovalRequest) (hitl.ApprovalResponse, error) {
		innerCalled = true
		return hitl.ApprovalResponse{Approved: false}, nil
	})
	gw := hitl.ExcludeTools(inner, "read_file")

	// Excluded tool: auto-approved without calling inner.
	resp, _ := gw.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "read_file"})
	if !resp.Approved {
		t.Error("excluded tool should be auto-approved")
	}
	if innerCalled {
		t.Error("inner should not be called for excluded tool")
	}

	// Non-excluded: goes through inner.
	gw.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "delete_file"})
	if !innerCalled {
		t.Error("inner should be called for non-excluded tool")
	}
}

func TestChannelGateway_SendsRequestReceivesResponse(t *testing.T) {
	reqs := make(chan hitl.ApprovalRequest, 1)
	resps := make(chan hitl.ApprovalResponse, 1)
	gw := hitl.NewChannelGateway(reqs, resps)

	// Respond in background.
	go func() {
		req := <-reqs
		resps <- hitl.ApprovalResponse{Approved: req.ToolName == "safe_tool"}
	}()

	resp, err := gw.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "safe_tool"})
	if err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	if !resp.Approved {
		t.Error("expected safe_tool to be approved")
	}
}

// --- Agent integration tests ---

func TestWithHITLGateway_ApprovedToolExecutes(t *testing.T) {
	var toolExecuted bool

	addTool := makeTool("add", func(_ context.Context, _ string) (string, error) {
		toolExecuted = true
		return "42", nil
	})

	p := testutil.NewMockProvider(
		testutil.ToolUseResponse(core.ToolCall{Name: "add", Input: `{"a":1,"b":2}`}),
		testutil.EndTurnResponse("The answer is 42"),
	)
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithTools(addTool),
		chainforge.WithHITLGateway(hitl.AlwaysApprove),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), "s1", "add 1 and 2")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !toolExecuted {
		t.Error("expected tool to execute when approved")
	}
	if result != "The answer is 42" {
		t.Errorf("result = %q, want %q", result, "The answer is 42")
	}
}

func TestWithHITLGateway_RejectedToolSkippedWithOverride(t *testing.T) {
	toolExecuted := false

	deleteTool := makeTool("delete", func(_ context.Context, _ string) (string, error) {
		toolExecuted = true
		return "deleted", nil
	})

	rejectAll := hitl.NewFuncGateway(func(_ context.Context, _ hitl.ApprovalRequest) (hitl.ApprovalResponse, error) {
		return hitl.ApprovalResponse{Approved: false, Override: "Action denied by policy."}, nil
	})

	p := testutil.NewMockProvider(
		testutil.ToolUseResponse(core.ToolCall{Name: "delete", Input: `{}`}),
		testutil.EndTurnResponse("I cannot delete files."),
	)
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithTools(deleteTool),
		chainforge.WithHITLGateway(rejectAll),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), "s1", "delete everything")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if toolExecuted {
		t.Error("tool should NOT have executed when rejected")
	}
	// LLM should have seen the override message and responded accordingly.
	if result == "" {
		t.Error("expected a non-empty final response")
	}

	// Verify the override message was passed to the LLM.
	calls := p.Calls()
	if len(calls) < 2 {
		t.Fatalf("expected at least 2 provider calls, got %d", len(calls))
	}
	lastReq := calls[len(calls)-1]
	found := false
	for _, msg := range lastReq.Request.Messages {
		if msg.Role == core.RoleTool && strings.Contains(msg.Content, "Action denied by policy.") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected override message in history sent to LLM")
	}
}

func TestWithHITLGateway_MultipleToolsPartialApproval(t *testing.T) {
	var approvedCalls []string
	var rejectedCalls []string

	approveTool := makeTool("approve_tool", func(_ context.Context, _ string) (string, error) {
		approvedCalls = append(approvedCalls, "approve_tool")
		return "ok", nil
	})
	rejectTool := makeTool("reject_tool", func(_ context.Context, _ string) (string, error) {
		rejectedCalls = append(rejectedCalls, "reject_tool")
		return "ok", nil
	})

	gw := hitl.OnlyTools(
		hitl.NewFuncGateway(func(_ context.Context, req hitl.ApprovalRequest) (hitl.ApprovalResponse, error) {
			return hitl.ApprovalResponse{Approved: false, Override: "rejected"}, nil
		}),
		"reject_tool",
	)

	p := testutil.NewMockProvider(
		testutil.ToolUseResponse(
			core.ToolCall{Name: "approve_tool", Input: `{}`},
			core.ToolCall{Name: "reject_tool", Input: `{}`},
		),
		testutil.EndTurnResponse("Done."),
	)
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithTools(approveTool, rejectTool),
		chainforge.WithHITLGateway(gw),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	agent.Run(context.Background(), "s1", "run both tools")

	if len(approvedCalls) != 1 {
		t.Errorf("approve_tool should have executed once, got %d", len(approvedCalls))
	}
	if len(rejectedCalls) != 0 {
		t.Errorf("reject_tool should NOT have executed, got %d", len(rejectedCalls))
	}
}

func TestWithHITLGateway_DebugEventsFireForHITL(t *testing.T) {
	var hitlRequestFired, hitlResponseFired bool

	addTool := makeTool("add", func(_ context.Context, _ string) (string, error) {
		return "7", nil
	})

	p := testutil.NewMockProvider(
		testutil.ToolUseResponse(core.ToolCall{Name: "add", Input: `{}`}),
		testutil.EndTurnResponse("done"),
	)
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithTools(addTool),
		chainforge.WithHITLGateway(hitl.AlwaysApprove),
		chainforge.WithDebugHandler(func(_ context.Context, ev chainforge.DebugEvent) {
			if ev.Kind == chainforge.DebugHITLRequest {
				hitlRequestFired = true
			}
			if ev.Kind == chainforge.DebugHITLResponse {
				hitlResponseFired = true
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	agent.Run(context.Background(), "s1", "add numbers")

	if !hitlRequestFired {
		t.Error("DebugHITLRequest event never fired")
	}
	if !hitlResponseFired {
		t.Error("DebugHITLResponse event never fired")
	}
}
