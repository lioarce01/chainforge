package tests

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestFromConfigFile_Anthropic(t *testing.T) {
	path := writeTempConfig(t, `
provider: anthropic
api_key: sk-ant-fake
model: claude-sonnet-4-6
`)
	a, err := chainforge.FromConfigFile(path)
	if err != nil {
		t.Fatalf("FromConfigFile: %v", err)
	}
	if a == nil {
		t.Error("expected non-nil agent")
	}
}

func TestFromConfigFile_OpenAI(t *testing.T) {
	path := writeTempConfig(t, `
provider: openai
api_key: sk-fake
model: gpt-4o
`)
	a, err := chainforge.FromConfigFile(path)
	if err != nil {
		t.Fatalf("FromConfigFile: %v", err)
	}
	if a == nil {
		t.Error("expected non-nil agent")
	}
}

func TestFromConfigFile_Ollama(t *testing.T) {
	path := writeTempConfig(t, `
provider: ollama
model: llama3
`)
	a, err := chainforge.FromConfigFile(path)
	if err != nil {
		t.Fatalf("FromConfigFile: %v", err)
	}
	if a == nil {
		t.Error("expected non-nil agent")
	}
}

func TestFromConfigFile_ExtraOptsApplied(t *testing.T) {
	path := writeTempConfig(t, `
provider: ollama
model: llama3
`)
	a, err := chainforge.FromConfigFile(path,
		chainforge.WithSystemPrompt("You are helpful."),
		chainforge.WithMaxIterations(5),
	)
	if err != nil {
		t.Fatalf("FromConfigFile: %v", err)
	}
	if a == nil {
		t.Error("expected non-nil agent")
	}
}

func TestFromConfigFile_FileNotFound(t *testing.T) {
	_, err := chainforge.FromConfigFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "FromConfigFile") {
		t.Errorf("error = %q, want 'FromConfigFile' in message", err.Error())
	}
}

func TestFromConfigFile_InvalidYAML(t *testing.T) {
	path := writeTempConfig(t, `{not valid yaml: [`)
	_, err := chainforge.FromConfigFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "FromConfigFile") {
		t.Errorf("error = %q, want 'FromConfigFile' in message", err.Error())
	}
}

func TestFromConfigFile_UnknownProvider(t *testing.T) {
	path := writeTempConfig(t, `
provider: unknown_llm
api_key: key
model: some-model
`)
	_, err := chainforge.FromConfigFile(path)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "FromConfigFile") {
		t.Errorf("error = %q, want 'FromConfigFile' in message", err.Error())
	}
}

func TestFromConfigFile_MissingModel_ErrNoModel(t *testing.T) {
	path := writeTempConfig(t, `
provider: ollama
`)
	_, err := chainforge.FromConfigFile(path)
	if err == nil {
		t.Fatal("expected error for missing model")
	}
	if !errors.Is(err, core.ErrNoModel) {
		t.Errorf("error = %v, want ErrNoModel", err)
	}
}
