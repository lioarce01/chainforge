package hitl

import "context"

// OnlyTools wraps inner so that approval is only requested for the named tools.
// All other tools are automatically approved without calling inner.
//
//	hitl.OnlyTools(hitl.NewCLIGateway(os.Stdout, os.Stdin), "send_email", "delete_file")
func OnlyTools(inner Gateway, names ...string) Gateway {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return &onlyTools{inner: inner, names: m}
}

type onlyTools struct {
	inner Gateway
	names map[string]bool
}

func (g *onlyTools) RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
	if !g.names[req.ToolName] {
		return ApprovalResponse{Approved: true}, nil
	}
	return g.inner.RequestApproval(ctx, req)
}

// ExcludeTools wraps inner so that the named tools are automatically approved
// without calling inner. All other tools must pass through inner.
//
//	hitl.ExcludeTools(hitl.NewCLIGateway(os.Stdout, os.Stdin), "read_file")
func ExcludeTools(inner Gateway, names ...string) Gateway {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return &excludeTools{inner: inner, names: m}
}

type excludeTools struct {
	inner Gateway
	names map[string]bool
}

func (g *excludeTools) RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
	if g.names[req.ToolName] {
		return ApprovalResponse{Approved: true}, nil
	}
	return g.inner.RequestApproval(ctx, req)
}
