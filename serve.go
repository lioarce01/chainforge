package chainforge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lioarce01/chainforge/pkg/core"
)

const (
	serveReadTimeout  = 30 * time.Second
	serveWriteTimeout = 5 * time.Minute // generous for SSE streams
	serveIdleTimeout  = 120 * time.Second
	serveMaxBodyBytes = int64(1 << 20) // 1 MiB
	serveShutdownWait = 30 * time.Second
)

// Serve starts an HTTP server at addr that exposes the agent over REST and SSE.
// It blocks until SIGINT or SIGTERM is received, then performs a 30-second
// graceful shutdown. For custom configuration (CORS origins, TLS, chi router)
// use pkg/server directly.
//
//	agent, _ := chainforge.NewAgent(...)
//	log.Fatal(chainforge.Serve(":8080", agent))
func Serve(addr string, agent *Agent) error {
	if agent == nil {
		return fmt.Errorf("chainforge: Serve: agent must not be nil")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return ServeContext(ctx, addr, agent)
}

// ServeContext is like Serve but driven by the caller's context instead of OS signals.
// Cancel ctx to trigger a graceful shutdown. Useful for embedding chainforge into a
// larger application that manages its own lifecycle.
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	go chainforge.ServeContext(ctx, ":8080", agent)
func ServeContext(ctx context.Context, addr string, agent *Agent) error {
	if agent == nil {
		return fmt.Errorf("chainforge: ServeContext: agent must not be nil")
	}
	return serveInternal(ctx, addr, agent, slog.Default())
}

func serveInternal(ctx context.Context, addr string, agent *Agent, logger *slog.Logger) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("POST /v1/chat", func(w http.ResponseWriter, r *http.Request) {
		serveChat(w, r, agent)
	})

	mux.HandleFunc("POST /v1/chat/stream", func(w http.ResponseWriter, r *http.Request) {
		serveChatStream(w, r, agent)
	})

	var handler http.Handler = mux
	handler = serveCORS(handler)
	handler = serveBodyLimiter(handler)
	handler = serveRecovery(handler, logger)

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  serveReadTimeout,
		WriteTimeout: serveWriteTimeout,
		IdleTimeout:  serveIdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("chainforge: server listening", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), serveShutdownWait)
		defer cancel()
		logger.Info("chainforge: server shutting down")
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("chainforge: Serve shutdown: %w", err)
		}
		_ = agent.Close()
		return nil
	}
}

// serveChatRequest is the JSON body for POST /v1/chat and POST /v1/chat/stream.
type serveChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// serveChatResponse is the JSON body returned by POST /v1/chat.
type serveChatResponse struct {
	Message core.Message `json:"message"`
}

func serveChat(w http.ResponseWriter, r *http.Request, agent *Agent) {
	var req serveChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		serveWriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		serveWriteError(w, http.StatusBadRequest, "message is required")
		return
	}
	if req.SessionID == "" {
		serveWriteError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	content, err := agent.Run(r.Context(), req.SessionID, req.Message)
	if err != nil {
		serveWriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := serveChatResponse{
		Message: core.Message{Role: core.RoleAssistant, Content: content},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func serveChatStream(w http.ResponseWriter, r *http.Request, agent *Agent) {
	var req serveChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		serveWriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		serveWriteError(w, http.StatusBadRequest, "message is required")
		return
	}
	if req.SessionID == "" {
		serveWriteError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	ch := agent.RunStream(r.Context(), req.SessionID, req.Message)
	serveSSEDrain(w, r, ch)
}

// serveSSEDrain adapts a <-chan core.StreamEvent to text/event-stream.
func serveSSEDrain(w http.ResponseWriter, r *http.Request, ch <-chan core.StreamEvent) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		serveWriteError(w, http.StatusInternalServerError, "streaming not supported")
		go func() {
			for range ch {
			}
		}()
		return
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			go func() {
				for range ch {
				}
			}()
			return
		case ev, ok := <-ch:
			if !ok {
				serveWriteSSE(w, flusher, "done", `{"stop_reason":"end_turn"}`)
				return
			}
			switch ev.Type {
			case core.StreamEventText:
				payload, _ := json.Marshal(map[string]string{"delta": ev.TextDelta})
				serveWriteSSE(w, flusher, "text", string(payload))
			case core.StreamEventToolCall:
				if ev.ToolCall != nil {
					payload, _ := json.Marshal(map[string]string{
						"id":    ev.ToolCall.ID,
						"name":  ev.ToolCall.Name,
						"input": ev.ToolCall.Input,
					})
					serveWriteSSE(w, flusher, "tool_call", string(payload))
				}
			case core.StreamEventDone:
				payload, _ := json.Marshal(map[string]string{"stop_reason": string(ev.StopReason)})
				serveWriteSSE(w, flusher, "done", string(payload))
				return
			case core.StreamEventError:
				msg := "unknown error"
				if ev.Error != nil {
					msg = ev.Error.Error()
				}
				payload, _ := json.Marshal(map[string]string{"error": msg})
				serveWriteSSE(w, flusher, "error", string(payload))
				return
			}
		}
	}
}

func serveWriteSSE(w http.ResponseWriter, f http.Flusher, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	f.Flush()
}

func serveWriteError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	payload, _ := json.Marshal(map[string]string{"error": msg})
	_, _ = w.Write(payload)
}

func serveCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func serveBodyLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, serveMaxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

func serveRecovery(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.ErrorContext(r.Context(), "chainforge: http panic", slog.Any("panic", rec))
				serveWriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
