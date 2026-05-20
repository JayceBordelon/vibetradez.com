package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"vibetradez.com/internal/calendar"
	"vibetradez.com/internal/quotes"
)

/*
handleMarketStatus returns the current market state so the dashboard
can decide between rendering the live UI (open) or the closed-page
(closed). Always 200; the body distinguishes states. No caching — the
status flips on the cron-tick boundary and the dashboard polls this
on tab focus.
*/
type marketStatusResponse struct {
	Open      bool   `json:"open"`
	Session   string `json:"session"` // premarket | open | afterhours | closed
	NextOpen  string `json:"next_open,omitempty"`
	NextClose string `json:"next_close,omitempty"`
}

func (s *Server) handleMarketStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	st := calendar.CurrentStatus(time.Now())
	resp := marketStatusResponse{
		Open:    st.Open,
		Session: st.Session,
	}
	if !st.NextOpen.IsZero() {
		resp.NextOpen = st.NextOpen.UTC().Format(time.RFC3339)
	}
	if !st.NextClose.IsZero() {
		resp.NextClose = st.NextClose.UTC().Format(time.RFC3339)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, resp)
}

/*
handleQuoteStream is the SSE endpoint. Returns 503 with Retry-After
when the quotes hub isn't running (off-hours, weekend, holiday). When
running, sends:

	event: snapshot   on connect — full current state (matches old
	                  /api/quotes/live REST shape so the UI lookup keys
	                  are unchanged).
	event: quote      on each tick — single symbol or option key.
	event: ping       every 15s keepalive (some proxies kill idle
	                  connections under 30s).

The handler holds the connection until the client disconnects or the
hub stops. http.ResponseController is used to flush each frame
immediately — without that, the Go default response buffering would
batch ticks and defeat the whole streaming UX.
*/
func (s *Server) handleQuoteStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.hub == nil || !s.hub.Running() {
		// Set Retry-After so well-behaved EventSource clients don't
		// hammer the endpoint while the market is closed. 60s gives the
		// client enough time to render the closed-page and check
		// /api/market/status on a slower cadence.
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "market closed; no quote stream"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	// Disable proxy buffering (Traefik, nginx) so each event flushes
	// straight through.
	w.Header().Set("X-Accel-Buffering", "no")

	rc := http.NewResponseController(w)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Initial snapshot.
	st := calendar.CurrentStatus(time.Now())
	snap := s.hub.SnapshotNow(st.Open)
	if err := writeSSE(w, flusher, rc, "snapshot", snap); err != nil {
		return
	}

	client := s.hub.Register()
	defer s.hub.Unregister(client)

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-client.Done():
			return
		case u := <-client.Updates():
			if err := writeSSE(w, flusher, rc, "quote", u); err != nil {
				return
			}
		case <-heartbeat.C:
			if err := writeSSE(w, flusher, rc, "ping", map[string]string{"t": time.Now().UTC().Format(time.RFC3339)}); err != nil {
				return
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, rc *http.ResponseController, event string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("sse: marshal %s: %v", event, err)
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body); err != nil {
		return err
	}
	flusher.Flush()
	// Bump the per-write deadline so a slow client doesn't stall the
	// process indefinitely. ResponseController's SetWriteDeadline is a
	// no-op on older mux implementations; treat the error as
	// informational.
	_ = rc.SetWriteDeadline(time.Now().Add(30 * time.Second))
	return nil
}

// Sanity assert at compile time that the hub type stays compatible.
var _ = (*quotes.Hub)(nil)
