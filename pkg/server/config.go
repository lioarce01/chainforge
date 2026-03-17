// Package server provides the HTTP server for chainforge.
package server

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the full server configuration. API keys are env-only (yaml:"-")
// so they can never appear in a committed config file.
type Config struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`

	Provider ProviderConfig `yaml:"provider"`
	Model    string         `yaml:"model"`

	// Logging
	LogLevel  string `yaml:"log_level"`  // "debug", "info", "warn", "error"
	LogFormat string `yaml:"log_format"` // "json", "text"

	// OTel
	OTelEnabled  bool   `yaml:"otel_enabled"`
	OTelEndpoint string `yaml:"otel_endpoint"` // e.g. "localhost:4317"

	// Server tuning
	MaxRequestBodyBytes int64 `yaml:"max_request_body_bytes"` // default 1 MiB
}

// ProviderConfig holds provider identity. API keys come from env.
type ProviderConfig struct {
	Name    string `yaml:"name"`     // "anthropic" | "openai" | "ollama"
	BaseURL string `yaml:"base_url"` // for OpenAI-compatible providers

	// API keys are deliberately env-only — never read from YAML.
	AnthropicAPIKey string `yaml:"-"`
	OpenAIAPIKey    string `yaml:"-"`
}

// Addr returns "host:port".
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// defaults fills in zero values with sensible defaults.
func (c *Config) defaults() {
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}
	if c.Port == 0 {
		c.Port = 8080
	}
	if c.Provider.Name == "" {
		c.Provider.Name = "anthropic"
	}
	if c.Model == "" {
		c.Model = "claude-sonnet-4-6"
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.LogFormat == "" {
		c.LogFormat = "json"
	}
	if c.MaxRequestBodyBytes == 0 {
		c.MaxRequestBodyBytes = 1 << 20 // 1 MiB
	}
	if c.OTelEndpoint == "" {
		c.OTelEndpoint = "localhost:4317"
	}
}

// applyEnvOverrides reads API keys from environment variables.
// These keys must not appear in YAML files.
func (c *Config) applyEnvOverrides() {
	c.Provider.AnthropicAPIKey = os.Getenv("ANTHROPIC_API_KEY")
	c.Provider.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")

	// Allow env overrides for non-secret fields too.
	if v := os.Getenv("CHAINFORGE_HOST"); v != "" {
		c.Host = v
	}
	if v := os.Getenv("CHAINFORGE_PORT"); v != "" {
		var port int
		if _, err := fmt.Sscanf(v, "%d", &port); err == nil && port > 0 {
			c.Port = port
		}
	}
	if v := os.Getenv("CHAINFORGE_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); v != "" {
		c.OTelEndpoint = v
	}
}

// Load reads YAML from path, applies defaults, then overlays environment variables.
// If path is empty, only defaults and env vars are used.
func Load(path string) (*Config, error) {
	var cfg Config
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("server config: read %q: %w", path, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("server config: parse %q: %w", path, err)
		}
	}
	cfg.defaults()
	cfg.applyEnvOverrides()
	return &cfg, nil
}
