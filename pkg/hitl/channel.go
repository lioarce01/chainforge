package hitl

import "context"

type channelGateway struct {
	requests  chan<- ApprovalRequest
	responses <-chan ApprovalResponse
}

// NewChannelGateway creates a Gateway that communicates via Go channels.
// Useful for integrating approvals with HTTP servers, UIs, or test harnesses.
//
//	requests := make(chan hitl.ApprovalRequest)
//	responses := make(chan hitl.ApprovalResponse)
//	go func() {
//	    for req := range requests {
//	        responses <- hitl.ApprovalResponse{Approved: true}
//	    }
//	}()
//	gateway := hitl.NewChannelGateway(requests, responses)
func NewChannelGateway(requests chan<- ApprovalRequest, responses <-chan ApprovalResponse) Gateway {
	return &channelGateway{requests: requests, responses: responses}
}

func (g *channelGateway) RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
	select {
	case g.requests <- req:
	case <-ctx.Done():
		return ApprovalResponse{}, ctx.Err()
	}
	select {
	case resp := <-g.responses:
		return resp, nil
	case <-ctx.Done():
		return ApprovalResponse{}, ctx.Err()
	}
}
