package schwab

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	baseURL    = "https://api.schwabapi.com"
	authPath   = "/v1/oauth/authorize"
	tokenPath  = "/v1/oauth/token"
	tokenGrace = 2 * time.Minute // refresh 2 min before expiry
)

// TokenStore persists OAuth tokens across restarts.
type TokenStore interface {
	SaveOAuthToken(provider, accessToken, refreshToken string, expiresAt time.Time) error
	GetOAuthToken(provider string) (accessToken, refreshToken string, expiresAt time.Time, err error)
	/*
		MarkRefreshTokenIssued is called by ExchangeCode (a fresh OAuth
		consent) and never by the access-token refresh path. Schwab does
		not rotate refresh tokens on access-token refresh, so the
		original consent timestamp is what bounds the 7-day lifetime.
	*/
	MarkRefreshTokenIssued(provider string, issuedAt time.Time) error
	GetRefreshTokenIssuedAt(provider string) (issuedAt time.Time, lastNagSentOn time.Time, ok bool, err error)
	MarkReauthNagSent(provider string, sentOn time.Time) error
}

type Client struct {
	appKey      string
	secret      string
	callbackURL string
	tokenKey    []byte
	store       TokenStore
	httpClient  *http.Client

	mu          sync.RWMutex
	accessToken string
	refreshTok  string
	expiresAt   time.Time
}

/*
NewClient builds a Schwab client. tokenKey is the dedicated 32-byte
AES key for token-at-rest encryption (sourced from
SCHWAB_TOKEN_ENCRYPTION_KEY in config). All NEW persisted tokens go
through this key. Persisted tokens that pre-date the migration get
decrypted with the legacy sha256(SchwabSecret) key and immediately
re-persisted under the new key.
*/
func NewClient(appKey, secret, callbackURL string, tokenKey []byte, store TokenStore) *Client {
	c := &Client{
		appKey:      appKey,
		secret:      secret,
		callbackURL: callbackURL,
		tokenKey:    tokenKey,
		store:       store,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
	// Load persisted tokens on startup (stored encrypted). Try the new
	// dedicated key first; fall back to the legacy sha256(secret) key
	// so rows persisted before the migration still decrypt. After a
	// successful legacy decrypt, re-persist with the new key.
	if encAT, encRT, exp, err := store.GetOAuthToken("schwab"); err == nil && encRT != "" {
		at, atUsedLegacy, atErr := tryDecrypt(encAT, c.tokenKey, secret)
		rt, rtUsedLegacy, rtErr := tryDecrypt(encRT, c.tokenKey, secret)
		switch {
		case atErr == nil && rtErr == nil:
			c.accessToken = at
			c.refreshTok = rt
			c.expiresAt = exp
			log.Printf("Schwab: loaded persisted tokens (expires %s)", exp.Format(time.RFC3339))
			if atUsedLegacy || rtUsedLegacy {
				log.Println("Schwab: persisted tokens were encrypted with legacy key — re-encrypting with SCHWAB_TOKEN_ENCRYPTION_KEY")
				c.persistTokens(c.accessToken, c.refreshTok, c.expiresAt)
			}
		default:
			log.Printf("Schwab: failed to decrypt persisted tokens, re-authorization required (at_err=%v, rt_err=%v)", atErr, rtErr)
		}
	}
	return c
}

/*
tryDecrypt prefers the new dedicated key but falls back to the
legacy sha256(secret) key when the new key fails. Returns the
plaintext, whether the legacy path was used (so the caller can
trigger re-encryption), and the final error.
*/
func tryDecrypt(encoded string, newKey []byte, legacySecret string) (string, bool, error) {
	if len(newKey) == 32 {
		if pt, err := DecryptWithKey(encoded, newKey); err == nil {
			return pt, false, nil
		}
	}
	if legacySecret == "" {
		return "", false, fmt.Errorf("no decryption key available")
	}
	pt, err := Decrypt(encoded, legacySecret)
	if err != nil {
		return "", false, err
	}
	return pt, true, nil
}

// IsConnected returns true if we have a refresh token (may still need refreshing).
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.refreshTok != ""
}

/*
AuthorizationURL returns the URL to redirect the user to for OAuth
authorization. `state` is the CSRF token the caller minted and
double-submitted as a host-scoped cookie; Schwab will echo it back on
the callback. Without binding initiation to callback via state, a
crafted /auth/callback?code=... URL on this host can rebind the
production trader to an attacker's Schwab account once the operator
clicks it.
*/
func (c *Client) AuthorizationURL(state string) string {
	return fmt.Sprintf("%s%s?response_type=code&client_id=%s&redirect_uri=%s&state=%s",
		baseURL, authPath,
		url.QueryEscape(c.appKey),
		url.QueryEscape(c.callbackURL),
		url.QueryEscape(state),
	)
}

