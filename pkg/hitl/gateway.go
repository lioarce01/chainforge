// Package hitl provides Human-in-the-Loop approval gates for the agent tool loop.
// Before a tool executes, the configured Gateway is called with the tool name and
// input; the gateway can approve (tool runs normally) or reject (tool is skipped,
// an override message is returned to the LLM instead).
package hitl

import "context"

// ApprovalRequest describes a pending tool invocation awaiting human review.
type ApprovalRequest struct {
	// SessionID is the current agent session.
	SessionID string
	// Iteration is the current agent loop iteration (0-based).
	Iteration int
	// ToolName is the name of the tool the LLM requested.
	ToolName string
	// ToolInput is the raw JSON input string passed to the tool.
	ToolInput string
}

// ApprovalResponse is the gateway's decision for a pending tool call.
type ApprovalResponse struct {
	// Approved indicates whether the tool may execute.
	Approved bool
	// Override is the message returned to the LLM when Approved is false.
	// If empty, "Action not approved." is used.
	Override string
}

// Gateway decides whether a tool call is allowed to proceed.
// It is called synchronously before each tool execution.
// Implementations must be safe for concurrent use.
type Gateway interface {
	RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error)
}
