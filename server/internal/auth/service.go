package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

/*
Service is the trading-server's in-process auth layer. Owns the
Google OAuth client, the auth-DB connection, and the session-token
lifecycle. The HTTP handlers (handlers.go) and the AttachUser
middleware are thin wrappers that call into Service methods.
*/
type Service struct {
	store      *Store
	google     *GoogleClient
	sessionTTL time.Duration
	stateTTL   time.Duration
}

type Options struct {
	SessionTTL time.Duration // how long a signed-in cookie lasts (default 30 days)
	StateTTL   time.Duration // how long an in-flight Google flow can dangle (default 10 minutes)
}

func New(store *Store, google *GoogleClient, opts Options) *Service {
	if opts.SessionTTL == 0 {
		opts.SessionTTL = 30 * 24 * time.Hour
	}
	if opts.StateTTL == 0 {
		opts.StateTTL = 10 * time.Minute
	}
	return &Service{
		store:      store,
		google:     google,
		sessionTTL: opts.SessionTTL,
		stateTTL:   opts.StateTTL,
	}
}

// SessionTTL exposes the configured cookie TTL so HTTP handlers can
// build the Set-Cookie header without re-reading config.
func (s *Service) SessionTTL() time.Duration { return s.sessionTTL }

/*
StartLogin mints a CSRF state, stashes return_to behind it, and
returns the Google consent URL the caller should redirect the browser
to. The state IS the random string returned — handlers should set it
as an httpOnly state cookie in parallel so the callback can
double-submit verify.
*/
func (s *Service) StartLogin(returnTo string) (state, redirectURL string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate state: %w", err)
	}
	state = base64.RawURLEncoding.EncodeToString(b)

	if err := s.store.CreateOAuthState(state, returnTo, s.stateTTL); err != nil {
		return "", "", err
	}
	return state, s.google.AuthURL(state), nil
}

/*
CompleteLogin is the callback-handler's worker: exchange Google's
one-shot code for userinfo, upsert the user, mint a fresh session
token, persist its hash, and return the raw token + the resolved
user + the original return_to. Caller sets the session cookie with
the returned raw token.
*/
type CompletedLogin struct {
	Token    string // raw session token, set as the vt_session cookie value
	User     User
	ReturnTo string
}

func (s *Service) CompleteLogin(ctx context.Context, code, state, userAgent, ipAddr string) (*CompletedLogin, error) {
	returnTo, err := s.store.ConsumeOAuthState(state)
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}

	info, err := s.google.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("google exchange: %w", err)
	}
	if info.Sub == "" || info.Email == "" {
		return nil, fmt.Errorf("google userinfo missing sub or email")
	}

	userID, err := s.store.UpsertUser(info.Sub, info.Email, info.EmailVerified, info.Name, info.Picture)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	token, err := mintToken()
	if err != nil {
		return nil, fmt.Errorf("mint session token: %w", err)
	}
	if err := s.store.CreateSession(userID, hashToken(token), userAgent, ipAddr, s.sessionTTL); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &CompletedLogin{
		Token: token,
		User: User{
			ID:            userID,
			GoogleSub:     info.Sub,
			Email:         info.Email,
			EmailVerified: info.EmailVerified,
			Name:          info.Name,
			PictureURL:    info.Picture,
		},
		ReturnTo: returnTo,
	}, nil
}

/*
ResolveSession reads a raw session token (e.g. from the vt_session
cookie) and returns the matching user, or nil if the token is
invalid / revoked / expired. Non-error nil is the "not signed in"
signal; callers should treat it as anonymous.

One indexed Postgres lookup per call — fast enough to skip the
in-memory cache layer middleware-style auth historically used.
*/
func (s *Service) ResolveSession(token string) (*User, error) {
	if token == "" {
		return nil, nil
	}
	sess, err := s.store.LookupSession(hashToken(token))
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, nil
	}
	// Best-effort last-used touch (skips when the row was already
	// touched in the last hour; avoids hot-write contention).
	if err := s.store.TouchSession(sess.ID); err != nil {
		// Touching is opportunistic; a transient failure shouldn't
		// take down the auth lookup. Log via the caller if needed.
		_ = err
	}
	u := sess.User
	return &u, nil
}

/*
Logout revokes the session backing the given raw token. No-op when
the token doesn't resolve (already signed out, expired, forged).
*/
func (s *Service) Logout(token string) error {
	if token == "" {
		return nil
	}
	sess, err := s.store.LookupSession(hashToken(token))
	if err != nil {
		return err
	}
	if sess == nil {
		return nil
	}
	return s.store.RevokeSession(sess.ID)
}

// mintToken generates a high-entropy session token. 32 bytes of CSPRNG
// base64url-encoded = ~43 chars; safe to embed in a cookie value.
func mintToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken returns the SHA-256 of the raw token. Only the hash is
// stored in Postgres so a DB compromise can't be replayed as a cookie.
func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
