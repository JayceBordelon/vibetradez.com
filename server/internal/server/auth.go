package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"time"

	"vibetradez.com/internal/store"
)

const (
	oauthStateCookie = "vt_oauth_state"
	oauthStateTTL    = 10 * time.Minute
)

type userCtxKey struct{}

func withUser(ctx context.Context, u *store.User) context.Context {
	return context.WithValue(ctx, userCtxKey{}, u)
}

func userFrom(ctx context.Context) *store.User {
	if u, ok := ctx.Value(userCtxKey{}).(*store.User); ok {
		return u
	}
	return nil
}

// attachUser reads the session cookie (if any), looks up the session, and
// attaches the user to the request context. Non-blocking: invalid or
// missing sessions proceed with no user attached.
func (s *Server) attachUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(s.sessionCookie)
		if err != nil || c.Value == "" {
			next(w, r)
			return
		}
		hash := sha256.Sum256([]byte(c.Value))
		sess, err := s.db.LookupSession(hash[:])
		if err != nil {
			log.Printf("attachUser: lookup error: %v", err)
			next(w, r)
			return
		}
		if sess == nil {
			next(w, r)
			return
		}
		if err := s.db.TouchSession(sess.ID); err != nil {
			log.Printf("attachUser: touch error: %v", err)
		}
		next(w, r.WithContext(withUser(r.Context(), &sess.User)))
	}
}

func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if s.google == nil {
		http.Error(w, "Google auth not configured", http.StatusServiceUnavailable)
		return
	}

	returnTo := r.URL.Query().Get("return_to")
	if !isSafeReturnTo(returnTo) {
		returnTo = "/"
	}

	state, err := randomToken(32)
	if err != nil {
		log.Printf("handleGoogleLogin: random: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := s.db.CreateOAuthState(state, returnTo, oauthStateTTL); err != nil {
		log.Printf("handleGoogleLogin: create state: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   int(oauthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, s.google.AuthURL(state), http.StatusFound)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if s.google == nil {
		http.Error(w, "Google auth not configured", http.StatusServiceUnavailable)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || cookie.Value == "" || cookie.Value != state {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	returnTo, err := s.db.ConsumeOAuthState(state)
	if err != nil {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}
	if !isSafeReturnTo(returnTo) {
		returnTo = "/"
	}

	clearOAuthStateCookie(w)

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	info, err := s.google.Exchange(ctx, code)
	if err != nil {
		log.Printf("handleGoogleCallback: exchange: %v", err)
		http.Error(w, "OAuth exchange failed", http.StatusInternalServerError)
		return
	}

	if !info.EmailVerified {
		http.Redirect(w, r, "/?auth_error=email_unverified", http.StatusFound)
		return
	}

	email := strings.ToLower(strings.TrimSpace(info.Email))
	if email == "" {
		http.Error(w, "Google returned no email", http.StatusBadRequest)
		return
	}

	userID, err := s.db.UpsertUser(info.Sub, email, info.EmailVerified, info.Name, info.Picture)
	if err != nil {
		log.Printf("handleGoogleCallback: upsert user: %v", err)
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	if err := s.db.AddSubscriber(email, info.Name); err != nil {
		log.Printf("handleGoogleCallback: add subscriber: %v", err)
	}
	if err := s.db.LinkSubscriber(userID, email); err != nil {
		log.Printf("handleGoogleCallback: link subscriber: %v", err)
	}

	token, err := randomToken(32)
	if err != nil {
		log.Printf("handleGoogleCallback: random token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	hash := sha256.Sum256([]byte(token))
	if err := s.db.CreateSession(userID, hash[:], r.UserAgent(), clientIP(r), s.sessionTTL); err != nil {
		log.Printf("handleGoogleCallback: create session: %v", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(s.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	log.Printf("Google sign-in: user_id=%d email=%s", userID, email)
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if c, err := r.Cookie(s.sessionCookie); err == nil && c.Value != "" {
		hash := sha256.Sum256([]byte(c.Value))
		if sess, err := s.db.LookupSession(hash[:]); err == nil && sess != nil {
			if err := s.db.RevokeSession(sess.ID); err != nil {
				log.Printf("handleLogout: revoke: %v", err)
			}
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

type meUser struct {
	ID         int64  `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	PictureURL string `json:"picture_url"`
}

type meResponse struct {
	User *meUser `json:"user"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeJSON(w, http.StatusOK, meResponse{User: nil})
		return
	}
	writeJSON(w, http.StatusOK, meResponse{User: &meUser{
		ID:         u.ID,
		Email:      u.Email,
		Name:       u.Name,
		PictureURL: u.PictureURL,
	}})
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// isSafeReturnTo ensures we only redirect to same-origin paths so the OAuth
// flow can't be used as an open redirector.
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

func clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// clientIP reads X-Forwarded-For when present (Traefik sets this in prod),
// falling back to r.RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}
