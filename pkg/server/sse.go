package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lioarce01/chainforge/pkg/core"
)

// sseTextPayload is sent for StreamEventText.
type sseTextPayload struct {
	Delta string `json:"delta"`
}

// sseToolCallPayload is sent for StreamEventToolCall.
type sseToolCallPayload struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

// sseDonePayload is sent for StreamEventDone.
type sseDonePayload struct {
	StopReason string `json:"stop_reason"`
}

// sseErrorPayload is sent for StreamEventError.
type sseErrorPayload struct {
	Error string `json:"error"`
}

// writeSSEEvent writes a single SSE event frame and flushes.
// Format:  "event: <name>\ndata: <json>\n\n"
func writeSSEEvent(w http.ResponseWriter, f http.Flusher, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	f.Flush()
}

// serveSSE adapts a <-chan core.StreamEvent to text/event-stream.
// It sets the correct headers, then drains the channel until:
//   - a done or error event is received, or
//   - the client disconnects (r.Context().Done()).
//
// Goroutine safety: the channel is fully drained to avoid leaking the
// goroutine that writes to it.
func serveSSE(w http.ResponseWriter, r *http.Request, ch <-chan core.StreamEvent) {
	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Tell nginx not to buffer the response.
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		// drain so the producer goroutine can exit
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
			// Client disconnected — drain channel in background to unblock producer.
			go func() {
				for range ch {
				}
			}()
			return

		case ev, ok := <-ch:
			if !ok {
				// Channel closed without a done event — send done.
				payload, _ := json.Marshal(sseDonePayload{StopReason: "end_turn"})
				writeSSEEvent(w, flusher, "done", string(payload))
				return
			}

			switch ev.Type {
			case core.StreamEventText:
				payload, _ := json.Marshal(sseTextPayload{Delta: ev.TextDelta})
				writeSSEEvent(w, flusher, "text", string(payload))

			case core.StreamEventToolCall:
				if ev.ToolCall != nil {
					payload, _ := json.Marshal(sseToolCallPayload{
						ID:    ev.ToolCall.ID,
						Name:  ev.ToolCall.Name,
						Input: ev.ToolCall.Input,
					})
					writeSSEEvent(w, flusher, "tool_call", string(payload))
				}

			case core.StreamEventDone:
				payload, _ := json.Marshal(sseDonePayload{StopReason: string(ev.StopReason)})
				writeSSEEvent(w, flusher, "done", string(payload))
				return

			case core.StreamEventError:
				msg := "unknown error"
				if ev.Error != nil {
					msg = ev.Error.Error()
				}
				payload, _ := json.Marshal(sseErrorPayload{Error: msg})
				writeSSEEvent(w, flusher, "error", string(payload))
				return
			}
		}
	}
}
