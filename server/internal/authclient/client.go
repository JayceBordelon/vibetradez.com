// Package authclient is the trading server's client to auth.jaycebordelon.com.
// It wraps the three endpoints we use: POST /oauth/token, GET /oauth/verify,
// and POST /oauth/revoke. Verify responses are cached in-memory for a short
// window to avoid round-tripping the auth service on every /api/* request.
package authclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID         int64  `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	PictureURL string `json:"picture_url"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	User        User   `json:"user"`
}

type VerifyResponse struct {
	Active   bool   `json:"active"`
	ClientID string `json:"client_id"`
	User     User   `json:"user"`
}

type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	redirectURI  string
	http         *http.Client

	cache   map[string]cacheEntry
	cacheMu sync.RWMutex
	cacheT  time.Duration
}

type cacheEntry struct {
	user    User
	expires time.Time
}

func New(baseURL, clientID, clientSecret, redirectURI string) *Client {
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		http:         &http.Client{Timeout: 10 * time.Second},
		cache:        make(map[string]cacheEntry),
		cacheT:       60 * time.Second,
	}
}

func (c *Client) RedirectURI() string { return c.redirectURI }
func (c *Client) ClientID() string    { return c.clientID }

// Exchange swaps a one-shot auth code for an access token + user info.
func (c *Client) Exchange(ctx context.Context, code string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("redirect_uri", c.redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange status %d", resp.StatusCode)
	}

	var out TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &out, nil
}

// Verify introspects an access token. Returns (nil, nil) when the token is
// valid but the auth service reports it inactive, or an error on transport
// failure. Hits a 60s in-memory cache first.
func (c *Client) Verify(ctx context.Context, token string) (*User, error) {
	if token == "" {
		return nil, nil
	}

	c.cacheMu.RLock()
	if hit, ok := c.cache[token]; ok && time.Now().Before(hit.expires) {
		c.cacheMu.RUnlock()
		u := hit.user
		return &u, nil
	}
	c.cacheMu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/oauth/verify", nil)
	if err != nil {
		return nil, fmt.Errorf("build verify request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("verify request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		c.evict(token)
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verify status %d", resp.StatusCode)
	}

	var vr VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return nil, fmt.Errorf("decode verify: %w", err)
	}
	if !vr.Active {
		c.evict(token)
		return nil, nil
	}

	c.cacheMu.Lock()
	c.cache[token] = cacheEntry{user: vr.User, expires: time.Now().Add(c.cacheT)}
	c.cacheMu.Unlock()

	u := vr.User
	return &u, nil
}

// Revoke invalidates an access token on the auth service.
func (c *Client) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/oauth/revoke", nil)
	if err != nil {
		return fmt.Errorf("build revoke request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("revoke request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	c.evict(token)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("revoke status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) evict(token string) {
	c.cacheMu.Lock()
	delete(c.cache, token)
	c.cacheMu.Unlock()
}
