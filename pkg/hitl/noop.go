package hitl

import "context"

type alwaysApproveGateway struct{}

// AlwaysApprove is a Gateway that approves every tool call without prompting.
// Use it in tests or environments where all tool calls are implicitly trusted.
var AlwaysApprove Gateway = &alwaysApproveGateway{}

func (g *alwaysApproveGateway) RequestApproval(_ context.Context, _ ApprovalRequest) (ApprovalResponse, error) {
	return ApprovalResponse{Approved: true}, nil
}
