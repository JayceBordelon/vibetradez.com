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
golang.org/x/time/rate. One bucket per IP, identified by the right-
most X-Forwarded-For hop (Traefik's value) or the request's
RemoteAddr if XFF is absent.

Trusted-proxy semantics: the trading server runs behind Traefik in
production. Traefik APPENDS to whatever X-Forwarded-For a client
sent, so the LEFTmost entry is attacker-controlled and trusting it
lets a client spoof their source IP and bypass rate limits by
rotating values. The right-most entry is the last hop Traefik
recorded — that's the actual remote peer Traefik saw.

Buckets are reaped lazily during `limiterFor` whenever the map
exceeds a soft cap; entries older than `bucketTTL` get dropped.
Without this, a stream of distinct IPs (or a single client rotating
RemoteAddr ports) inflates the map until process restart.
*/
const (
	bucketTTL       = 30 * time.Minute
	bucketSoftMax   = 2048
	bucketReapEvery = 10 * time.Minute
)

type ipLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*rate.Limiter
	lastSeen map[string]time.Time
	lastReap time.Time
	rate     rate.Limit
	burst    int
}

func newIPLimiter(perMinute float64, burst int) *ipLimiter {
	return &ipLimiter{
		buckets:  make(map[string]*rate.Limiter),
		lastSeen: make(map[string]time.Time),
		rate:     rate.Limit(perMinute / 60.0),
		burst:    burst,
		lastReap: time.Now(),
	}
}

func (l *ipLimiter) limiterFor(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.maybeReapLocked()
	lim, ok := l.buckets[ip]
	if !ok {
		lim = rate.NewLimiter(l.rate, l.burst)
		l.buckets[ip] = lim
	}
	l.lastSeen[ip] = time.Now()
	return lim
}

/*
maybeReapLocked sweeps stale entries when either bucketReapEvery has
elapsed since the last sweep OR the map has grown past bucketSoftMax.
The combined condition guards against pathological growth on a
high-traffic minute that wouldn't otherwise tick the time-based
sweeper. Caller must hold l.mu.
*/
func (l *ipLimiter) maybeReapLocked() {
	if time.Since(l.lastReap) < bucketReapEvery && len(l.buckets) < bucketSoftMax {
		return
	}
	cutoff := time.Now().Add(-bucketTTL)
	for ip, seen := range l.lastSeen {
		if seen.Before(cutoff) {
			delete(l.buckets, ip)
			delete(l.lastSeen, ip)
		}
	}
	l.lastReap = time.Now()
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
clientIP extracts the originating client IP from an http.Request.
Uses the RIGHTmost X-Forwarded-For entry (the last hop our trusted
proxy recorded) and falls back to RemoteAddr. The leftmost entry is
attacker-controlled — Traefik appends to whatever the client sent
— so trusting it lets a client rotate XFF values to bypass per-IP
rate limits.
*/
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		// Walk right-to-left for the last non-empty entry.
		for i := len(parts) - 1; i >= 0; i-- {
			if v := strings.TrimSpace(parts[i]); v != "" {
				return v
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
