//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
	"github.com/lioarce01/chainforge/pkg/orchestrator"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
)

// --- Sequential ---

func TestOpenRouter_Sequential_TwoStep(t *testing.T) {
	// Step 1: extract the city name from the sentence.
	// Step 2: given a city name, return its country.
	step1 := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("Extract the city name from the user message. Reply with only the city name, nothing else."),
	)
	step2 := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("Reply with only the country name of the given city, nothing else."),
	)

	result, err := orchestrator.Sequential(context.Background(), "or-seq",
		"I visited Tokyo last summer.",
		orchestrator.StepOf("extract", step1),
		orchestrator.StepOf("country", step2),
	)
	if err != nil {
		t.Fatalf("Sequential: %v", err)
	}
	if !strings.Contains(strings.ToLower(result), "japan") {
		t.Errorf("expected Japan in result, got: %s", result)
	}
	t.Logf("result: %s", result)
}

func TestOpenRouter_Sequential_TemplateInterpolation(t *testing.T) {
	// Step 1 extracts a city from the input.
	// Step 2 receives {{.previous}} (the city) via template and returns its country.
	step1 := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("Extract the city name from the message. Reply with only the city name, nothing else."),
	)
	step2 := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("Reply with only the country name of the given city, nothing else."),
	)

	result, err := orchestrator.Sequential(context.Background(), "or-seq-tmpl",
		"The Eiffel Tower is in Paris.",
		orchestrator.StepOf("city", step1),
		orchestrator.StepOf("country", step2, "What country is {{.previous}} in?"),
	)
	if err != nil {
		t.Fatalf("Sequential: %v", err)
	}
	if !strings.Contains(strings.ToLower(result), "france") {
		t.Errorf("expected France in result, got: %s", result)
	}
	t.Logf("result: %s", result)
}

// --- Parallel ---

func TestOpenRouter_Parallel_ConcurrentAgents(t *testing.T) {
	capitalAgent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("Reply with only the capital city name, nothing else."),
	)
	colorAgent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("Reply with only the color name, nothing else."),
	)

	results, err := orchestrator.Parallel(context.Background(), "or-par",
		orchestrator.FanOf("capital", capitalAgent, "What is the capital of France?"),
		orchestrator.FanOf("color", colorAgent, "What color is the sky on a clear day?"),
	)
	if err != nil {
		t.Fatalf("Parallel: %v", err)
	}
	if results.FirstError() != nil {
		t.Fatalf("branch error: %v", results.FirstError())
	}

	capital, ok := results.Get("capital")
	if !ok {
		t.Fatal("capital branch missing")
	}
	color, ok := results.Get("color")
	if !ok {
		t.Fatal("color branch missing")
	}

	if !strings.Contains(strings.ToLower(capital.Output), "paris") {
		t.Errorf("capital = %q, expected Paris", capital.Output)
	}
	if !strings.Contains(strings.ToLower(color.Output), "blue") {
		t.Errorf("color = %q, expected blue", color.Output)
	}
	t.Logf("capital=%s color=%s", capital.Output, color.Output)
}

func TestOpenRouter_Parallel_AllOutputsReturned(t *testing.T) {
	agent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("Reply with exactly: ok"),
	)

	results, err := orchestrator.Parallel(context.Background(), "or-par-all",
		orchestrator.FanOf("a", agent, "hi"),
		orchestrator.FanOf("b", agent, "hi"),
		orchestrator.FanOf("c", agent, "hi"),
	)
	if err != nil {
		t.Fatalf("Parallel: %v", err)
	}
	outputs := results.Outputs()
	if len(outputs) != 3 {
		t.Errorf("expected 3 outputs, got %d", len(outputs))
	}
	t.Logf("outputs: %v", outputs)
}

// --- LLMRouter ---

func TestOpenRouter_LLMRouter_RoutesCorrectly(t *testing.T) {
	supervisor := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt(
			"You are a routing agent. You will be given a list of routes and a user message. "+
				"Reply with ONLY the route name that best matches, nothing else.",
		),
		chainforge.WithMaxTokens(16),
	)

	mathAgent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a math assistant. Always use the calculator tool for arithmetic."),
	)
	triviaAgent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a trivia assistant. Answer in one sentence."),
	)

	router := orchestrator.NewLLMRouter(supervisor,
		orchestrator.RouteOf("math", "handles arithmetic calculations and math questions", mathAgent),
		orchestrator.RouteOf("trivia", "answers general knowledge and trivia questions", triviaAgent),
	).WithDefault("trivia")

	result, err := router.Route(context.Background(), "or-router", "What is the capital of Germany?")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty response")
	}
	if !strings.Contains(strings.ToLower(result), "berlin") {
		t.Errorf("expected Berlin in trivia answer, got: %s", result)
	}
	t.Logf("result: %s", result)
}

// --- Conditional ---

