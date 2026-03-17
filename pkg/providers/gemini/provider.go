// Package gemini provides a Google Gemini LLM adapter for core.Provider.
package gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lioarce01/chainforge/pkg/core"
	"google.golang.org/genai"
)

// Compile-time guard
var _ core.Provider = (*Provider)(nil)

// Provider wraps the Google genai client as a core.Provider.
type Provider struct {
	client    *genai.Client
	modelName string
}

// New creates a new Gemini provider with the given API key and model name.
func New(apiKey, modelName string) (*Provider, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: create client: %w", err)
	}
	return &Provider{client: client, modelName: modelName}, nil
}

func (p *Provider) Name() string { return "gemini" }

// Chat sends a request to Gemini and returns the full response.
func (p *Provider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	contents, systemInstruction := toGeminiContents(req.Messages)
	cfg := buildConfig(req, systemInstruction)

	resp, err := p.client.Models.GenerateContent(ctx, p.modelName, contents, cfg)
	if err != nil {
		return core.ChatResponse{}, fmt.Errorf("gemini: generate: %w", err)
	}
	return toCoreResponse(resp)
}

// ChatStream sends a request to Gemini and streams events.
func (p *Provider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	contents, systemInstruction := toGeminiContents(req.Messages)
	cfg := buildConfig(req, systemInstruction)

	ch := make(chan core.StreamEvent, 16)
	go func() {
		defer close(ch)

		stream := p.client.Models.GenerateContentStream(ctx, p.modelName, contents, cfg)
		for resp, err := range stream {
			if err != nil {
				ch <- core.StreamEvent{Type: core.StreamEventError, Error: fmt.Errorf("gemini: stream: %w", err)}
				return
			}
			if resp == nil || len(resp.Candidates) == 0 {
				continue
			}

			candidate := resp.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part.Text != "" {
						ch <- core.StreamEvent{Type: core.StreamEventText, TextDelta: part.Text}
					}
					if part.FunctionCall != nil {
						input, _ := json.Marshal(part.FunctionCall.Args)
						ch <- core.StreamEvent{
							Type: core.StreamEventToolCall,
							ToolCall: &core.ToolCall{
								ID:    part.FunctionCall.ID,
								Name:  part.FunctionCall.Name,
								Input: string(input),
							},
						}
					}
				}
			}

			// Emit done on each chunk that has a finish reason
			if candidate.FinishReason != "" && candidate.FinishReason != "FINISH_REASON_UNSPECIFIED" {
				var usage core.Usage
				if resp.UsageMetadata != nil {
					usage = core.Usage{
						InputTokens:  int(resp.UsageMetadata.PromptTokenCount),
						OutputTokens: int(resp.UsageMetadata.CandidatesTokenCount),
					}
				}
				ch <- core.StreamEvent{
					Type:       core.StreamEventDone,
					StopReason: toStopReason(candidate.FinishReason),
					Usage:      &usage,
				}
			}
		}
	}()

	return ch, nil
}

// buildConfig creates a GenerateContentConfig from a ChatRequest.
func buildConfig(req core.ChatRequest, systemInstruction *genai.Content) *genai.GenerateContentConfig {
	cfg := &genai.GenerateContentConfig{
		MaxOutputTokens:   int32(req.Options.MaxTokens),
		SystemInstruction: systemInstruction,
	}
	if req.Options.Temperature != 0 {
		t := float32(req.Options.Temperature)
		cfg.Temperature = &t
	}
	if len(req.Tools) > 0 {
		cfg.Tools = []*genai.Tool{toGeminiTool(req.Tools)}
	}
	return cfg
}

// toGeminiContents converts core messages to genai Contents and extracts the system instruction.
func toGeminiContents(msgs []core.Message) ([]*genai.Content, *genai.Content) {
	var contents []*genai.Content
	var systemInstruction *genai.Content

	for _, m := range msgs {
		switch m.Role {
		case core.RoleSystem:
			systemInstruction = &genai.Content{
				Parts: []*genai.Part{{Text: m.Content}},
			}
		case core.RoleUser:
			contents = append(contents, &genai.Content{
				Role:  genai.RoleUser,
				Parts: []*genai.Part{{Text: m.Content}},
			})
		case core.RoleAssistant:
			parts := []*genai.Part{}
			if m.Content != "" {
				parts = append(parts, &genai.Part{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var args map[string]any
				_ = json.Unmarshal([]byte(tc.Input), &args)
				parts = append(parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						ID:   tc.ID,
						Name: tc.Name,
						Args: args,
					},
				})
			}
			if len(parts) > 0 {
				contents = append(contents, &genai.Content{
					Role:  genai.RoleModel,
					Parts: parts,
				})
			}
		case core.RoleTool:
			contents = append(contents, &genai.Content{
				Role: genai.RoleUser,
				Parts: []*genai.Part{
					{
						FunctionResponse: &genai.FunctionResponse{
							ID:       m.ToolCallID,
							Name:     m.Name,
							Response: map[string]any{"output": m.Content},
						},
					},
				},
			})
		}
	}
	return contents, systemInstruction
}

// toGeminiTool converts core tool definitions to a single genai Tool.
func toGeminiTool(defs []core.ToolDefinition) *genai.Tool {
	fns := make([]*genai.FunctionDeclaration, 0, len(defs))
	for _, d := range defs {
		var schema any
		if len(d.InputSchema) > 0 {
			schema = json.RawMessage(d.InputSchema)
		}
		fns = append(fns, &genai.FunctionDeclaration{
			Name:                 d.Name,
			Description:          d.Description,
			ParametersJsonSchema: schema,
		})
	}
	return &genai.Tool{FunctionDeclarations: fns}
}

// toCoreResponse converts a Gemini response to a core.ChatResponse.
func toCoreResponse(resp *genai.GenerateContentResponse) (core.ChatResponse, error) {
	if len(resp.Candidates) == 0 {
		return core.ChatResponse{}, fmt.Errorf("gemini: no candidates in response")
	}

	candidate := resp.Candidates[0]
	var (
		text      string
		toolCalls []core.ToolCall
	)

	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				text += part.Text
			}
			if part.FunctionCall != nil {
				input, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, core.ToolCall{
					ID:    part.FunctionCall.ID,
					Name:  part.FunctionCall.Name,
					Input: string(input),
				})
			}
		}
	}

	var usage core.Usage
	if resp.UsageMetadata != nil {
		usage = core.Usage{
			InputTokens:  int(resp.UsageMetadata.PromptTokenCount),
			OutputTokens: int(resp.UsageMetadata.CandidatesTokenCount),
		}
	}

	msg := core.Message{
		Role:      core.RoleAssistant,
		Content:   text,
		ToolCalls: toolCalls,
	}

	return core.ChatResponse{
		Message:    msg,
		StopReason: toStopReason(candidate.FinishReason),
		Usage:      usage,
	}, nil
}

// toStopReason maps Gemini FinishReason to core.StopReason.
func toStopReason(reason genai.FinishReason) core.StopReason {
	switch reason {
	case genai.FinishReasonStop:
		return core.StopReasonEndTurn
	case genai.FinishReasonMaxTokens:
		return core.StopReasonMaxTokens
	default:
		if len(reason) > 0 {
			return core.StopReasonEndTurn
		}
		return ""
	}
}
