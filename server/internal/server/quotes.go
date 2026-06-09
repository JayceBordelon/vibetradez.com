package server

import (
	"fmt"
	"net/http"
	"time"

	"vibetradez.com/internal/quotes"
)

/*
handleQuotesStream is the SSE fan-out of the Schwab streaming quotes: one
`data:` frame per tick, {type, symbol, mark, bid, ask}, for the held book
plus SPY. The browser re-prices position marks on every frame, so the
dashboard moves in real time instead of on the 60-second poll. Heartbeat
comments keep intermediaries from idling the connection out.

503 when no hub is wired (trading disabled, or no Schwab client): the UI
treats that as "polling only" and degrades quietly.
*/
func (s *Server) handleQuotesStream(w http.ResponseWriter, r *http.Request) {
	if s.quotesHub == nil {
		http.Error(w, "quote stream unavailable", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Tells nginx-style proxies not to buffer the stream.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, detach := s.quotesHub.Subscribe(r.Context())
	defer detach()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case payload, open := <-ch:
			if !open {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": hb\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// SetQuotesHub wires the streaming-quotes hub. nil (the default) leaves
// the SSE endpoint answering 503 and the UI on polling alone.
func (s *Server) SetQuotesHub(h *quotes.Hub) { s.quotesHub = h }
