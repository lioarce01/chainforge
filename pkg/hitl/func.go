package hitl

import "context"

type funcGateway struct {
	fn func(context.Context, ApprovalRequest) (ApprovalResponse, error)
}

// NewFuncGateway creates a Gateway from a plain function.
// This is the simplest way to integrate custom approval logic:
//
//	hitl.NewFuncGateway(func(ctx context.Context, req hitl.ApprovalRequest) (hitl.ApprovalResponse, error) {
//	    fmt.Printf("Approve %s? [y/n] ", req.ToolName)
//	    var s string; fmt.Scan(&s)
//	    return hitl.ApprovalResponse{Approved: s == "y"}, nil
//	})
func NewFuncGateway(fn func(context.Context, ApprovalRequest) (ApprovalResponse, error)) Gateway {
	return &funcGateway{fn: fn}
}

func (g *funcGateway) RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
	return g.fn(ctx, req)
}
