package schwab

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

/*
newTestClient wires a Schwab Client with a stubbed HTTP transport and a
pre-warmed access token so authenticatedRequest can run without a real
token-exchange round-trip.
*/
func newTestClient(rt http.RoundTripper) *Client {
	return &Client{
		httpClient:  &http.Client{Transport: rt, Timeout: 5 * time.Second},
		accessToken: "test-access-token",
		refreshTok:  "test-refresh-token",
		expiresAt:   time.Now().Add(time.Hour),
	}
}

func newResp(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("{}")),
		Header:     make(http.Header),
	}
}

func TestAuthenticatedRequest_RetriesGetOn5xx(t *testing.T) {
	var calls int32
	c := newTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return newResp(http.StatusBadGateway), nil
		}
		return newResp(http.StatusOK), nil
	}))

	resp, err := c.authenticatedRequest("GET", "https://example.test/orders", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected final 200, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts on GET 502, got %d", got)
	}
}

func TestAuthenticatedRequest_DoesNotRetryPostOn5xx(t *testing.T) {
	var calls int32
	c := newTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return newResp(http.StatusBadGateway), nil
	}))

	resp, err := c.authenticatedRequest("POST", "https://example.test/orders", strings.NewReader(`{"x":1}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected first-attempt 502 returned to caller, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly 1 attempt on POST 502 (no retry — order may have landed at broker), got %d", got)
	}
}

func TestAuthenticatedRequest_DoesNotRetryPostOn429(t *testing.T) {
	var calls int32
	c := newTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return newResp(http.StatusTooManyRequests), nil
	}))

	_, err := c.authenticatedRequest("POST", "https://example.test/orders", strings.NewReader(`{"x":1}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 attempt on POST 429, got %d", got)
	}
}

func TestAuthenticatedRequest_RetriesDeleteOn5xx(t *testing.T) {
	var calls int32
	c := newTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			return newResp(http.StatusServiceUnavailable), nil
		}
		return newResp(http.StatusOK), nil
	}))

	resp, err := c.authenticatedRequest("DELETE", "https://example.test/orders/123", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after retry, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 attempts on DELETE 503, got %d", got)
	}
}

func TestAuthenticatedRequest_PassesThroughOn200(t *testing.T) {
	var calls int32
	c := newTestClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return newResp(http.StatusCreated), nil
	}))

	resp, err := c.authenticatedRequest("POST", "https://example.test/orders", strings.NewReader(`{"x":1}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 attempt on success, got %d", got)
	}
}
