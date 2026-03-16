package anthropic

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/lioarce01/chainforge/pkg/core"
)

// Provider wraps the Anthropic SDK as a core.Provider.
type Provider struct {
	client *anthropic.Client
}

// New creates a new Anthropic provider with the given API key.
func New(apiKey string) *Provider {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Provider{client: &client}
}

// NewWithClient creates a provider with an existing Anthropic client (useful for testing).
func NewWithClient(client *anthropic.Client) *Provider {
	return &Provider{client: client}
}

func (p *Provider) Name() string { return "anthropic" }

func (p *Provider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	systemPrompt := extractSystemPrompt(req.Messages, req.Options.SystemPrompt)
	msgs := toAnthropicMessages(req.Messages)
	anthropicTools := toAnthropicTools(req.Tools)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: int64(req.Options.MaxTokens),
		Messages:  msgs,
	}

	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	if len(anthropicTools) > 0 {
		params.Tools = anthropicTools
	}

	if req.Options.Temperature != 0 {
		params.Temperature = anthropic.Float(req.Options.Temperature)
	}

	msg, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return core.ChatResponse{}, &core.ProviderError{
			Provider: "anthropic",
			Err:      err,
		}
	}

	resp, err := toCoreResponse(*msg)
	if err != nil {
		return core.ChatResponse{}, fmt.Errorf("anthropic: translate response: %w", err)
	}
	return resp, nil
}

func (p *Provider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	systemPrompt := extractSystemPrompt(req.Messages, req.Options.SystemPrompt)
	msgs := toAnthropicMessages(req.Messages)
	anthropicTools := toAnthropicTools(req.Tools)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: int64(req.Options.MaxTokens),
		Messages:  msgs,
	}

	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	if len(anthropicTools) > 0 {
		params.Tools = anthropicTools
	}

	if req.Options.Temperature != 0 {
		params.Temperature = anthropic.Float(req.Options.Temperature)
	}

	stream := p.client.Messages.NewStreaming(ctx, params)

	ch := make(chan core.StreamEvent, 16)
	go func() {
		defer close(ch)

		// Track tool call accumulation per index
		toolInputs := make(map[int]string)
		toolMeta := make(map[int]struct{ id, name string })

		for stream.Next() {
			event := stream.Current()
			switch e := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				switch d := e.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					ch <- core.StreamEvent{
						Type:      core.StreamEventText,
						TextDelta: d.Text,
					}
				case anthropic.InputJSONDelta:
					idx := int(e.Index)
					toolInputs[idx] += d.PartialJSON
				}
			case anthropic.ContentBlockStartEvent:
				if tb, ok := e.ContentBlock.AsAny().(anthropic.ToolUseBlock); ok {
					idx := int(e.Index)
					toolMeta[idx] = struct{ id, name string }{id: tb.ID, name: tb.Name}
				}
			case anthropic.ContentBlockStopEvent:
				idx := int(e.Index)
				if meta, ok := toolMeta[idx]; ok {
					tc := core.ToolCall{
						ID:    meta.id,
						Name:  meta.name,
						Input: toolInputs[idx],
					}
					ch <- core.StreamEvent{
						Type:     core.StreamEventToolCall,
						ToolCall: &tc,
					}
					delete(toolMeta, idx)
					delete(toolInputs, idx)
				}
			case anthropic.MessageDeltaEvent:
				usage := core.Usage{
					OutputTokens: int(e.Usage.OutputTokens),
				}
				ch <- core.StreamEvent{
					Type:       core.StreamEventDone,
					StopReason: toCoreStopReason(string(e.Delta.StopReason)),
					Usage:      &usage,
				}
			}
		}

		if err := stream.Err(); err != nil {
			ch <- core.StreamEvent{
				Type:  core.StreamEventError,
				Error: &core.ProviderError{Provider: "anthropic", Err: err},
			}
		}
	}()

	return ch, nil
}
