package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	chainforge "github.com/lioarce01/chainforge"
)

const version = "0.3.0"

// Server wraps the HTTP server and its dependencies.
type Server struct {
	cfg    *Config
	agent  *chainforge.Agent
	logger *slog.Logger
	http   *http.Server
}

// New creates a Server from config. The agent must be provided externally
// so callers can compose providers, memory, and middleware as needed.
func New(cfg *Config, agent *chainforge.Agent, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	h := &handlers{agent: agent, cfg: cfg, version: version}
	router := newRouterWithServer(h, cfg)

	// Wrap router with request logger and panic recovery.
	var handler http.Handler = router
	handler = Recovery(logger)(handler)
	handler = RequestLogger(logger)(handler)

	httpSrv := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // generous for SSE streams
		IdleTimeout:  120 * time.Second,
	}

	return &Server{
		cfg:    cfg,
		agent:  agent,
		logger: logger,
		http:   httpSrv,
	}
}

// Start begins listening. It blocks until the server is stopped.
// Call Shutdown() to stop it gracefully.
func (s *Server) Start() error {
	s.logger.Info("chainforge server starting",
		slog.String("addr", s.cfg.Addr()),
		slog.String("provider", s.cfg.Provider.Name),
		slog.String("model", s.cfg.Model),
		slog.String("version", version),
	)
	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: listen: %w", err)
	}
	return nil
}

// Shutdown drains in-flight requests within the timeout, then closes the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("chainforge server shutting down")
	if err := s.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("server: shutdown: %w", err)
	}
	if s.agent != nil {
		if err := s.agent.Close(); err != nil {
			s.logger.Warn("agent close error", slog.String("error", err.Error()))
		}
	}
	return nil
}

// Addr returns the address the server is configured to listen on.
func (s *Server) Addr() string { return s.cfg.Addr() }
