package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/lioarce01/chainforge/pkg/core"
)

func TestNewToolError_FieldsSet(t *testing.T) {
	inner := errors.New("boom")
	te := core.NewToolError("my_tool", inner)
	if te.ToolName != "my_tool" {
		t.Errorf("ToolName = %q, want %q", te.ToolName, "my_tool")
	}
	if te.Err != inner {
		t.Errorf("Err = %v, want %v", te.Err, inner)
	}
}

func TestNewToolError_ErrorString(t *testing.T) {
	te := core.NewToolError("calc", errors.New("div by zero"))
	got := te.Error()
	if !strings.Contains(got, "calc") || !strings.Contains(got, "div by zero") {
		t.Errorf("Error() = %q, want tool name and inner error", got)
	}
}

func TestNewToolError_ErrorsAs(t *testing.T) {
	wrapped := errors.New("inner")
	err := error(core.NewToolError("t", wrapped))
	var te *core.ToolError
	if !errors.As(err, &te) {
		t.Fatal("errors.As failed for *ToolError")
	}
	if te.ToolName != "t" {
		t.Errorf("ToolName via As = %q, want %q", te.ToolName, "t")
	}
}

func TestNewToolError_ErrorsIs_Unwrap(t *testing.T) {
	sentinel := errors.New("sentinel")
	te := core.NewToolError("tool", sentinel)
	if !errors.Is(te, sentinel) {
		t.Error("errors.Is should match via Unwrap")
	}
}

func TestNewProviderError_FieldsSet(t *testing.T) {
	inner := errors.New("network error")
	pe := core.NewProviderError("anthropic", 500, inner)
	if pe.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", pe.Provider, "anthropic")
	}
	if pe.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", pe.StatusCode)
	}
	if pe.Err != inner {
		t.Errorf("Err = %v, want %v", pe.Err, inner)
	}
}

func TestNewProviderError_ZeroStatusCode_NoStatusInString(t *testing.T) {
	pe := core.NewProviderError("openai", 0, errors.New("oops"))
	got := pe.Error()
	if strings.Contains(got, "status 0") {
		t.Errorf("Error() should not contain 'status 0', got %q", got)
	}
}

func TestNewProviderError_Status429(t *testing.T) {
	pe := core.NewProviderError("openai", 429, errors.New("rate limited"))
	got := pe.Error()
	if !strings.Contains(got, "429") {
		t.Errorf("Error() = %q, want '429'", got)
	}
	if !strings.Contains(got, "openai") {
		t.Errorf("Error() = %q, want 'openai'", got)
	}
}
