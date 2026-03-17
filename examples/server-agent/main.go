// Package main demonstrates running chainforge as an HTTP server with
// LoggedProvider middleware. This is the simplest production-style setup.
//
// Run:
//
//	ANTHROPIC_API_KEY=sk-ant-... go run ./examples/server-agent/
//	curl localhost:8080/healthz
//	curl -X POST localhost:8080/v1/chat \
//	  -H 'Content-Type: application/json' \
//	  -d '{"session_id":"demo","message":"What is 2+2?"}'
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
	"github.com/lioarce01/chainforge/pkg/middleware/logging"
	"github.com/lioarce01/chainforge/pkg/providers"
	"github.com/lioarce01/chainforge/pkg/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// 1. Load config (defaults only — no config file for this example).
	cfg, err := server.Load("")
	if err != nil {
		return err
	}
	cfg.Provider.Name = "anthropic"
	cfg.Model = "claude-haiku-4-5-20251001" // cheap model for the demo

	// 2. Build provider + logging wrapper.
	rawProvider, err := providers.NewFromConfig(providers.Config{
		Provider: cfg.Provider.Name,
		APIKey:   os.Getenv("ANTHROPIC_API_KEY"),
		Model:    cfg.Model,
	})
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	loggedProvider := logging.NewLoggedProvider(rawProvider, logger)

	// 3. Build agent with in-memory history.
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(loggedProvider),
		chainforge.WithModel(cfg.Model),
		chainforge.WithMemory(inmemory.New()),
		chainforge.WithSystemPrompt("You are a helpful assistant."),
	)
	if err != nil {
		return fmt.Errorf("agent: %w", err)
	}

	// 4. Start server.
	srv := server.New(cfg, agent, logger)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	logger.Info("server ready", slog.String("addr", cfg.Addr()))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-quit:
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}
