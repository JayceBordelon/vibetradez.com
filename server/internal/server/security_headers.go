package server

import "net/http"

/*
securityHeaders wraps an http.Handler and emits a baseline set of
defense-in-depth response headers on every response. None of these are
load-bearing on their own — Traefik already terminates TLS, SameSite
cookies already block CSRF, the origin model already prevents most
cross-site mischief — but each one closes a small door that an
attacker would otherwise have ajar.

Headers emitted:

	X-Content-Type-Options: nosniff
	    Browsers won't second-guess the Content-Type on responses,
	    blocking content-type-confusion XSS via uploaded files etc.

	X-Frame-Options: DENY
	    Page can't be loaded in an iframe, blocking clickjacking on
	    /execute (the auto-trade confirmation page) and /dashboard.

	Referrer-Policy: strict-origin-when-cross-origin
	    Outbound clicks to schwab.com etc. don't leak the full URL
	    (which can include the signed token query param).

	Strict-Transport-Security: max-age=63072000; includeSubDomains
	    Two-year HSTS pin including subdomains. Safe to send because
	    the production deployment is HTTPS-only behind Traefik.

	Permissions-Policy: ...
	    Disables every browser API the trading UI doesn't use.
*/
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=(), payment=(), usb=()")
		next.ServeHTTP(w, r)
	})
}
