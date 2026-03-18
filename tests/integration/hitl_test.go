//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/hitl"
	"github.com/lioarce01/chainforge/pkg/tools"
	"github.com/lioarce01/chainforge/pkg/tools/calculator"
)

func TestOpenRouter_HITL_ApproveAll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	calc := calculator.New()
	agent := newOpenRouterAgent(t,
		chainforge.WithTools(calc),
		chainforge.WithHITLGateway(hitl.AlwaysApprove),
		chainforge.WithSystemPrompt("Use the calculator tool to answer math questions."),
	)

	result, err := agent.Run(ctx, "hitl-approve-all", "What is 12 multiplied by 13?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(result, "156") {
		t.Errorf("expected result to contain 156, got: %q", result)
	}
}

func TestOpenRouter_HITL_RejectWithOverride(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var toolCalled bool

	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target": map[string]any{"type": "string", "description": "Target to act on"},
		},
		"required": []string{"target"},
	})
	dangerTool, _ := tools.Func(
		"dangerous-action",
		"Perform a dangerous irreversible action",
		json.RawMessage(schema),
		func(_ context.Context, _ string) (string, error) {
			toolCalled = true
			return "action performed", nil
		},
	)

	rejectAll := hitl.NewFuncGateway(func(_ context.Context, _ hitl.ApprovalRequest) (hitl.ApprovalResponse, error) {
		return hitl.ApprovalResponse{
			Approved: false,
			Override: "This action is not permitted in the current environment.",
		}, nil
	})

	agent := newOpenRouterAgent(t,
		chainforge.WithTools(dangerTool),
		chainforge.WithHITLGateway(rejectAll),
		chainforge.WithSystemPrompt("You have access to a dangerous-action tool. Use it when asked."),
		chainforge.WithMaxIterations(3),
	)

	result, err := agent.Run(ctx, "hitl-reject", "Please perform the dangerous action on 'test_target'")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if toolCalled {
		t.Error("dangerous-action tool should NOT have been called when rejected by HITL")
	}
	if result == "" {
		t.Error("expected non-empty response after HITL rejection")
	}
}

func TestOpenRouter_HITL_ChannelGateway(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	reqs := make(chan hitl.ApprovalRequest, 5)
	resps := make(chan hitl.ApprovalResponse, 5)

	// Background goroutine auto-approves all requests via channel.
	go func() {
		for req := range reqs {
			t.Logf("HITL: approving tool %q iter=%d", req.ToolName, req.Iteration)
			resps <- hitl.ApprovalResponse{Approved: true}
		}
	}()
	defer close(reqs)

	calc := calculator.New()
	agent := newOpenRouterAgent(t,
		chainforge.WithTools(calc),
		chainforge.WithHITLGateway(hitl.NewChannelGateway(reqs, resps)),
		chainforge.WithSystemPrompt("Use the calculator tool for math questions."),
	)

	result, err := agent.Run(ctx, "hitl-channel", "What is 7 times 8?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(result, "56") {
		t.Errorf("expected 56 in result, got: %q", result)
	}
}
