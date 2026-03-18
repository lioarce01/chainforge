package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/tools"
)

// Retriever retrieves relevant documents for a query.
type Retriever interface {
	Retrieve(ctx context.Context, query string, topK int) ([]Document, error)
}

// Option configures RAG operations such as NewRetrieverTool and WithRetriever.
type Option func(*ragOptions)

type ragOptions struct {
	topK int
}

func defaultOptions() ragOptions {
	return ragOptions{topK: 5}
}

// WithTopK sets the number of documents to retrieve (default: 5).
func WithTopK(n int) Option {
	return func(o *ragOptions) { o.topK = n }
}

// RetrieveOption is an alias for Option for use with chainforge.WithRetriever.
type RetrieveOption = Option

// ToolOption is an alias for Option for use with NewRetrieverTool.
type ToolOption = Option

// ApplyOptions merges a slice of Options into a resolved ragOptions value.
// Exported so callers (e.g. chainforge.WithRetriever) can inspect resolved values.
func ApplyOptions(opts ...Option) *ragOptions {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return &o
}

// TopK returns the resolved top-K value.
func (o *ragOptions) TopK() int { return o.topK }

// retrieverToolInput is the JSON input the LLM sends when calling the retriever tool.
type retrieverToolInput struct {
	Query string `json:"query"`
}

// NewRetrieverTool wraps a Retriever as a core.Tool so the LLM can call
// retrieval explicitly rather than having it injected automatically.
//
//	tool := rag.NewRetrieverTool(retriever, rag.WithTopK(5))
//	agent, _ := chainforge.NewAgent(chainforge.WithTools(tool), ...)
func NewRetrieverTool(r Retriever, opts ...ToolOption) core.Tool {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	topK := o.topK

	schema := tools.NewSchema().
		AddString("query", "The search query to retrieve relevant documents for", true).
		MustBuild()

	fn := func(ctx context.Context, input string) (string, error) {
		var in retrieverToolInput
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return "", fmt.Errorf("retriever_tool: invalid input: %w", err)
		}
		if in.Query == "" {
			return "", fmt.Errorf("retriever_tool: query is required")
		}
		docs, err := r.Retrieve(ctx, in.Query, topK)
		if err != nil {
			return "", fmt.Errorf("retriever_tool: retrieve: %w", err)
		}
		if len(docs) == 0 {
			return "No relevant documents found.", nil
		}
		return FormatContext(docs), nil
	}

	t, _ := tools.Func("retriever_tool", "Retrieve relevant documents from the knowledge base for a given query.", schema, fn)
	return t
}

// FormatContext formats a slice of documents into a compact string suitable for
// appending to a system prompt or returning as a tool result.
func FormatContext(docs []Document) string {
	var sb strings.Builder
	for i, d := range docs {
		fmt.Fprintf(&sb, "[%d] Source: %s\n%s", i+1, d.Source, d.Content)
		if i < len(docs)-1 {
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}
