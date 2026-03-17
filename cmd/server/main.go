// Command server starts the chainforge HTTP API server.
//
// Usage:
//
//	chainforge-server [--config path/to/config.yaml]
//
// Environment variables override YAML values. API keys must be set via env:
//
//	ANTHROPIC_API_KEY=sk-ant-... chainforge-server --config config.yaml
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/middleware/logging"
	"github.com/lioarce01/chainforge/pkg/providers"
	"github.com/lioarce01/chainforge/pkg/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "chainforge-server: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config.yaml (optional)")
	flag.Parse()

	cfg, err := server.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := buildLogger(cfg)

	// Build provider from server config.
	providerCfg := providers.Config{
		Provider: cfg.Provider.Name,
		APIKey:   apiKey(cfg),
		BaseURL:  cfg.Provider.BaseURL,
		Model:    cfg.Model,
	}
	rawProvider, err := providers.NewFromConfig(providerCfg)
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	// Wrap with logging middleware.
	loggedProvider := logging.NewLoggedProvider(rawProvider, logger)

	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(loggedProvider),
		chainforge.WithModel(cfg.Model),
	)
	if err != nil {
		return fmt.Errorf("build agent: %w", err)
	}

	srv := server.New(cfg, agent, logger)

	// Start server in background; wait for signal.
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		logger.Info("signal received, shutting down", slog.String("signal", sig.String()))
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

func buildLogger(cfg *server.Config) *slog.Logger {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

func apiKey(cfg *server.Config) string {
	switch cfg.Provider.Name {
	case "anthropic":
		return cfg.Provider.AnthropicAPIKey
	case "openai":
		return cfg.Provider.OpenAIAPIKey
	default:
		return ""
	}
}
