package providers

import (
	"fmt"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/providers/anthropic"
	"github.com/lioarce01/chainforge/pkg/providers/ollama"
	"github.com/lioarce01/chainforge/pkg/providers/openai"
)

// NewFromConfig creates a core.Provider from a Config struct.
// API keys fall back to environment variables if not set in config.
func NewFromConfig(cfg Config) (core.Provider, error) {
	cfg.applyEnvFallbacks()

	switch cfg.Provider {
	case "anthropic":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("providers: anthropic requires api_key or ANTHROPIC_API_KEY env var")
		}
		return anthropic.New(cfg.APIKey), nil

	case "openai":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("providers: openai requires api_key or OPENAI_API_KEY env var")
		}
		if cfg.BaseURL != "" {
			return openai.NewWithBaseURL(cfg.APIKey, cfg.BaseURL, "openai-compatible"), nil
		}
		return openai.New(cfg.APIKey), nil

	case "ollama":
		return ollama.New(cfg.BaseURL), nil

	default:
		return nil, fmt.Errorf("providers: unknown provider %q (supported: anthropic, openai, ollama)", cfg.Provider)
	}
}
