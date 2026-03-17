package server

import (
	"encoding/json"
	"net/http"
	"strings"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
)

// ChatRequest is the JSON body for POST /v1/chat and POST /v1/chat/stream.
type ChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// ChatResponse is the JSON body returned by POST /v1/chat.
type ChatResponse struct {
	Message core.Message `json:"message"`
	Usage   *core.Usage  `json:"usage,omitempty"`
}

// InfoResponse is returned by GET /v1/info.
type InfoResponse struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Version  string `json:"version"`
}

// handlers bundles the agent and config needed by all HTTP handlers.
type handlers struct {
	agent   *chainforge.Agent
	cfg     *Config
	version string
}

// handleHealthz is the liveness probe — always returns 200.
func (h *handlers) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// handleReadyz is the readiness probe — returns 200 if the agent is configured.
func (h *handlers) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent not ready")
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

// handleInfo returns provider/model metadata.
func (h *handlers) handleInfo(w http.ResponseWriter, r *http.Request) {
	resp := InfoResponse{
		Provider: h.cfg.Provider.Name,
		Model:    h.cfg.Model,
		Version:  h.version,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleChat handles POST /v1/chat (synchronous).
func (h *handlers) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	content, err := h.agent.Run(r.Context(), req.SessionID, req.Message)
	if err != nil {
		writeError(w, statusFor(err), err.Error())
		return
	}

	resp := ChatResponse{
		Message: core.Message{
			Role:    core.RoleAssistant,
			Content: content,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleChatStream handles POST /v1/chat/stream (SSE streaming).
func (h *handlers) handleChatStream(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	ch := h.agent.RunStream(r.Context(), req.SessionID, req.Message)
	serveSSE(w, r, ch)
}