// ExchangeCode exchanges an authorization code for access + refresh tokens.
func (c *Client) ExchangeCode(code string) error {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {c.callbackURL},
	}

	tokens, err := c.tokenRequest(data)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	// Floor ExpiresIn at 60s. Schwab returning 0 (or a negative value
	// from a malformed response) would set expiresAt to now() and
	// trigger a refresh storm on every subsequent API call.
	expiresIn := time.Duration(tokens.ExpiresIn) * time.Second
	if expiresIn < 60*time.Second {
		expiresIn = 60 * time.Second
	}
	c.mu.Lock()
	c.accessToken = tokens.AccessToken
	c.refreshTok = tokens.RefreshToken
	c.expiresAt = time.Now().Add(expiresIn)
	c.mu.Unlock()

	c.persistTokens(tokens.AccessToken, tokens.RefreshToken, c.expiresAt)
	if err := c.store.MarkRefreshTokenIssued("schwab", time.Now()); err != nil {
		log.Printf("Schwab: warning: failed to mark refresh-token issuance: %v", err)
	}
	log.Println("Schwab: OAuth tokens obtained successfully")
	return nil
}

/*
RefreshTokenIssuedAt returns the timestamp the current refresh token
was minted (via the OAuth consent flow), and ok=false if it's unknown
(legacy row pre the issuance-tracking migration, or no token at all).
The daily expiry-warning cron uses this to decide when to nag.
*/
func (c *Client) RefreshTokenIssuedAt() (time.Time, bool) {
	issuedAt, _, ok, err := c.store.GetRefreshTokenIssuedAt("schwab")
	if err != nil {
		log.Printf("Schwab: warning: failed to read refresh-token issuance: %v", err)
		return time.Time{}, false
	}
	return issuedAt, ok
}

// ValidToken returns a valid access token, refreshing if necessary.
func (c *Client) ValidToken() (string, error) {
	if c == nil {
		return "", fmt.Errorf("schwab not configured: client is nil")
	}
	c.mu.RLock()
	tok := c.accessToken
	exp := c.expiresAt
	rt := c.refreshTok
	c.mu.RUnlock()

	if rt == "" {
		return "", fmt.Errorf("schwab not connected: no refresh token")
	}

	if tok != "" && time.Now().Before(exp.Add(-tokenGrace)) {
		return tok, nil
	}

	// Need to refresh.
	return c.doRefresh(rt)
}

/*
doRefresh performs the access-token refresh while keeping the write
lock held for the bare minimum of time. The previous shape held
c.mu.Lock() across the full tokenRequest network round-trip, which
serialized EVERY other Schwab call in the process behind a single
refresh — per-minute reconcile + live-quote polls all stalled on
the same mutex.

New shape:
 1. Take the lock briefly to double-check whether another goroutine
    already refreshed (releases lock immediately on hit).
 2. Capture the in-flight refresh token while locked, then release.
 3. Network round-trip happens unlocked.
 4. Re-take the lock to commit the new token state. Final double-
    check covers the case where a faster concurrent doRefresh
    committed a fresher token while we were waiting on Schwab —
    prefer the fresher one and discard our result.

Schwab's refresh endpoint is idempotent for the same input refresh
token (returns the same access token), so two concurrent inflight
refreshes against the same refresh value would return interchangeable
results; the loser's bytes are simply discarded.

ExpiresIn is also defended below: Schwab returning 0 (omitted /
malformed) would set expiresAt to now() and trigger an immediate
refresh storm on every subsequent call. Clamp to a 60s minimum so a
bad response self-throttles.
*/
func (c *Client) doRefresh(refreshToken string) (string, error) {
	c.mu.Lock()
	// Fast path: someone else already refreshed.
	if c.accessToken != "" && time.Now().Before(c.expiresAt.Add(-tokenGrace)) {
		tok := c.accessToken
		c.mu.Unlock()
		return tok, nil
	}
	c.mu.Unlock()

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	tokens, err := c.tokenRequest(data)
	if err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}

	expiresIn := time.Duration(tokens.ExpiresIn) * time.Second
	if expiresIn < 60*time.Second {
		expiresIn = 60 * time.Second
	}
	newExpiresAt := time.Now().Add(expiresIn)

	c.mu.Lock()
	defer c.mu.Unlock()
	// Final double-check: prefer a fresher token committed by a
	// concurrent doRefresh while we were waiting on Schwab.
	if c.accessToken != "" && c.expiresAt.After(newExpiresAt) {
		return c.accessToken, nil
	}

	c.accessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		c.refreshTok = tokens.RefreshToken
	}
	c.expiresAt = newExpiresAt

	c.persistTokens(c.accessToken, c.refreshTok, c.expiresAt)
	log.Println("Schwab: access token refreshed")
	return c.accessToken, nil
}

