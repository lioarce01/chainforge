// Package chainforge is a provider-agnostic Go agent orchestration library.
// Developers import this package and call NewAgent() to build AI applications
// without touching any LLM SDK directly.
//
// Quick start:
//
//	agent := chainforge.NewAgent(
//	    chainforge.WithProvider(myProvider),
//	    chainforge.WithModel("claude-sonnet-4-6"),
//	    chainforge.WithSystemPrompt("You are a helpful assistant."),
//	)
//	result, err := agent.Run(ctx, "session-1", "Hello!")
package chainforge

import (
	"encoding/json"
	"fmt"
	"unicode"

	"github.com/lioarce01/chainforge/pkg/core"
)

// NewAgent constructs a new Agent with the provided options.
// Returns an error if required configuration (Provider, Model) is missing.
func NewAgent(opts ...AgentOption) (*Agent, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	for _, wrap := range cfg.providerWrappers {
		cfg.provider = wrap(cfg.provider)
	}
	return newAgent(cfg), nil
}

// MustNewAgent is like NewAgent but panics on configuration errors.
// Useful in main() or test setup where misconfiguration is a programmer error.
func MustNewAgent(opts ...AgentOption) *Agent {
	a, err := NewAgent(opts...)
	if err != nil {
		panic(fmt.Sprintf("chainforge: %v", err))
	}
	return a
}

func validateConfig(cfg agentConfig) error {
	// Deferred error from provider shorthand (e.g. WithGemini with bad key).
	if cfg.initErr != nil {
		return fmt.Errorf("chainforge: %w", cfg.initErr)
	}
	if cfg.provider == nil {
		return core.ErrNoProvider
	}
	if cfg.model == "" {
		return core.ErrNoModel
	}
	if cfg.maxIterations <= 0 {
		return fmt.Errorf("chainforge: maxIterations must be > 0")
	}
	if cfg.historySummarizer != nil && cfg.maxHistory <= 0 {
		return fmt.Errorf("chainforge: WithHistorySummarizer requires WithMaxHistory to be set to a positive value")
	}
	if cfg.retriever != nil && cfg.retrieverTopK <= 0 {
		cfg.retrieverTopK = 5 // apply default silently
	}
	// Tool validation: names must be non-empty, valid, and unique; schemas must be valid JSON.
	seen := make(map[string]bool, len(cfg.tools))
	for _, t := range cfg.tools {
		def := t.Definition()
		if def.Name == "" {
			return fmt.Errorf("chainforge: a registered tool has an empty name")
		}
		if !isValidToolName(def.Name) {
			return fmt.Errorf("chainforge: tool %q has an invalid name (use letters, digits, underscore, or hyphen only)", def.Name)
		}
		if seen[def.Name] {
			return fmt.Errorf("chainforge: duplicate tool name %q", def.Name)
		}
		seen[def.Name] = true
		if len(def.InputSchema) > 0 && !json.Valid(def.InputSchema) {
			return fmt.Errorf("chainforge: tool %q has an invalid JSON schema", def.Name)
		}
	}
	return nil
}

func isValidToolName(name string) bool {
	for _, c := range name {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' && c != '-' {
			return false
		}
	}
	return true
}
