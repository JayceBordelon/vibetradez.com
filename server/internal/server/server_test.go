package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"vibetradez.com/internal/sentiment"
	"vibetradez.com/internal/store"
	"vibetradez.com/internal/unsub"
)

/*
testDatabaseURL is the connection string used by every test in this
package. CI exports TEST_DATABASE_URL pointing at the workflow's
ephemeral Postgres service container; when the env var is unset
(developer running tests locally), fall back to the dev DB string.
*/
var testDatabaseURL = func() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgresql://jaycebordelon@localhost:5432/vibetradez_test?sslmode=disable"
}()

// Pre-baked 32-byte test key — fine to be static, no production secret.
var testUnsubKey = []byte("0123456789abcdef0123456789abcdef")

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := store.New(testDatabaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	db.RemoveAllForTest()

	return New(db, nil, nil, sentiment.NewScraper(), nil, "", "", "", "vt_session", 30*24*time.Hour, "0", nil, testUnsubKey, nil, "https://vibetradez.test")
}

/*
apiRequest is the internal /api/* test helper. The requireInternal
middleware on the API mux rejects every request without an X-VT-Source
header (403 forbidden) so external callers can't bypass the Next.js
proxy. Production traffic always carries the header because the
trading-frontend's API route adds it; tests have to mimic that.
*/
func apiRequest(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("X-VT-Source", "test")
	return req
}

func TestSubscribeEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	body, _ := json.Marshal(subscribeRequest{Email: "api@test.com", Name: "API User"})
	req := apiRequest(http.MethodPost, "/api/subscribe", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got: %+v", resp)
	}
}

func TestSubscribeInvalidEmail(t *testing.T) {
	srv := setupTestServer(t)

	body, _ := json.Marshal(subscribeRequest{Email: "notanemail", Name: "Bad"})
	req := apiRequest(http.MethodPost, "/api/subscribe", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubscribeMethodNotAllowed(t *testing.T) {
	srv := setupTestServer(t)

	req := apiRequest(http.MethodGet, "/api/subscribe", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSubscribeRejectsExternalCaller(t *testing.T) {
	srv := setupTestServer(t)

	body, _ := json.Marshal(subscribeRequest{Email: "external@test.com", Name: "External"})
	req := httptest.NewRequest(http.MethodPost, "/api/subscribe", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without X-VT-Source, got %d", w.Code)
	}
}

func TestUnsubscribeEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	body, _ := json.Marshal(subscribeRequest{Email: "unsub@test.com", Name: "Unsub"})
	req := apiRequest(http.MethodPost, "/api/subscribe", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	token := unsub.Sign(testUnsubKey, "unsub@test.com")
	body, _ = json.Marshal(unsubscribeRequest{Email: "unsub@test.com", Token: token})
	req = apiRequest(http.MethodPost, "/api/unsubscribe", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnsubscribeNotFound(t *testing.T) {
	srv := setupTestServer(t)

	token := unsub.Sign(testUnsubKey, "ghost@test.com")
	body, _ := json.Marshal(unsubscribeRequest{Email: "ghost@test.com", Token: token})
	req := apiRequest(http.MethodPost, "/api/unsubscribe", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUnsubscribeRejectsForgedToken(t *testing.T) {
	srv := setupTestServer(t)

	body, _ := json.Marshal(unsubscribeRequest{Email: "victim@test.com", Token: "obviously-forged"})
	req := apiRequest(http.MethodPost, "/api/unsubscribe", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for forged token, got %d", w.Code)
	}
}

/*
TestHealthEndpoint exercises the granular /health shape. In a test
env no downstream services are configured (no Anthropic key, no
Schwab client, signal scrapers can't reach the internet), so the
overall status is legitimately 503 and `ok` is false. We assert the
response SHAPE: JSON parses, includes the per-service breakdown, and
the api self-check reports ok. The deploy healthcheck job is what
gates on status=200 against production where downstreams ARE
configured.
*/
func TestHealthEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 200 or 503, got %d: %s", w.Code, w.Body.String())
	}

	var resp healthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode /health response: %v", err)
	}
	if resp.Uptime == "" {
		t.Error("expected uptime to be populated")
	}
	if resp.Services == nil {
		t.Fatal("expected services map to be populated")
	}
	if api, ok := resp.Services["api"]; !ok || api.Status != "ok" {
		t.Errorf("expected services.api.status=ok (self-check), got %+v", api)
	}
}
