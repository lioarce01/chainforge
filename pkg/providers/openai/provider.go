package openai

import (
	"context"
	"fmt"
	"io"

	gogpt "github.com/sashabaranov/go-openai"
	"github.com/lioarce01/chainforge/pkg/core"
)

// Provider wraps the go-openai SDK as a core.Provider.
type Provider struct {
	client  *gogpt.Client
	name    string
	baseURL string
}

// New creates an OpenAI provider with the given API key.
func New(apiKey string) *Provider {
	client := gogpt.NewClient(apiKey)
	return &Provider{client: client, name: "openai"}
}

// NewWithBaseURL creates an OpenAI-compatible provider with a custom base URL (e.g. for Kimi, local servers).
func NewWithBaseURL(apiKey, baseURL, name string) *Provider {
	cfg := gogpt.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	client := gogpt.NewClientWithConfig(cfg)
	return &Provider{client: client, name: name, baseURL: baseURL}
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	msgs := toOpenAIMessages(req.Messages)
	oaiTools := toOpenAITools(req.Tools)

	chatReq := gogpt.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   req.Options.MaxTokens,
		Temperature: float32(req.Options.Temperature),
	}
	if len(oaiTools) > 0 {
		chatReq.Tools = oaiTools
	}

	resp, err := p.client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return core.ChatResponse{}, &core.ProviderError{
			Provider: p.name,
			Err:      err,
		}
	}

	out, err := toCoreResponse(resp)
	if err != nil {
		return core.ChatResponse{}, fmt.Errorf("openai: translate response: %w", err)
	}
	return out, nil
}

func (p *Provider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	msgs := toOpenAIMessages(req.Messages)
	oaiTools := toOpenAITools(req.Tools)

	chatReq := gogpt.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   req.Options.MaxTokens,
		Temperature: float32(req.Options.Temperature),
		Stream:      true,
	}
	if len(oaiTools) > 0 {
		chatReq.Tools = oaiTools
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, chatReq)
	if err != nil {
		return nil, &core.ProviderError{Provider: p.name, Err: err}
	}

	ch := make(chan core.StreamEvent, 16)
	go func() {
		defer close(ch)
		defer stream.Close()

		// Accumulate tool call arguments by index
		toolInputs := make(map[int]string)
		toolMeta := make(map[int]struct{ id, name string })

		for {
			delta, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				ch <- core.StreamEvent{
					Type:  core.StreamEventError,
					Error: &core.ProviderError{Provider: p.name, Err: err},
				}
				return
			}

			if len(delta.Choices) == 0 {
				continue
			}

			choice := delta.Choices[0]
			msg := choice.Delta

			if msg.Content != "" {
				ch <- core.StreamEvent{
					Type:      core.StreamEventText,
					TextDelta: msg.Content,
				}
			}

			for _, tc := range msg.ToolCalls {
				var idx int
				if tc.Index != nil {
					idx = int(*tc.Index)
				}
				if tc.ID != "" {
					toolMeta[idx] = struct{ id, name string }{id: tc.ID, name: tc.Function.Name}
				}
				toolInputs[idx] += tc.Function.Arguments
			}

			if choice.FinishReason != "" {
				// Emit accumulated tool calls
				for idx, meta := range toolMeta {
					tc := core.ToolCall{
						ID:    meta.id,
						Name:  meta.name,
						Input: toolInputs[idx],
					}
					ch <- core.StreamEvent{
						Type:     core.StreamEventToolCall,
						ToolCall: &tc,
					}
				}
				ch <- core.StreamEvent{
					Type:       core.StreamEventDone,
					StopReason: toCoreStopReason(string(choice.FinishReason)),
				}
				return
			}
		}
	}()

	return ch, nil
}