func TestOpenRouter_Conditional_TrueBranch(t *testing.T) {
	// The real assertion is routing: predicate(true) → if-agent is invoked, else-agent is not.
	var ifCalled, elseCalled bool

	ifAgent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a helpful assistant."),
		chainforge.WithDebugHandler(func(_ context.Context, ev chainforge.DebugEvent) {
			if ev.Kind == chainforge.DebugLLMRequest {
				ifCalled = true
			}
		}),
	)
	elseAgent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a helpful assistant."),
		chainforge.WithDebugHandler(func(_ context.Context, ev chainforge.DebugEvent) {
			if ev.Kind == chainforge.DebugLLMRequest {
				elseCalled = true
			}
		}),
	)

	result, err := orchestrator.Conditional(
		context.Background(), "or-cond-true",
		"The sky is blue.",
		func(input string) bool { return strings.Contains(strings.ToLower(input), "blue") },
		ifAgent,
		elseAgent,
	)
	if err != nil {
		t.Fatalf("Conditional: %v", err)
	}
	if !ifCalled {
		t.Error("if-agent was not called (predicate should be true)")
	}
	if elseCalled {
		t.Error("else-agent should not have been called")
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	t.Logf("result: %s", result)
}

func TestOpenRouter_Conditional_FalseBranch(t *testing.T) {
	// The real assertion is routing: predicate(false) → else-agent is invoked, if-agent is not.
	var ifCalled, elseCalled bool

	ifAgent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a helpful assistant."),
		chainforge.WithDebugHandler(func(_ context.Context, ev chainforge.DebugEvent) {
			if ev.Kind == chainforge.DebugLLMRequest {
				ifCalled = true
			}
		}),
	)
	elseAgent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a helpful assistant."),
		chainforge.WithDebugHandler(func(_ context.Context, ev chainforge.DebugEvent) {
			if ev.Kind == chainforge.DebugLLMRequest {
				elseCalled = true
			}
		}),
	)

	result, err := orchestrator.Conditional(
		context.Background(), "or-cond-false",
		"The sky is red.",
		func(input string) bool { return strings.Contains(strings.ToLower(input), "blue") },
		ifAgent,
		elseAgent,
	)
	if err != nil {
		t.Fatalf("Conditional: %v", err)
	}
	if ifCalled {
		t.Error("if-agent should not have been called")
	}
	if !elseCalled {
		t.Error("else-agent was not called (predicate should be false)")
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	t.Logf("result: %s", result)
}

// --- Loop ---

func TestOpenRouter_Loop_RunsNTimes(t *testing.T) {
	agent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("Reply with exactly: ok"),
	)

	iterCount := 0
	result, err := orchestrator.Loop(
		context.Background(), "or-loop",
		"start",
		agent,
		func(iter int, output string) bool {
			iterCount++
			return iter < 3 // run 3 iterations then stop
		},
		10,
	)
	if err != nil {
		t.Fatalf("Loop: %v", err)
	}
	if iterCount < 3 {
		t.Errorf("expected at least 3 condition checks, got %d", iterCount)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	t.Logf("iterations=%d result=%s", iterCount, result)
}

// --- WithHistorySummarizer ---

func TestOpenRouter_HistorySummarizer_CompressesHistory(t *testing.T) {
	mem := inmemory.New()

	summarizer, err := chainforge.NewAgent(
		chainforge.WithProvider(openai.NewWithBaseURL(openRouterKey(t), openRouterBaseURL, "openrouter")),
		chainforge.WithModel(openRouterModel),
		chainforge.WithSystemPrompt("Summarize the conversation concisely. Preserve key facts."),
		chainforge.WithMaxTokens(128),
		chainforge.WithRunTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("NewAgent summarizer: %v", err)
	}

	agent := newOpenRouterAgent(t,
		chainforge.WithSystemPrompt("You are a helpful assistant. Be concise."),
		chainforge.WithMemory(mem),
		chainforge.WithMaxHistory(3),
		chainforge.WithHistorySummarizer(summarizer),
		chainforge.WithMaxTokens(128),
	)

	ctx := context.Background()
	// Accumulate enough turns to trigger summarization.
	for i := 0; i < 5; i++ {
		if _, err := agent.Run(ctx, "or-summ", "Say ok."); err != nil {
			t.Fatalf("turn %d: %v", i, err)
		}
	}

	msgs, err := mem.Get(ctx, "or-summ")
	if err != nil {
		t.Fatalf("mem.Get: %v", err)
	}

	hasSummary := false
	for _, m := range msgs {
		if strings.HasPrefix(m.Content, "[Summary:") {
			hasSummary = true
			break
		}
	}
	if !hasSummary {
		t.Errorf("expected a [Summary: ...] message in history, got %d messages: %v", len(msgs), msgs)
	}
	t.Logf("history len=%d hasSummary=%v", len(msgs), hasSummary)
}
