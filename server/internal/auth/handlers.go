package auth

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	stateCookieName = "vt_oauth_state"
	// stateCookieMaxAge gives a user 10 minutes to come back from
	// Google. The DB-side state row has the same TTL so the cookie and
	// the row expire in lockstep.
	stateCookieMaxAge = 600
)

type userCtxKey struct{}

// WithUser stashes a resolved user on the request context. Handlers
// downstream of attachUser read it via UserFrom.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userCtxKey{}, u)
}

// UserFrom returns the user attached by attachUser, or nil for an
// anonymous request.
func UserFrom(ctx context.Context) *User {
	if u, ok := ctx.Value(userCtxKey{}).(*User); ok {
		return u
	}
	return nil
}

/*
AttachUser is the middleware that pulls the vt_session cookie off
each request, looks it up against the sessions table, and stashes
the resolved user on the context. Non-blocking: invalid / missing
tokens just proceed anonymously, so public surfaces still render.
*/
func (s *Service) AttachUser(sessionCookieName string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookieName)
		if err != nil || c.Value == "" {
			next(w, r)
			return
		}
		u, err := s.ResolveSession(c.Value)
		if err != nil {
			log.Printf("auth attachUser: resolve: %v", err)
			next(w, r)
			return
		}
		if u == nil {
			next(w, r)
			return
		}
		next(w, r.WithContext(WithUser(r.Context(), u)))
	}
}

/*
HandleStart begins the Google OAuth flow. Generates a CSRF state,
stashes return_to behind it in the oauth_states table, sets a
double-submit state cookie, and redirects to Google's consent screen.
*/
func (s *Service) HandleStart(w http.ResponseWriter, r *http.Request) {
	returnTo := r.URL.Query().Get("return_to")
	if !isSafeReturnTo(returnTo) {
		returnTo = "/dashboard"
	}

	state, redirectURL, err := s.StartLogin(returnTo)
	if err != nil {
		log.Printf("auth start: %v", err)
		http.Error(w, "sign-in start failed", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/auth/google",
		MaxAge:   stateCookieMaxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

/*
HandleCallback completes the Google flow. Validates the state
parameter against the cookie (CSRF) and against the oauth_states row
(one-shot, TTL-bound), exchanges the Google code, upserts the user,
mints a session token, sets the vt_session cookie, and redirects to
return_to.

sessionCookieName is passed in so callers can use a per-deployment
cookie name (default vt_session) without this package holding it.
*/
func (s *Service) HandleCallback(sessionCookieName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}

		c, err := r.Cookie(stateCookieName)
		if err != nil || c.Value == "" || state == "" ||
			subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
			http.Error(w, "invalid oauth state", http.StatusBadRequest)
			return
		}
		// Clear the state cookie now that we've verified it.
		http.SetCookie(w, &http.Cookie{
			Name:     stateCookieName,
			Value:    "",
			Path:     "/auth/google",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		userAgent := r.Header.Get("User-Agent")
		ipAddr := clientIP(r)
		out, err := s.CompleteLogin(ctx, code, state, userAgent, ipAddr)
		if err != nil {
			log.Printf("auth callback: %v", err)
			http.Error(w, "sign-in failed", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    out.Token,
			Path:     "/",
			MaxAge:   int(s.sessionTTL.Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		log.Printf("auth sign-in: user_id=%d email=%s", out.User.ID, out.User.Email)
		http.Redirect(w, r, out.ReturnTo, http.StatusFound)
	}
}

/*
HandleLogout revokes the current session (if any) and clears the
session cookie. POST-only to defuse trivial CSRF.
*/
func (s *Service) HandleLogout(sessionCookieName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
			if err := s.Logout(c.Value); err != nil {
				log.Printf("auth logout: %v", err)
			}
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"message":"signed out"}`))
	}
}

/*
isSafeReturnTo refuses anything that would let the callback act as
an open redirector: must be a same-origin absolute path, not a
scheme-relative URL, not a Windows path.
*/
func isSafeReturnTo(p string) bool {
	if p == "" {
		return false
	}
	if !strings.HasPrefix(p, "/") {
		return false
	}
	if strings.HasPrefix(p, "//") {
		return false
	}
	if strings.Contains(p, "\\") {
		return false
	}
	return true
}

/*
clientIP returns the rightmost X-Forwarded-For hop (Traefik adds the
real-client hop at the right when forwarding), falling back to
r.RemoteAddr when the header is missing. The rate limiter elsewhere
on trading-server uses the same parsing.
*/
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// rightmost is the closest hop to us
		if i := strings.LastIndex(xff, ","); i >= 0 {
			return strings.TrimSpace(xff[i+1:])
		}
		return strings.TrimSpace(xff)
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}
