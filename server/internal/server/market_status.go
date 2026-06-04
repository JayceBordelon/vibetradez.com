package server

import (
	"net/http"
	"time"

	"vibetradez.com/internal/calendar"
)

/*
handleMarketStatus returns the current market state so the dashboard can
decide its refresh cadence (poll while open, idle when closed). Always 200;
the body distinguishes states. No caching — the status flips on the
cron-tick boundary and the dashboard polls this on tab focus.
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
