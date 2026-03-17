package qdrant

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	defaultPort           = 6334
	defaultCollectionName = "chainforge_memory"
	defaultTopK           = uint64(20)
)

// Config holds all settings for the Qdrant memory store.
type Config struct {
	Host           string
	Port           int
	APIKey         string
	CollectionName string
	UseTLS         bool
	TopK           uint64
	Embedder       Embedder
}

// Option mutates a Config.
type Option func(*Config)

func defaultConfig() Config {
	return Config{
		Host:           "localhost",
		Port:           defaultPort,
		CollectionName: defaultCollectionName,
		TopK:           defaultTopK,
	}
}

// WithURL parses rawURL and sets host, port, and TLS flag.
// Accepted forms: "localhost:6334", "https://xyz.cloud.qdrant.io:6334",
// "http://localhost:6334".
func WithURL(rawURL string) Option {
	return func(c *Config) {
		// If no scheme is provided, prepend one so url.Parse works correctly.
		if !strings.Contains(rawURL, "://") {
			rawURL = "grpc://" + rawURL
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			// Fall back to treating the whole string as a host.
			c.Host = rawURL
			return
		}
		c.Host = u.Hostname()
		if p := u.Port(); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				c.Port = n
			}
		}
		scheme := strings.ToLower(u.Scheme)
		c.UseTLS = scheme == "https" || scheme == "grpcs"
	}
}

// WithHost sets the Qdrant host and port directly.
func WithHost(host string, port int) Option {
	return func(c *Config) {
		c.Host = host
		c.Port = port
	}
}

// WithAPIKey sets the Qdrant API key (required for Qdrant Cloud).
func WithAPIKey(key string) Option {
	return func(c *Config) { c.APIKey = key }
}

// WithCollectionName overrides the default collection name.
func WithCollectionName(name string) Option {
	return func(c *Config) { c.CollectionName = name }
}

// WithTopK sets the maximum number of messages returned by Get.
func WithTopK(k uint64) Option {
	return func(c *Config) { c.TopK = k }
}

// WithEmbedder sets the embedding backend (required).
func WithEmbedder(e Embedder) Option {
	return func(c *Config) { c.Embedder = e }
}

// WithTLS explicitly enables or disables TLS.
func WithTLS(enabled bool) Option {
	return func(c *Config) { c.UseTLS = enabled }
}

func (c *Config) validate() error {
	if c.Embedder == nil {
		return ErrNoEmbedder
	}
	if c.Host == "" {
		return fmt.Errorf("qdrant: host is required")
	}
	return nil
}
