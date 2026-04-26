package schwab

import (
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
}

type Client struct {
	appKey      string
	secret      string
	callbackURL string
	store       TokenStore
	httpClient  *http.Client

	mu          sync.RWMutex
	accessToken string
	refreshTok  string
	expiresAt   time.Time
}

func NewClient(appKey, secret, callbackURL string, store TokenStore) *Client {
	c := &Client{
		appKey:      appKey,
		secret:      secret,
		callbackURL: callbackURL,
		store:       store,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
	// Load persisted tokens on startup (stored encrypted).
	if encAT, encRT, exp, err := store.GetOAuthToken("schwab"); err == nil && encRT != "" {
		at, errA := Decrypt(encAT, secret)
		rt, errR := Decrypt(encRT, secret)
		if errA == nil && errR == nil {
			c.accessToken = at
			c.refreshTok = rt
			c.expiresAt = exp
			log.Printf("Schwab: loaded persisted tokens (expires %s)", exp.Format(time.RFC3339))
		} else {
			log.Printf("Schwab: failed to decrypt persisted tokens, re-authorization required")
		}
	}
	return c
}

// IsConnected returns true if we have a refresh token (may still need refreshing).
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.refreshTok != ""
}

// AuthorizationURL returns the URL to redirect the user to for OAuth authorization.
func (c *Client) AuthorizationURL() string {
	return fmt.Sprintf("%s%s?response_type=code&client_id=%s&redirect_uri=%s",
		baseURL, authPath,
		url.QueryEscape(c.appKey),
		url.QueryEscape(c.callbackURL),
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

	c.mu.Lock()
	c.accessToken = tokens.AccessToken
	c.refreshTok = tokens.RefreshToken
	c.expiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	c.mu.Unlock()

	c.persistTokens(tokens.AccessToken, tokens.RefreshToken, c.expiresAt)
	log.Println("Schwab: OAuth tokens obtained successfully")
	return nil
}

// ValidToken returns a valid access token, refreshing if necessary.
func (c *Client) ValidToken() (string, error) {
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

func (c *Client) doRefresh(refreshToken string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check under write lock (another goroutine may have refreshed).
	if c.accessToken != "" && time.Now().Before(c.expiresAt.Add(-tokenGrace)) {
		return c.accessToken, nil
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	tokens, err := c.tokenRequest(data)
	if err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}

	c.accessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		c.refreshTok = tokens.RefreshToken
	}
	c.expiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)

	c.persistTokens(c.accessToken, c.refreshTok, c.expiresAt)
	log.Println("Schwab: access token refreshed")
	return c.accessToken, nil
}

func (c *Client) persistTokens(accessToken, refreshToken string, expiresAt time.Time) {
	encAT, errA := Encrypt(accessToken, c.secret)
	encRT, errR := Encrypt(refreshToken, c.secret)
	if errA != nil || errR != nil {
		log.Printf("Schwab: warning: failed to encrypt tokens for storage")
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
	token, err := c.ValidToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

/*
AuthenticatedDo performs an arbitrary authenticated request. Used by
the Trader API client for POST (place order) and DELETE (cancel
order) calls. Body may be nil; if non-nil the caller must set the
reader and the Content-Type header gets defaulted to application/json
so callers don't have to repeat themselves.
*/
func (c *Client) AuthenticatedDo(method, url string, body io.Reader) (*http.Response, error) {
	token, err := c.ValidToken()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}