func (c *Client) persistTokens(accessToken, refreshToken string, expiresAt time.Time) {
	var (
		encAT, encRT string
		errA, errR   error
	)
	if len(c.tokenKey) == 32 {
		encAT, errA = EncryptWithKey(accessToken, c.tokenKey)
		encRT, errR = EncryptWithKey(refreshToken, c.tokenKey)
	} else {
		// Migration mode: SCHWAB_TOKEN_ENCRYPTION_KEY not set yet.
		// Fall back to the legacy sha256(secret) key so refreshes still
		// persist. Once the operator sets the env var on next deploy,
		// the load path migrates rows transparently.
		encAT, errA = Encrypt(accessToken, c.secret)
		encRT, errR = Encrypt(refreshToken, c.secret)
	}
	if errA != nil || errR != nil {
		log.Printf("Schwab: warning: failed to encrypt tokens for storage (at_err=%v, rt_err=%v)", errA, errR)
		return
	}
	if err := c.store.SaveOAuthToken("schwab", encAT, encRT, expiresAt); err != nil {
		log.Printf("Schwab: warning: failed to persist tokens: %v", err)
	}
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func (c *Client) tokenRequest(data url.Values) (*tokenResponse, error) {
	req, err := http.NewRequest("POST", baseURL+tokenPath, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	creds := base64.StdEncoding.EncodeToString([]byte(c.appKey + ":" + c.secret))
	req.Header.Set("Authorization", "Basic "+creds)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request returned %d: %s", resp.StatusCode, string(body))
	}

	var tokens tokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	return &tokens, nil
}

// AuthenticatedGet performs an authenticated GET request to the Schwab API.
func (c *Client) AuthenticatedGet(url string) (*http.Response, error) {
	return c.authenticatedRequest("GET", url, nil)
}

/*
AuthenticatedDo performs an arbitrary authenticated request. Used by
the Trader API client for POST (place order) and DELETE (cancel
order) calls. Body may be nil; if non-nil the caller must set the
reader and the Content-Type header gets defaulted to application/json
so callers don't have to repeat themselves.
*/
func (c *Client) AuthenticatedDo(method, url string, body io.Reader) (*http.Response, error) {
	return c.authenticatedRequest(method, url, body)
}

/*
authenticatedRequest wraps a Schwab API call in a small retry loop
for transient HTTP failures (429 rate-limited and 5xx gateway). Each
retry refreshes the access token before reissuing so a token that
expired mid-basket gets a clean second chance. 401/403 are NOT
retried (real auth failure, retrying just wastes time). Network
errors before any response are retried up to maxRetryAttempts as
well.

POST is NOT retried because it is not idempotent: Schwab Trader's
POST /accounts/{hash}/orders accepts the order on the broker side
before the response body is read, and a 5xx during response read +
automatic retry will submit a duplicate real-money order. For POST
the caller gets exactly one attempt and must reconcile broker state
itself (the reconcile cron picks up orphan orders if the response
fails after broker acceptance). GET / DELETE / PUT are safe to retry
under HTTP semantics; PATCH is also idempotent in practice for the
Schwab endpoints we touch (none today).
*/
func (c *Client) authenticatedRequest(method, url string, body io.Reader) (*http.Response, error) {
	const (
		maxRetryAttempts = 3
		baseBackoff      = 250 * time.Millisecond
	)

	maxAttempts := maxRetryAttempts
	if method == http.MethodPost {
		maxAttempts = 1
	}

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
	}

	var lastResp *http.Response
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		token, err := c.ValidToken()
		if err != nil {
			return nil, err
		}

		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequest(method, url, reqBody)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				time.Sleep(baseBackoff * time.Duration(1<<(attempt-1)))
				continue
			}
			return nil, err
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			_ = resp.Body.Close()
			lastResp = resp
			if attempt < maxAttempts {
				log.Printf("Schwab: retrying %s %s after HTTP %d (attempt %d/%d)", method, url, resp.StatusCode, attempt, maxAttempts)
				time.Sleep(baseBackoff * time.Duration(1<<(attempt-1)))
				continue
			}
		}

		return resp, nil
	}

	if lastResp != nil {
		return lastResp, nil
	}
	return nil, lastErr
}
