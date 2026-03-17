package chainforge

import (
	"fmt"

	"github.com/lioarce01/chainforge/pkg/providers"
)

// FromConfigFile loads provider configuration from a YAML file and returns a ready Agent.
// The YAML file must contain at minimum "provider" and "model" fields.
//
// Example config.yaml:
//
//	provider: anthropic
//	api_key:  sk-ant-...
//	model:    claude-sonnet-4-6
//
// extraOpts are applied after the provider and model are set, so they can override
// anything loaded from the file (e.g. WithSystemPrompt, WithTools, WithLogging).
func FromConfigFile(path string, extraOpts ...AgentOption) (*Agent, error) {
	cfg, err := providers.LoadConfig(path)
	if err != nil {
		return nil, fmt.Errorf("chainforge: FromConfigFile: %w", err)
	}
	p, err := providers.NewFromConfig(*cfg)
	if err != nil {
		return nil, fmt.Errorf("chainforge: FromConfigFile: %w", err)
	}
	opts := make([]AgentOption, 0, 2+len(extraOpts))
	opts = append(opts, WithProvider(p), WithModel(cfg.Model))
	opts = append(opts, extraOpts...)
	agent, err := NewAgent(opts...)
	if err != nil {
		return nil, fmt.Errorf("chainforge: FromConfigFile: %w", err)
	}
	return agent, nil
}
