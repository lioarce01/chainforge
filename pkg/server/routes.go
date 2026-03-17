package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// newRouter wires all routes and middleware into a chi.Router.
func newRouter(h *handlers, cfg *Config, logger interface {
	// We accept slog.Logger via the Server; pass it through closure.
}) http.Handler {
	return newRouterWithServer(h, cfg)
}

// newRouterWithServer builds the chi router. Exposed separately for testing.
func newRouterWithServer(h *handlers, cfg *Config) http.Handler {
	r := chi.NewRouter()

	// Global middleware stack (innermost = last applied).
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.RequestID)
	r.Use(CORS())
	r.Use(LimitBody(cfg.MaxRequestBodyBytes))

	// Probes — no auth, no body limit override needed.
	r.Get("/healthz", h.handleHealthz)
	r.Get("/readyz", h.handleReadyz)

	// API routes.
	r.Route("/v1", func(r chi.Router) {
		r.Get("/info", h.handleInfo)
		r.Post("/chat", h.handleChat)
		r.Post("/chat/stream", h.handleChatStream)
	})

	return r
}
