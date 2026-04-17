package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"vibetradez.com/internal/authclient"
)

const ssoStateCookie = "vt_sso_state"

type userCtxKey struct{}

func withUser(ctx context.Context, u *authclient.User) context.Context {
	return context.WithValue(ctx, userCtxKey{}, u)
}

func userFrom(ctx context.Context) *authclient.User {
	if u, ok := ctx.Value(userCtxKey{}).(*authclient.User); ok {
		return u
	}
	return nil
}

// attachUser reads the local vt_session cookie (holds an opaque access
// token issued by auth.jaycebordelon.com), verifies it via the auth
// service's /oauth/verify endpoint (cached 60s), and attaches the user
// to the request context. Non-blocking: invalid or missing tokens just
// proceed with no user.
func (s *Server) attachUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(s.sessionCookie)
		if err != nil || c.Value == "" {
			next(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		u, err := s.auth.Verify(ctx, c.Value)
		if err != nil {
			log.Printf("attachUser: verify: %v", err)
			next(w, r)
			return
		}
		if u == nil {
			next(w, r)
			return
		}
		next(w, r.WithContext(withUser(r.Context(), u)))
	}
}

// handleSSOStart kicks off the authorization code flow to
// auth.jaycebordelon.com. Generates a CSRF state, stores it in an
// httpOnly cookie (double-submit) and redirects to the auth service's
// /oauth/authorize with the consumer client id + registered redirect
// URI. return_to is echoed back through the auth service so the
// callback can bounce the user to the originating page.
func (s *Server) handleSSOStart(w http.ResponseWriter, r *http.Request) {
	returnTo := r.URL.Query().Get("return_to")
	if !isSafeReturnTo(returnTo) {
		returnTo = "/"
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "sso start failed", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(b)

	http.SetCookie(w, &http.Cookie{
		Name:     ssoStateCookie,
		Value:    state,
		Path:     "/auth/sso",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	q := url.Values{}
	q.Set("client_id", s.ssoClientID)
	q.Set("redirect_uri", s.ssoRedirectURI)
	q.Set("state", state)
	q.Set("return_to", returnTo)
	http.Redirect(w, r, s.ssoPublicURL+"/oauth/authorize?"+q.Encode(), http.StatusFound)
}

// handleSSOCallback completes the auth.jaycebordelon.com authorization
// code flow: exchanges the one-shot code for an access token, then sets
// the access token as the vt_session cookie on vibetradez.com.
func (s *Server) handleSSOCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	returnTo := r.URL.Query().Get("return_to")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	c, err := r.Cookie(ssoStateCookie)
	if err != nil || c.Value == "" || c.Value != state {
		http.Error(w, "invalid sso state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     ssoStateCookie,
		Value:    "",
		Path:     "/auth/sso",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tok, err := s.auth.Exchange(ctx, code)
	if err != nil {
		log.Printf("handleSSOCallback: exchange: %v", err)
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	email := strings.ToLower(strings.TrimSpace(tok.User.Email))
	if email != "" {
		if err := s.db.AddSubscriber(email, tok.User.Name); err != nil {
			log.Printf("handleSSOCallback: add subscriber: %v", err)
		}
		if err := s.db.LinkSubscriberAuthUser(tok.User.ID, email); err != nil {
			log.Printf("handleSSOCallback: link subscriber: %v", err)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.sessionCookie,
		Value:    tok.AccessToken,
		Path:     "/",
		MaxAge:   int(s.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	if !isSafeReturnTo(returnTo) {
		returnTo = "/"
	}
	log.Printf("SSO sign-in: auth_user_id=%d email=%s", tok.User.ID, email)
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if c, err := r.Cookie(s.sessionCookie); err == nil && c.Value != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := s.auth.Revoke(ctx, c.Value); err != nil {
			log.Printf("handleLogout: revoke: %v", err)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Message: "signed out"})
}

type meResponse struct {
	User *authclient.User `json:"user"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, meResponse{User: userFrom(r.Context())})
}

// isSafeReturnTo ensures we only redirect to same-origin paths so the
// callback can't be used as an open redirector.
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
