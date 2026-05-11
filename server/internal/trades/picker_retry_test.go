package trades

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// newAPIError builds an *anthropic.Error shaped like one the real SDK would
// return: StatusCode set plus non-nil Request/Response (the SDK's Error()
// method dereferences both when formatting, so the retry-loop log line
// would panic with a bare {StatusCode: ...}).
func newAPIError(status int) *anthropic.Error {
	return &anthropic.Error{
		StatusCode: status,
		Request:    &http.Request{Method: "POST", URL: &url.URL{Scheme: "https", Host: "api.anthropic.com", Path: "/v1/messages"}},
		Response:   &http.Response{StatusCode: status},
	}
}

func TestIsRetryableAPIError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain non-API error", errors.New("boom"), false},
		{"context canceled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
		{"400 bad request", newAPIError(400), false},
		{"401 auth", newAPIError(401), false},
		{"403 forbidden", newAPIError(403), false},
		{"404 not found", newAPIError(404), false},
		{"500 generic server error (not in retry list)", newAPIError(500), false},
		{"429 rate limit", newAPIError(429), true},
		{"502 bad gateway", newAPIError(502), true},
		{"503 unavailable", newAPIError(503), true},
		{"504 gateway timeout", newAPIError(504), true},
		{"529 overloaded", newAPIError(529), true},
		{"wrapped 529 still retryable", fmt.Errorf("anthropic messages.new: %w", newAPIError(529)), true},
		{"wrapped non-API stays non-retryable", fmt.Errorf("wrapped: %w", errors.New("boom")), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableAPIError(tc.err); got != tc.want {
				t.Fatalf("isRetryableAPIError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestBackoffDelay(t *testing.T) {
	// For each attempt the pre-jitter base delay is
	// min(pickerBackoffBase * 2^(attempt-1), pickerBackoffCeil),
	// and the returned value sits in [0.75*base, 1.25*base).
	cases := []struct {
		attempt int
		base    time.Duration
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second}, // 32s capped
		{6, 30 * time.Second}, // 64s capped
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("attempt=%d", tc.attempt), func(t *testing.T) {
			lo := tc.base - tc.base/4
			hi := tc.base + tc.base/4
			// Sample several times to exercise the jitter PRNG.
			for i := 0; i < 50; i++ {
				d := backoffDelay(tc.attempt)
				if d < lo || d >= hi {
					t.Fatalf("backoffDelay(%d) = %s, want in [%s, %s)", tc.attempt, d, lo, hi)
				}
			}
		})
	}
}

func TestRetryWithBackoff_SuccessFirstTry(t *testing.T) {
	var calls int32
	msg, err := retryWithBackoff(context.Background(), 5, noDelay, func() (*anthropic.Message, error) {
		atomic.AddInt32(&calls, 1)
		return &anthropic.Message{}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected non-nil message on success")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

func TestRetryWithBackoff_TransientThenSuccess(t *testing.T) {
	var calls int32
	msg, err := retryWithBackoff(context.Background(), 5, noDelay, func() (*anthropic.Message, error) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return nil, newAPIError(529)
		}
		return &anthropic.Message{}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected non-nil message after recovery")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 calls (2 retries + success), got %d", got)
	}
}

func TestRetryWithBackoff_NonRetryableFastFail(t *testing.T) {
	var calls int32
	want := newAPIError(401)
	_, err := retryWithBackoff(context.Background(), 5, noDelay, func() (*anthropic.Message, error) {
		atomic.AddInt32(&calls, 1)
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected non-retryable error to propagate, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call (no retry on 401), got %d", got)
	}
}

func TestRetryWithBackoff_AllAttemptsFail(t *testing.T) {
	var calls int32
	maxAttempts := 4
	_, err := retryWithBackoff(context.Background(), maxAttempts, noDelay, func() (*anthropic.Message, error) {
		atomic.AddInt32(&calls, 1)
		return nil, newAPIError(529)
	})
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 529 {
		t.Fatalf("expected final 529 error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != int32(maxAttempts) {
		t.Fatalf("expected %d calls, got %d", maxAttempts, got)
	}
}

func TestRetryWithBackoff_ContextCancelDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel shortly after the first attempt fails, while the retry
	// loop is parked in the select waiting on time.After.
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	var calls int32
	_, err := retryWithBackoff(ctx, 5, func(int) time.Duration { return time.Second }, func() (*anthropic.Message, error) {
		atomic.AddInt32(&calls, 1)
		return nil, newAPIError(529)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly 1 call before cancel, got %d", got)
	}
}

func noDelay(int) time.Duration { return 0 }
