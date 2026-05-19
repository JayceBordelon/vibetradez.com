package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

/*
TestClientIP_PrefersRightmostXFFHop guards the audit fix: the
leftmost X-Forwarded-For entry is attacker-controlled (Traefik
appends to whatever the client sent), so trusting it lets a
client rotate values to bypass per-IP rate limits.
*/
func TestClientIP_PrefersRightmostXFFHop(t *testing.T) {
	cases := []struct {
		name       string
		xff        string
		remoteAddr string
		want       string
	}{
		{"single hop", "203.0.113.5", "1.2.3.4:1234", "203.0.113.5"},
		{"two hops, trust right", "10.0.0.1, 203.0.113.5", "1.2.3.4:1234", "203.0.113.5"},
		{"attacker-spoofed left ignored", "evil-spoof, real-traefik", "1.2.3.4:1234", "real-traefik"},
		{"trailing comma tolerated", "203.0.113.5, ", "1.2.3.4:1234", "203.0.113.5"},
		{"no XFF, fall back to RemoteAddr without port", "", "1.2.3.4:1234", "1.2.3.4"},
		{"empty XFF, fall back to RemoteAddr", "  ", "5.6.7.8:9999", "5.6.7.8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/x", nil)
			r.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			got := clientIP(r)
			if got != tc.want {
				t.Errorf("clientIP(xff=%q, RemoteAddr=%q) = %q, want %q", tc.xff, tc.remoteAddr, got, tc.want)
			}
		})
	}
}

/*
TestIPLimiter_ReapsStaleBuckets guards the unbounded-growth bug: a
flood of distinct IPs would grow ipLimiter.buckets forever. The
reaper drops entries older than bucketTTL once a sweep threshold
is crossed.
*/
func TestIPLimiter_ReapsStaleBuckets(t *testing.T) {
	l := newIPLimiter(60, 5)
	// Force the soft-max threshold: register one more than the cap.
	for i := 0; i <= bucketSoftMax; i++ {
		l.limiterFor("ip-" + strconv.Itoa(i))
	}

	l.mu.Lock()
	if len(l.buckets) < bucketSoftMax {
		l.mu.Unlock()
		t.Fatalf("setup: expected at least %d buckets, got %d", bucketSoftMax, len(l.buckets))
	}
	// Force every existing entry to look ancient so the reaper drops them.
	ancient := time.Now().Add(-2 * bucketTTL)
	for ip := range l.lastSeen {
		l.lastSeen[ip] = ancient
	}
	l.lastReap = time.Now().Add(-2 * bucketReapEvery)
	l.mu.Unlock()

	// Next call triggers maybeReapLocked.
	l.limiterFor("trigger-reap")

	l.mu.Lock()
	defer l.mu.Unlock()
	// After reap, only the trigger entry should remain.
	if len(l.buckets) > 1 {
		t.Fatalf("expected reap to leave 1 bucket, got %d", len(l.buckets))
	}
	if _, ok := l.buckets["trigger-reap"]; !ok {
		t.Fatalf("trigger entry should survive the sweep")
	}
}
