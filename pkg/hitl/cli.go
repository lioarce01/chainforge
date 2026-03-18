package hitl

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

type cliGateway struct {
	out io.Writer
	in  io.Reader
}

// NewCLIGateway creates a Gateway that prompts on out and reads decisions from in.
// Accepts "y" or "yes" (case-insensitive) to approve; anything else rejects.
//
//	agent, _ := chainforge.NewAgent(
//	    chainforge.WithHITLGateway(hitl.NewCLIGateway(os.Stdout, os.Stdin)),
//	)
func NewCLIGateway(out io.Writer, in io.Reader) Gateway {
	return &cliGateway{out: out, in: in}
}

func (g *cliGateway) RequestApproval(_ context.Context, req ApprovalRequest) (ApprovalResponse, error) {
	fmt.Fprintf(g.out, "[HITL] Approve tool %q with input %s? [y/N] ", req.ToolName, req.ToolInput)
	scanner := bufio.NewScanner(g.in)
	if !scanner.Scan() {
		return ApprovalResponse{Approved: false, Override: "Action not approved."}, nil
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer == "y" || answer == "yes" {
		return ApprovalResponse{Approved: true}, nil
	}
	return ApprovalResponse{Approved: false, Override: "Action not approved."}, nil
}
