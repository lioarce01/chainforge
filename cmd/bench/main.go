// Command bench runs end-to-end latency benchmarks against real providers.
//
// Usage:
//
//	chainforge-bench --config path/to/config.yaml \
//	  --concurrency 4 --requests 50 --warmup 5
//
// The benchmark measures agent.Run() latency end-to-end including LLM API
// calls, and reports p50/p95/p99 latencies and requests/sec.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/benchutil"
	"github.com/lioarce01/chainforge/pkg/memory/inmemory"
	"github.com/lioarce01/chainforge/pkg/middleware/logging"
	"github.com/lioarce01/chainforge/pkg/providers"
	"github.com/lioarce01/chainforge/pkg/server"
	"log/slog"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "chainforge-bench: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configPath  string
		concurrency int
		requests    int
		warmup      int
		message     string
		mock        bool
	)
	flag.StringVar(&configPath, "config", "", "path to config.yaml")
	flag.IntVar(&concurrency, "concurrency", 1, "number of concurrent workers")
	flag.IntVar(&requests, "requests", 20, "total requests to send")
	flag.IntVar(&warmup, "warmup", 3, "warmup requests (not included in results)")
	flag.StringVar(&message, "message", "Say hello in one sentence.", "message to send")
	flag.BoolVar(&mock, "mock", false, "use mock provider (no real API calls)")
	flag.Parse()

	cfg, err := server.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	var agent *chainforge.Agent

	if mock {
		fmt.Println("Using mock provider (zero network latency)")
		p := benchutil.NewMockProvider(benchutil.LargeResponseText(256))
		agent, err = chainforge.NewAgent(
			chainforge.WithProvider(p),
			chainforge.WithModel("mock"),
			chainforge.WithMemory(inmemory.New()),
		)
	} else {
		providerCfg := providers.Config{
			Provider: cfg.Provider.Name,
			APIKey:   apiKey(cfg),
			BaseURL:  cfg.Provider.BaseURL,
			Model:    cfg.Model,
		}
		rawProvider, perr := providers.NewFromConfig(providerCfg)
		if perr != nil {
			return fmt.Errorf("build provider: %w", perr)
		}
		loggedProvider := logging.NewLoggedProvider(rawProvider, logger)
		agent, err = chainforge.NewAgent(
			chainforge.WithProvider(loggedProvider),
			chainforge.WithModel(cfg.Model),
			chainforge.WithMemory(inmemory.New()),
		)
	}
	if err != nil {
		return fmt.Errorf("build agent: %w", err)
	}
	defer agent.Close()

	fmt.Printf("chainforge benchmark\n")
	fmt.Printf("  provider    : %s\n", cfg.Provider.Name)
	fmt.Printf("  model       : %s\n", cfg.Model)
	fmt.Printf("  concurrency : %d\n", concurrency)
	fmt.Printf("  requests    : %d (+ %d warmup)\n", requests, warmup)
	fmt.Println()

	// Warmup.
	if warmup > 0 {
		fmt.Printf("Warming up (%d requests)...\n", warmup)
		for i := 0; i < warmup; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			_, _ = agent.Run(ctx, fmt.Sprintf("warmup-%d", i), message)
			cancel()
		}
	}

	// Benchmark.
	fmt.Printf("Running benchmark (%d requests, concurrency %d)...\n", requests, concurrency)
	rec := &benchutil.LatencyRecorder{}
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		errCount int
	)

	sem := make(chan struct{}, concurrency)
	start := time.Now()

	for i := 0; i < requests; i++ {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			reqStart := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			_, err := agent.Run(ctx, fmt.Sprintf("bench-%d", idx), message)
			dur := time.Since(reqStart)

			mu.Lock()
			if err != nil {
				errCount++
			} else {
				rec.Record(dur)
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	total := time.Since(start)

	// Results.
	summary := rec.Summarize()
	rps := benchutil.ThroughputRPS(rec.Len(), total)

	fmt.Printf("\nResults:\n")
	summary.Print(os.Stdout)
	fmt.Printf("  errors   : %d\n", errCount)
	fmt.Printf("  total    : %s\n", total)
	fmt.Printf("  rps      : %.2f\n", rps)

	return nil
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
