package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

/*
ipLimiter is a per-source-IP token-bucket rate limiter built on
golang.org/x/time/rate. One bucket per IP, identified by either the
X-Forwarded-For first hop (Traefik sets this) or the request's
RemoteAddr.

Trusted-proxy assumption: the trading server runs behind Traefik in
production, so X-Forwarded-For is set by infrastructure we control.
A direct external request that spoofs XFF would still be limited
because we use ONLY the leftmost token, which Traefik will overwrite
or append to.

Buckets are kept indefinitely in-memory. At single-server scale with
a small subscriber population this is fine; if the trading server
ever gets meaningful unauthenticated traffic the map needs a janitor.
*/
type ipLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
	lastSeen map[string]time.Time
}

func newIPLimiter(perMinute float64, burst int) *ipLimiter {
	return &ipLimiter{
		buckets:  make(map[string]*rate.Limiter),
		lastSeen: make(map[string]time.Time),
		rate:     rate.Limit(perMinute / 60.0),
		burst:    burst,
	}
}

func (l *ipLimiter) limiterFor(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.buckets[ip]
	if !ok {
		lim = rate.NewLimiter(l.rate, l.burst)
		l.buckets[ip] = lim
	}
	l.lastSeen[ip] = time.Now()
	return lim
}

/*
middleware wraps a handler with the rate limiter. On reject the
response is 429 Too Many Requests with a JSON body so the dashboard
client can surface a useful message. Doesn't set Retry-After because
golang.org/x/time/rate doesn't expose remaining tokens cheaply; the
client should back off by ~5s on 429.
*/
func (l *ipLimiter) middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !l.limiterFor(ip).Allow() {
			writeJSON(w, http.StatusTooManyRequests, apiResponse{OK: false, Message: "rate limit exceeded"})
			return
		}
		next(w, r)
	}
}

/*
clientIP extracts the originating client IP from an http.Request,
preferring the leftmost X-Forwarded-For hop (Traefik's value) and
falling back to RemoteAddr. Strips port from RemoteAddr because Go
formats it as "ip:port".
*/
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if first := strings.TrimSpace(parts[0]); first != "" {
			return first
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
