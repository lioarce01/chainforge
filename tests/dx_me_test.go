package tests

// Tests for Phase 2 Medium Effort:
// ME-1: tools.TypedFunc / MustTypedFunc
// ME-2: WithDebugHandler / PrettyPrintDebugHandler
// ME-4: validateConfig tool validation (duplicates, invalid names, bad schema)

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/tools"
)

// --- ME-1: TypedFunc ---

type calcInput struct {
	A int `json:"a" cf:"required,description=First operand"`
	B int `json:"b" cf:"required,description=Second operand"`
}

func TestTypedFunc_HappyPath(t *testing.T) {
	tool, err := tools.TypedFunc[calcInput]("add", "Add two numbers",
		func(_ context.Context, in calcInput) (string, error) {
			return strings.Repeat("x", in.A+in.B), nil
		})
	if err != nil {
		t.Fatalf("TypedFunc: %v", err)
	}

	out, err := tool.Call(context.Background(), `{"a":3,"b":4}`)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out != "xxxxxxx" {
		t.Errorf("output = %q, want %q", out, "xxxxxxx")
	}
}

func TestTypedFunc_SchemaGenerated(t *testing.T) {
	tool, err := tools.TypedFunc[calcInput]("add", "Add two numbers",
		func(_ context.Context, in calcInput) (string, error) { return "", nil })
	if err != nil {
		t.Fatalf("TypedFunc: %v", err)
	}

	def := tool.Definition()
	if def.Name != "add" {
		t.Errorf("Name = %q, want %q", def.Name, "add")
	}
	if !json.Valid(def.InputSchema) {
		t.Errorf("InputSchema is not valid JSON: %s", def.InputSchema)
	}
	if !strings.Contains(string(def.InputSchema), `"a"`) {
		t.Errorf("schema should contain field 'a', got: %s", def.InputSchema)
	}
}

func TestTypedFunc_InvalidJSON_ReturnsError(t *testing.T) {
	tool, _ := tools.TypedFunc[calcInput]("add", "desc",
		func(_ context.Context, in calcInput) (string, error) { return "ok", nil })

	_, err := tool.Call(context.Background(), `not json`)
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestTypedFunc_NonStructType_ReturnsError(t *testing.T) {
	// string is not a struct — TypedFunc should return an error
	_, err := tools.TypedFunc[string]("bad", "desc",
		func(_ context.Context, in string) (string, error) { return in, nil })
	if err == nil {
		t.Error("expected error when TInput is not a struct")
	}
}

func TestMustTypedFunc_PanicsOnNonStruct(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustTypedFunc should panic for non-struct TInput")
		}
	}()
	tools.MustTypedFunc[int]("bad", "desc",
		func(_ context.Context, in int) (string, error) { return "", nil })
}

func TestTypedFunc_IntegrationWithAgent(t *testing.T) {
	var gotInput calcInput

	tool := tools.MustTypedFunc[calcInput]("add", "Add two numbers",
		func(_ context.Context, in calcInput) (string, error) {
			gotInput = in
			return "7", nil
		})

	p := NewMockProvider(
		ToolUseResponse(core.ToolCall{Name: "add", Input: `{"a":3,"b":4}`}),
		EndTurnResponse("The answer is 7"),
	)

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithTools(tool),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), "s1", "what is 3+4?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotInput.A != 3 || gotInput.B != 4 {
		t.Errorf("got input %+v, want {A:3 B:4}", gotInput)
	}
}

// --- ME-2: WithDebugHandler ---

func TestWithDebugHandler_LLMEventsFireInOrder(t *testing.T) {
	var kinds []chainforge.DebugEventKind

	p := NewMockProvider(EndTurnResponse("hello"))
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithDebugHandler(func(_ context.Context, ev chainforge.DebugEvent) {
			kinds = append(kinds, ev.Kind)
		}),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), "s1", "hi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(kinds) < 2 {
		t.Fatalf("expected at least 2 debug events, got %d", len(kinds))
	}
	if kinds[0] != chainforge.DebugLLMRequest {
		t.Errorf("first event = %q, want DebugLLMRequest", kinds[0])
	}
	if kinds[1] != chainforge.DebugLLMResponse {
		t.Errorf("second event = %q, want DebugLLMResponse", kinds[1])
	}
}

