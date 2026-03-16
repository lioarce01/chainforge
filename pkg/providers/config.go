package providers

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds provider configuration loaded from YAML or environment variables.
type Config struct {
	Provider string `yaml:"provider"` // "anthropic", "openai", "ollama"
	APIKey   string `yaml:"api_key"`  // falls back to env var
	BaseURL  string `yaml:"base_url"` // for openai-compatible providers
	Model    string `yaml:"model"`
}

// LoadConfig loads provider configuration from a YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("providers: read config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("providers: parse config: %w", err)
	}
	return &cfg, nil
}

// applyEnvFallbacks fills empty fields from environment variables.
func (c *Config) applyEnvFallbacks() {
	if c.APIKey == "" {
		switch c.Provider {
		case "anthropic":
			c.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		case "openai":
			c.APIKey = os.Getenv("OPENAI_API_KEY")
		}
	}
	if c.BaseURL == "" && c.Provider == "ollama" {
		c.BaseURL = os.Getenv("OLLAMA_BASE_URL")
	}
}
