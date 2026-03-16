package anthropic

import (
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/lioarce01/chainforge/pkg/core"
)

// toCoreResponse translates an anthropic.Message to core.ChatResponse.
func toCoreResponse(msg anthropic.Message) (core.ChatResponse, error) {
	resp := core.ChatResponse{
		StopReason: toCoreStopReason(string(msg.StopReason)),
		Usage: core.Usage{
			InputTokens:  int(msg.Usage.InputTokens),
			OutputTokens: int(msg.Usage.OutputTokens),
		},
	}

	var textContent string
	var toolCalls []core.ToolCall

	for _, block := range msg.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			textContent += b.Text
		case anthropic.ToolUseBlock:
			inputJSON, err := b.Input.MarshalJSON()
			if err != nil {
				return resp, fmt.Errorf("anthropic: marshal tool input: %w", err)
			}
			toolCalls = append(toolCalls, core.ToolCall{
				ID:    b.ID,
				Name:  b.Name,
				Input: string(inputJSON),
			})
		}
	}

	resp.Message = core.Message{
		Role:      core.RoleAssistant,
		Content:   textContent,
		ToolCalls: toolCalls,
	}
	return resp, nil
}

// toCoreStopReason maps Anthropic stop reasons to core.StopReason.
func toCoreStopReason(reason string) core.StopReason {
	switch reason {
	case "end_turn":
		return core.StopReasonEndTurn
	case "tool_use":
		return core.StopReasonToolUse
	case "max_tokens":
		return core.StopReasonMaxTokens
	case "stop_sequence":
		return core.StopReasonStop
	default:
		return core.StopReasonEndTurn
	}
}

// toAnthropicMessages converts core.Messages to Anthropic message params.
func toAnthropicMessages(msgs []core.Message) []anthropic.MessageParam {
	var out []anthropic.MessageParam
	for _, m := range msgs {
		switch m.Role {
		case core.RoleSystem:
			// System messages are passed separately in the API call
			continue
		case core.RoleUser:
			out = append(out, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case core.RoleAssistant:
			var blocks []anthropic.ContentBlockParamUnion
			if m.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(m.Content))
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Name, tc.Input))
			}
			if len(blocks) > 0 {
				out = append(out, anthropic.NewAssistantMessage(blocks...))
			}
		case core.RoleTool:
			// Tool results go as user messages with tool_result content
			out = append(out, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(m.ToolCallID, m.Content, false),
			))
		}
	}
	return out
}

// toAnthropicTools converts core.ToolDefinitions to Anthropic tool params.
func toAnthropicTools(defs []core.ToolDefinition) []anthropic.ToolUnionParam {
	if len(defs) == 0 {
		return nil
	}
	out := make([]anthropic.ToolUnionParam, len(defs))
	for i, d := range defs {
		tp := anthropic.ToolParam{
			Name:        d.Name,
			Description: anthropic.String(d.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: d.InputSchema,
			},
		}
		out[i] = anthropic.ToolUnionParam{OfTool: &tp}
	}
	return out
}

// extractSystemPrompt finds the system message content from a message list.
func extractSystemPrompt(msgs []core.Message, optSystem string) string {
	if optSystem != "" {
		return optSystem
	}
	for _, m := range msgs {
		if m.Role == core.RoleSystem {
			return m.Content
		}
	}
	return ""
}
