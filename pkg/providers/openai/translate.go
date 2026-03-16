package openai

import (
	"encoding/json"
	"fmt"

	gogpt "github.com/sashabaranov/go-openai"
	"github.com/lioarce01/chainforge/pkg/core"
)

// toCoreResponse converts an OpenAI ChatCompletionResponse to core.ChatResponse.
func toCoreResponse(resp gogpt.ChatCompletionResponse) (core.ChatResponse, error) {
	if len(resp.Choices) == 0 {
		return core.ChatResponse{}, fmt.Errorf("openai: empty choices")
	}

	choice := resp.Choices[0]
	msg := choice.Message

	var toolCalls []core.ToolCall
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, core.ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: tc.Function.Arguments,
		})
	}

	return core.ChatResponse{
		Message: core.Message{
			Role:      core.RoleAssistant,
			Content:   msg.Content,
			ToolCalls: toolCalls,
		},
		StopReason: toCoreStopReason(string(choice.FinishReason)),
		Usage: core.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}, nil
}

// toCoreStopReason maps OpenAI finish reasons to core.StopReason.
func toCoreStopReason(reason string) core.StopReason {
	switch reason {
	case "stop":
		return core.StopReasonEndTurn
	case "tool_calls":
		return core.StopReasonToolUse
	case "length":
		return core.StopReasonMaxTokens
	default:
		return core.StopReasonEndTurn
	}
}

// toOpenAIMessages converts core.Messages to OpenAI ChatCompletionMessages.
func toOpenAIMessages(msgs []core.Message) []gogpt.ChatCompletionMessage {
	var out []gogpt.ChatCompletionMessage
	for _, m := range msgs {
		switch m.Role {
		case core.RoleSystem:
			out = append(out, gogpt.ChatCompletionMessage{
				Role:    gogpt.ChatMessageRoleSystem,
				Content: m.Content,
			})
		case core.RoleUser:
			out = append(out, gogpt.ChatCompletionMessage{
				Role:    gogpt.ChatMessageRoleUser,
				Content: m.Content,
			})
		case core.RoleAssistant:
			msg := gogpt.ChatCompletionMessage{
				Role:    gogpt.ChatMessageRoleAssistant,
				Content: m.Content,
			}
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, gogpt.ToolCall{
					ID:   tc.ID,
					Type: gogpt.ToolTypeFunction,
					Function: gogpt.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Input,
					},
				})
			}
			out = append(out, msg)
		case core.RoleTool:
			out = append(out, gogpt.ChatCompletionMessage{
				Role:       gogpt.ChatMessageRoleTool,
				Content:    m.Content,
				ToolCallID: m.ToolCallID,
			})
		}
	}
	return out
}

// toOpenAITools converts core.ToolDefinitions to OpenAI tool definitions.
func toOpenAITools(defs []core.ToolDefinition) []gogpt.Tool {
	if len(defs) == 0 {
		return nil
	}
	out := make([]gogpt.Tool, len(defs))
	for i, d := range defs {
		var params map[string]interface{}
		_ = json.Unmarshal(d.InputSchema, &params)
		out[i] = gogpt.Tool{
			Type: gogpt.ToolTypeFunction,
			Function: &gogpt.FunctionDefinition{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  params,
			},
		}
	}
	return out
}
