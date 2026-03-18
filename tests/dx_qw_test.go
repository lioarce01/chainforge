package tests

// Tests for Phase 1 Quick Wins:
// QW-2: initErr surfacing from WithGemini
// QW-3: WithHistorySummarizer without WithMaxHistory
// QW-4: ParallelResults.Get / FirstError / Outputs
// QW-5: StepOf without template

import (
	"context"
	"errors"
	"strings"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/orchestrator"
)

// --- QW-2: initErr from provider shorthand ---

func TestWithGemini_BadKey_SurfacesError(t *testing.T) {
	// gemini.New returns an error for an empty API key.
	_, err := chainforge.NewAgent(
		chainforge.WithGemini("", "gemini-2.0-flash"),
	)
	if err == nil {
		t.Fatal("expected error for empty Gemini API key, got nil")
	}
	if !strings.Contains(err.Error(), "WithGemini") {
		t.Errorf("error should mention WithGemini, got: %v", err)
	}
}

// --- QW-3: WithHistorySummarizer without WithMaxHistory ---

func TestValidateConfig_HistorySummarizer_RequiresMaxHistory(t *testing.T) {
	summarizer, _ := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("summary"))),
		chainforge.WithModel("mock"),
	)
	_, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(EndTurnResponse("ok"))),
		chainforge.WithModel("mock"),
		chainforge.WithHistorySummarizer(summarizer),
		// WithMaxHistory intentionally omitted
	)
	if err == nil {
		t.Fatal("expected error when WithHistorySummarizer set without WithMaxHistory")
	}
}

// --- QW-4: ParallelResults ---

func makeParallelResults() orchestrator.ParallelResults {
	return orchestrator.ParallelResults{
		{Name: "a", Output: "result-a"},
		{Name: "b", Output: "result-b", Error: errors.New("b failed")},
		{Name: "c", Output: "result-c"},
	}
}

func TestParallelResults_Get_Found(t *testing.T) {
	results := makeParallelResults()
	r, ok := results.Get("a")
	if !ok {
		t.Fatal("Get(\"a\") returned not found")
	}
	if r.Output != "result-a" {
		t.Errorf("Output = %q, want %q", r.Output, "result-a")
	}
}

func TestParallelResults_Get_NotFound(t *testing.T) {
	results := makeParallelResults()
	_, ok := results.Get("nonexistent")
	if ok {
		t.Error("Get(\"nonexistent\") should return not found")
	}
}

func TestParallelResults_FirstError_ReturnsFirst(t *testing.T) {
	results := makeParallelResults()
	err := results.FirstError()
	if err == nil {
		t.Fatal("FirstError should return non-nil")
	}
	if err.Error() != "b failed" {
		t.Errorf("FirstError = %q, want %q", err.Error(), "b failed")
	}
}

func TestParallelResults_FirstError_NilWhenAllSucceed(t *testing.T) {
	results := orchestrator.ParallelResults{
		{Name: "a", Output: "ok"},
		{Name: "b", Output: "ok"},
	}
	if err := results.FirstError(); err != nil {
		t.Errorf("FirstError should be nil, got %v", err)
	}
}

func TestParallelResults_Outputs_OnlySuccessful(t *testing.T) {
	results := makeParallelResults()
	outputs := results.Outputs()
	if len(outputs) != 2 {
		t.Fatalf("Outputs len = %d, want 2 (b failed)", len(outputs))
	}
	if outputs["a"] != "result-a" {
		t.Errorf("outputs[a] = %q, want %q", outputs["a"], "result-a")
	}
	if outputs["c"] != "result-c" {
		t.Errorf("outputs[c] = %q, want %q", outputs["c"], "result-c")
	}
	if _, ok := outputs["b"]; ok {
		t.Error("outputs should not include failed branch b")
	}
}

func TestParallelResults_RangeStillWorks(t *testing.T) {
	// Verify backwards compat — ParallelResults is []ParallelResult
	results := makeParallelResults()
	count := 0
	for _, r := range results {
		_ = r
		count++
	}
	if count != 3 {
		t.Errorf("range yielded %d items, want 3", count)
	}
}

// --- QW-5: StepOf without template ---

func TestStepOf_NoTemplate_PassesPreviousVerbatim(t *testing.T) {
	ctx := context.Background()

	step1Output := "step1-output"
	step2ReceivedInput := ""

	p1 := NewMockProvider(EndTurnResponse(step1Output))
	p2 := NewMockProvider(EndTurnResponse("done"))

	agent1, _ := chainforge.NewAgent(chainforge.WithProvider(p1), chainforge.WithModel("mock"))
	agent2, _ := chainforge.NewAgent(chainforge.WithProvider(p2), chainforge.WithModel("mock"))

	_, err := orchestrator.Sequential(ctx, "sess",
		"original-input",
		orchestrator.StepOf("step1", agent1), // no template
		orchestrator.StepOf("step2", agent2), // no template
	)
	if err != nil {
		t.Fatalf("Sequential: %v", err)
	}

	// step2 should have received step1's output as its message
	req := p2.Calls()[0].Request
	found := false
	for _, m := range req.Messages {
		if strings.Contains(m.Content, step1Output) {
			found = true
			break
		}
	}
	_ = step2ReceivedInput
	if !found {
		t.Errorf("step2 did not receive step1 output %q in its messages", step1Output)
	}
}

func TestStepOf_WithTemplate_InterpolatesCorrectly(t *testing.T) {
	ctx := context.Background()

	p1 := NewMockProvider(EndTurnResponse("topic-result"))
	p2 := NewMockProvider(EndTurnResponse("done"))

	agent1, _ := chainforge.NewAgent(chainforge.WithProvider(p1), chainforge.WithModel("mock"))
	agent2, _ := chainforge.NewAgent(chainforge.WithProvider(p2), chainforge.WithModel("mock"))

	_, err := orchestrator.Sequential(ctx, "sess",
		"my-input",
		orchestrator.StepOf("step1", agent1, "Research: {{.input}}"),
		orchestrator.StepOf("step2", agent2, "Write about: {{.previous}}"),
	)
	if err != nil {
		t.Fatalf("Sequential: %v", err)
	}

	// step1 should have received "Research: my-input"
	req1 := p1.Calls()[0].Request
	found1 := false
	for _, m := range req1.Messages {
		if strings.Contains(m.Content, "Research: my-input") {
			found1 = true
			break
		}
	}
	if !found1 {
		t.Error("step1 did not receive interpolated template with original input")
	}
}
