package tests

import (
	"errors"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
)

func TestWithAnthropic_NonNilAgent(t *testing.T) {
	a, err := chainforge.NewAgent(
		chainforge.WithAnthropic("sk-ant-fake", "claude-sonnet-4-6"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if a == nil {
		t.Error("expected non-nil agent")
	}
}

func TestWithOpenAI_NonNilAgent(t *testing.T) {
	a, err := chainforge.NewAgent(
		chainforge.WithOpenAI("sk-fake", "gpt-4o"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if a == nil {
		t.Error("expected non-nil agent")
	}
}

func TestWithOllama_NonNilAgent(t *testing.T) {
	a, err := chainforge.NewAgent(
		chainforge.WithOllama("", "llama3"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if a == nil {
		t.Error("expected non-nil agent")
	}
}

func TestWithOpenAICompatible_NonNilAgent(t *testing.T) {
	a, err := chainforge.NewAgent(
		chainforge.WithOpenAICompatible("key", "http://localhost:8080/v1", "localai", "mistral"),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if a == nil {
		t.Error("expected non-nil agent")
	}
}

func TestWithAnthropic_EmptyModel_ErrNoModel(t *testing.T) {
	_, err := chainforge.NewAgent(
		chainforge.WithAnthropic("sk-ant-fake", ""),
	)
	if err == nil {
		t.Fatal("expected error for empty model")
	}
	if !errors.Is(err, core.ErrNoModel) {
		t.Errorf("error = %v, want ErrNoModel", err)
	}
}

func TestShortcut_OverriddenByLaterOptions(t *testing.T) {
	// WithAnthropic sets provider+model; later WithModel overrides model
	mock := NewMockProvider(EndTurnResponse("ok"))
	a, err := chainforge.NewAgent(
		chainforge.WithAnthropic("sk-ant-fake", "claude-haiku-4-5-20251001"),
		chainforge.WithProvider(mock), // overrides provider
		chainforge.WithModel("mock"),  // overrides model
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if a == nil {
		t.Error("expected non-nil agent")
	}
}
