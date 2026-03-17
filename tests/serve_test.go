package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
)

// freeAddr finds a free TCP port on localhost.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freeAddr: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// startTestServer starts chainforge.ServeContext in a goroutine and waits
// until /healthz responds 200. Returns the base URL and a cancel func.
func startTestServer(t *testing.T, agent *chainforge.Agent) string {
	t.Helper()
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())

	go func() { _ = chainforge.ServeContext(ctx, addr, agent) }()

	baseURL := "http://" + addr
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Cleanup(cancel)
	return baseURL
}

func newTestAgent(t *testing.T, responses ...MockResponse) *chainforge.Agent {
	t.Helper()
	if len(responses) == 0 {
		responses = []MockResponse{EndTurnResponse("hello")}
	}
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(NewMockProvider(responses...)),
		chainforge.WithModel("mock"),
	)
	if err != nil {
		t.Fatalf("newTestAgent: %v", err)
	}
	return agent
}

func TestServeHealthz(t *testing.T) {
	baseURL := startTestServer(t, newTestAgent(t))

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("want status=ok, got %q", body["status"])
	}
}

func TestServeChat(t *testing.T) {
	baseURL := startTestServer(t, newTestAgent(t, EndTurnResponse("pong")))

	payload, _ := json.Marshal(map[string]string{"session_id": "s1", "message": "ping"})
	resp, err := http.Post(baseURL+"/v1/chat", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /v1/chat: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Message core.Message `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Message.Content != "pong" {
		t.Fatalf("want content=pong, got %q", result.Message.Content)
	}
}

func TestServeChatMissingFields(t *testing.T) {
	baseURL := startTestServer(t, newTestAgent(t))

	cases := []struct {
		name    string
		payload string
		wantMsg string
	}{
		{"empty message", `{"session_id":"s1","message":""}`, "message is required"},
		{"empty session", `{"session_id":"","message":"hi"}`, "session_id is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Post(baseURL+"/v1/chat", "application/json", strings.NewReader(tc.payload))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("want 400, got %d", resp.StatusCode)
			}
			var body map[string]string
			_ = json.NewDecoder(resp.Body).Decode(&body)
			if !strings.Contains(body["error"], tc.wantMsg) {
				t.Fatalf("want error %q, got %q", tc.wantMsg, body["error"])
			}
		})
	}
}

func TestServeChatStream(t *testing.T) {
	baseURL := startTestServer(t, newTestAgent(t, EndTurnResponse("streamed response")))

	payload, _ := json.Marshal(map[string]string{"session_id": "stream-1", "message": "hello"})
	resp, err := http.Post(baseURL+"/v1/chat/stream", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /v1/chat/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	raw := string(body)
	if !strings.Contains(raw, "event: done") {
		t.Fatalf("expected done event:\n%s", raw)
	}
	if !strings.Contains(raw, "event: text") {
		t.Fatalf("expected text event:\n%s", raw)
	}
}

func TestServeCORSHeaders(t *testing.T) {
	baseURL := startTestServer(t, newTestAgent(t))

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("want CORS *, got %q", got)
	}
}

func TestServeNilAgent(t *testing.T) {
	if err := chainforge.Serve(":0", nil); err == nil {
		t.Fatal("expected error for nil agent")
	}
}

func TestServeContextNilAgent(t *testing.T) {
	err := chainforge.ServeContext(context.Background(), ":0", nil)
	if err == nil {
		t.Fatal("expected error for nil agent")
	}
}

func TestServeContextGracefulShutdown(t *testing.T) {
	agent := newTestAgent(t)
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- chainforge.ServeContext(ctx, addr, agent) }()

	// Wait until ready.
	baseURL := "http://" + addr
	for i := 0; i < 50; i++ {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Cancel triggers shutdown.
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeContext returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ServeContext did not shut down within 5s")
	}
}