func TestWithDebugHandler_ToolEventsFireForToolCall(t *testing.T) {
	var kinds []chainforge.DebugEventKind
	var toolCallSeen, toolResultSeen bool

	addTool := tools.MustTypedFunc[calcInput]("add", "add",
		func(_ context.Context, in calcInput) (string, error) { return "7", nil })

	p := NewMockProvider(
		ToolUseResponse(core.ToolCall{Name: "add", Input: `{"a":3,"b":4}`}),
		EndTurnResponse("done"),
	)

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithTools(addTool),
		chainforge.WithDebugHandler(func(_ context.Context, ev chainforge.DebugEvent) {
			kinds = append(kinds, ev.Kind)
			if ev.Kind == chainforge.DebugToolCall {
				toolCallSeen = true
				if ev.ToolCall == nil || ev.ToolCall.Name != "add" {
					t.Errorf("DebugToolCall: ToolCall = %v, want name=add", ev.ToolCall)
				}
			}
			if ev.Kind == chainforge.DebugToolResult {
				toolResultSeen = true
				if ev.ToolOutput != "7" {
					t.Errorf("DebugToolResult: ToolOutput = %q, want %q", ev.ToolOutput, "7")
				}
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), "s1", "add 3 and 4")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !toolCallSeen {
		t.Error("DebugToolCall event never fired")
	}
	if !toolResultSeen {
		t.Error("DebugToolResult event never fired")
	}
}

func TestWithDebugHandler_IterationFieldCorrect(t *testing.T) {
	var iterations []int

	p := NewMockProvider(
		ToolUseResponse(core.ToolCall{Name: "add", Input: `{"a":1,"b":2}`}),
		EndTurnResponse("done"),
	)
	addTool := tools.MustTypedFunc[calcInput]("add", "add",
		func(_ context.Context, in calcInput) (string, error) { return "3", nil })

	agent, _ := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithTools(addTool),
		chainforge.WithDebugHandler(func(_ context.Context, ev chainforge.DebugEvent) {
			if ev.Kind == chainforge.DebugLLMRequest {
				iterations = append(iterations, ev.Iteration)
			}
		}),
	)

	agent.Run(context.Background(), "s1", "go")

	if len(iterations) != 2 {
		t.Fatalf("expected 2 LLMRequest events (one per iteration), got %d", len(iterations))
	}
	if iterations[0] != 0 || iterations[1] != 1 {
		t.Errorf("iterations = %v, want [0 1]", iterations)
	}
}

func TestPrettyPrintDebugHandler_WritesToWriter(t *testing.T) {
	var buf bytes.Buffer

	p := NewMockProvider(EndTurnResponse("hello world"))
	agent, _ := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithDebugHandler(chainforge.PrettyPrintDebugHandler(&buf)),
	)

	agent.Run(context.Background(), "s1", "hi")

	out := buf.String()
	if !strings.Contains(out, "LLM") {
		t.Errorf("expected LLM in debug output, got: %s", out)
	}
}

// --- ME-4: validateConfig tool validation ---

func TestValidateConfig_DuplicateToolName_ReturnsError(t *testing.T) {
	t1, _ := tools.Func("search", "desc", nil,
		func(_ context.Context, _ string) (string, error) { return "", nil })
	t2, _ := tools.Func("search", "other desc", nil,
		func(_ context.Context, _ string) (string, error) { return "", nil })

	_, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider()),
		chainforge.WithModel("mock"),
		chainforge.WithTools(t1, t2),
	)
	if err == nil {
		t.Error("expected error for duplicate tool name")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error should mention duplicate, got: %v", err)
	}
}

func TestValidateConfig_InvalidToolName_ReturnsError(t *testing.T) {
	badTool, _ := tools.Func("bad name!", "desc", nil,
		func(_ context.Context, _ string) (string, error) { return "", nil })

	_, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider()),
		chainforge.WithModel("mock"),
		chainforge.WithTools(badTool),
	)
	if err == nil {
		t.Error("expected error for invalid tool name with spaces and !")
	}
	if !strings.Contains(err.Error(), "invalid name") {
		t.Errorf("error should mention invalid name, got: %v", err)
	}
}

func TestValidateConfig_ValidToolNames_Accepted(t *testing.T) {
	names := []string{"search", "get_weather", "fetch-data", "tool123"}
	toolList := make([]core.Tool, len(names))
	for i, name := range names {
		n := name
		toolList[i], _ = tools.Func(n, "desc", nil,
			func(_ context.Context, _ string) (string, error) { return "", nil })
	}

	_, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("ok"))),
		chainforge.WithModel("mock"),
		chainforge.WithTools(toolList...),
	)
	if err != nil {
		t.Errorf("valid tool names should be accepted, got: %v", err)
	}
}

func TestValidateConfig_InvalidJSONSchema_ReturnsError(t *testing.T) {
	badSchema := json.RawMessage(`{not valid json`)
	badTool, _ := tools.Func("my_tool", "desc", badSchema,
		func(_ context.Context, _ string) (string, error) { return "", nil })

	_, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider()),
		chainforge.WithModel("mock"),
		chainforge.WithTools(badTool),
	)
	if err == nil {
		t.Error("expected error for invalid JSON schema")
	}
	if !strings.Contains(err.Error(), "invalid JSON schema") {
		t.Errorf("error should mention invalid JSON schema, got: %v", err)
	}
}
