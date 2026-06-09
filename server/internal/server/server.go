package server

import (
	"context"
	crand "crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"

	"vibetradez.com/internal/auth"
	"vibetradez.com/internal/email"
	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/quotes"
	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/store"
	"vibetradez.com/internal/unsub"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

/*
sanitizeForLog strips ASCII control characters (CR, LF, NUL, etc.)
and trims outer whitespace before user-controlled values land in
log.Printf or apiResponse messages. Without this, an attacker
subscribing with `Name: "Foo\n[ERROR] forged log entry"` plants a
fake log line that any tail-based alerting will treat as a real
error. Applied to email + name inputs on the subscribe/unsubscribe
paths.
*/
func sanitizeForLog(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		// Keep printable Unicode; drop any control char (categories Cc / Cf).
		// '\t' is also dropped on purpose — logs shouldn't carry tab-aligned
		// attacker text.
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

type Server struct {
	db             *store.Store
	schwab         *schwab.Client
	auth           *auth.Service
	emailClient    *email.Client
	emailFrom      string
	anthropicKey   string
	anthropicModel string
	sessionCookie  string
	sessionTTL     time.Duration
	mux            *http.ServeMux
	port           string
	// Live auto-execution. nil = trading disabled at startup.
	executor *exec.Service
	// Streaming-quotes fan-out. nil = SSE endpoint disabled (no Schwab).
	quotesHub *quotes.Hub

	// Unsubscribe HMAC key + previous keys for rotation + the public-
	// facing URL used to build email links. The /auth/unsubscribe
	// handler validates ?t=<token> via unsub.VerifyWithFallback so
	// links signed with the previous key (still in subscriber inboxes
	// post-rotation) keep working.
	unsubscribeKey     []byte
	unsubscribePrevKey [][]byte
	publicBaseURL      string
}

type subscribeRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type unsubscribeRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

type apiResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func New(db *store.Store, schwabClient *schwab.Client, authService *auth.Service, emailClient *email.Client, emailFrom, anthropicKey, anthropicModel, sessionCookie string, sessionTTL time.Duration, port string, executor *exec.Service, unsubscribeKey []byte, unsubscribePrevKey [][]byte, publicBaseURL string) *Server {
	s := &Server{
		db:                 db,
		schwab:             schwabClient,
		auth:               authService,
		emailClient:        emailClient,
		emailFrom:          emailFrom,
		anthropicKey:       anthropicKey,
		anthropicModel:     anthropicModel,
		sessionCookie:      sessionCookie,
		sessionTTL:         sessionTTL,
		mux:                http.NewServeMux(),
		port:               port,
		executor:           executor,
		unsubscribeKey:     unsubscribeKey,
		unsubscribePrevKey: unsubscribePrevKey,
		publicBaseURL:      strings.TrimRight(publicBaseURL, "/"),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	/*
		Per-endpoint rate limiters. Buckets are per-source-IP; keys come
		from clientIP (X-Forwarded-For first hop, set by Traefik in prod).
		  - subscribe: anti-spam (1/min — humans never re-subscribe)
		  - auth:      anti-brute-force on OAuth start endpoints
		  - unsub:     public endpoint hit via email links; 30/min/IP is
		               generous for a single user but blocks token-
		               enumeration storms
	*/
	subscribeLimit := newIPLimiter(1, 3)
	authLimit := newIPLimiter(10, 5)
	unsubLimit := newIPLimiter(30, 10)

	s.mux.HandleFunc("/health", s.handleHealth)
	// /healthz is a lightweight liveness probe — db ping only, no
	// external upstream probes. Wired to the docker-compose healthcheck
	// so a slow Schwab / Anthropic response can't trip a container
	// restart cascade. The richer /health still runs probes for the
	// post-deploy healthcheck.yml workflow.
	s.mux.HandleFunc("/healthz", s.handleLiveness)
	s.mux.HandleFunc("/auth/unsubscribe", unsubLimit.middleware(s.handleUnsubscribeLink))
	s.mux.HandleFunc("/auth/schwab", authLimit.middleware(s.handleSchwabAuth))
	// Callback is gated by the state-cookie check inside the handler, but
	// also rate-limited so a flood of bogus callbacks can't churn the
	// token-exchange path.
	s.mux.HandleFunc("/auth/callback", authLimit.middleware(s.handleSchwabCallback))
	// In-process Google OAuth. Callback registered at vibetradez.com.
	s.mux.HandleFunc("/auth/google/start", authLimit.middleware(s.auth.HandleStart))
	s.mux.HandleFunc("/auth/google/callback", authLimit.middleware(s.auth.HandleCallback(s.sessionCookie)))
	s.mux.HandleFunc("/auth/logout", s.auth.HandleLogout(s.sessionCookie))

	// API routes — require internal header (requests must come from the website)
	s.mux.HandleFunc("/api/subscribe", requireInternal(subscribeLimit.middleware(s.handleSubscribe)))
	s.mux.HandleFunc("/api/unsubscribe", requireInternal(subscribeLimit.middleware(s.handleUnsubscribe)))
	s.mux.HandleFunc("/api/me", requireInternal(s.auth.AttachUser(s.sessionCookie, s.handleMe)))
	s.mux.HandleFunc("/api/portfolio", requireInternal(s.handlePortfolio))
	s.mux.HandleFunc("/api/portfolio/equity-curve", requireInternal(s.handlePortfolioEquityCurve))
	s.mux.HandleFunc("/api/portfolio/holdings", requireInternal(s.handlePortfolioHoldings))
	s.mux.HandleFunc("/api/portfolio/closed", requireInternal(s.handlePortfolioClosed))
	s.mux.HandleFunc("/api/portfolio/position-history", requireInternal(s.handlePortfolioPositionHistory))
	s.mux.HandleFunc("/api/price-history", requireInternal(s.handlePriceHistory))
	s.mux.HandleFunc("/api/transcript", requireInternal(s.handleTranscript))
	s.mux.HandleFunc("/api/market/status", requireInternal(s.handleMarketStatus))
	s.mux.HandleFunc("/api/quotes/stream", requireInternal(s.handleQuotesStream))
}

func (s *Server) Start() {
	addr := ":" + s.port
	log.Printf("API server listening on %s", addr)
	/*
		Wrap mux in baseline security headers (HSTS, X-Frame-Options,
		X-Content-Type-Options, Referrer-Policy, Permissions-Policy).
		Defense-in-depth — each closes a door an attacker would otherwise
		have ajar even though SameSite cookies + origin model already
		block the dominant attack classes.
	*/
	handler := securityHeaders(s.mux)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("API server error: %v", err)
	}
}

/*
requireInternal rejects requests to /api/* that don't include the internal header.
This prevents direct external API access — callers must go through the website.
*/
func requireInternal(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-VT-Source") == "" {
			writeJSON(w, http.StatusForbidden, apiResponse{OK: false, Message: "forbidden"})
			return
		}
		next(w, r)
	}
}

/*
transcriptResponse is the wire shape for GET /api/transcript. Available
is false (with an empty Events array) when no transcript exists for the
date+kind, so the client renders a clean "no transcript yet" state
rather than treating a missing row as an error — same convention the
trades endpoints use for empty days. Events is the raw stored JSONB
event array (ordered), passed through verbatim.
*/
type transcriptResponse struct {
	Date       string          `json:"date"`
	Kind       string          `json:"kind"`
	Model      string          `json:"model"`
	Available  bool            `json:"available"`
	CreatedAt  string          `json:"created_at,omitempty"`
	Events     json.RawMessage `json:"events"`
	Usage      json.RawMessage `json:"usage"`
	DurationMS int64           `json:"duration_ms"`
}

/*
handleTranscript serves the captured model conversation for a given
trading day and kind (e.g. "portfolio" for the daily session). It is the
same single public account the dashboard already shows, so the transcript
is served verbatim on the internal-header-gated surface, balances included.
*/
func (s *Server) handleTranscript(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	kind := r.URL.Query().Get("kind")

	if kind != "selection" && kind != "execution" && kind != "portfolio" {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "kind must be 'selection', 'execution', or 'portfolio'"})
		return
	}
	if date == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "date query param required (YYYY-MM-DD)"})
		return
	}

	tv, err := s.db.GetTranscript(date, kind)
	if err != nil {
		log.Printf("handleTranscript: GetTranscript(%s, %s): %v", date, kind, err)
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Message: "failed to load transcript"})
		return
	}

	if tv == nil {
		writeJSON(w, http.StatusOK, transcriptResponse{
			Date:      date,
			Kind:      kind,
			Available: false,
			Events:    json.RawMessage("[]"),
			Usage:     json.RawMessage("{}"),
		})
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=30")
	// Never emit events:null. A session that errored before any move (or any
	// nil-slice JSONB) is stored as null/empty; coerce to an empty array so the
	// client always receives a list and renders the empty state instead of
	// crashing on events.length.
	events := tv.Events
	if len(events) == 0 || strings.TrimSpace(string(events)) == "null" {
		events = json.RawMessage("[]")
	}
	usage := tv.Usage
	if len(usage) == 0 || strings.TrimSpace(string(usage)) == "null" {
		usage = json.RawMessage("{}")
	}
	writeJSON(w, http.StatusOK, transcriptResponse{
		Date:       tv.Date,
		Kind:       tv.Kind,
		Model:      tv.Model,
		Available:  true,
		CreatedAt:  tv.CreatedAt.UTC().Format(time.RFC3339),
		Events:     events,
		Usage:      usage,
		DurationMS: tv.DurationMS,
	})
}

func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Message: "method not allowed"})
		return
	}

	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "invalid JSON body"})
		return
	}

	req.Email = sanitizeForLog(strings.ToLower(req.Email))
	req.Name = sanitizeForLog(req.Name)

	if req.Email == "" || !emailRegex.MatchString(req.Email) {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "valid email is required"})
		return
	}

	if err := s.db.AddSubscriber(req.Email, req.Name); err != nil {
		log.Printf("Error adding subscriber %s: %v", req.Email, err)
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Message: "failed to subscribe"})
		return
	}

	log.Printf("New subscriber: %s (%s)", req.Email, req.Name)
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Message: "subscribed successfully"})
}

/*
handleUnsubscribe is the JSON-API path for the web form. Was a
guess-the-email mass-unsub vector — anyone who knew or guessed a
subscriber's email could POST `{"email": "…"}` and remove them,
gated only by a defeatable per-IP rate limit.

Now requires a valid HMAC token signed by the server (mintEd via
unsub.Sign with the operator's UNSUBSCRIBE_HMAC_KEY). The website's
UnsubscribeForm no longer hits this endpoint directly — users
unsubscribe by clicking the link in any vibetradez.com email,
which lands on GET /auth/unsubscribe with the token already in the
URL.
*/
func (s *Server) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Message: "method not allowed"})
		return
	}

	var req unsubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "invalid JSON body"})
		return
	}

	req.Email = sanitizeForLog(strings.ToLower(req.Email))

	if req.Email == "" || req.Token == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "email and token are required (use the unsubscribe link from any vibetradez.com email)"})
		return
	}
	if !unsub.VerifyWithFallback(s.unsubscribeKey, s.unsubscribePrevKey, req.Email, req.Token) {
		writeJSON(w, http.StatusForbidden, apiResponse{OK: false, Message: "invalid unsubscribe token"})
		return
	}

	if err := s.db.RemoveSubscriber(req.Email); err != nil {
		writeJSON(w, http.StatusNotFound, apiResponse{OK: false, Message: err.Error()})
		return
	}

	log.Printf("Unsubscribed (api): %s", req.Email)
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Message: "unsubscribed successfully"})
}

/*
handleUnsubscribeLink is the public endpoint reached by clicking the
unsubscribe link in any vibetradez.com email. Method semantics:

  - GET renders a confirmation page with a POST-button form. The
    token + email are validated for shape only; the actual unsub
    happens on POST. This pattern avoids "side-effect GET" failures
    where Gmail / Outlook / Slack link-preview prefetch bots
    accidentally unsubscribe users on forward or paste.

  - POST performs the actual unsub. Triggered either by clicking the
    confirmation page's button OR by Gmail / Apple Mail's native
    one-click unsub UI (driven by the List-Unsubscribe-Post header
    we set in resend.go).

Token validation is shared. The 30/min/IP rate limit at the route
layer applies to both methods.
*/
func (s *Server) handleUnsubscribeLink(w http.ResponseWriter, r *http.Request) {
	var email, token string
	switch r.Method {
	case http.MethodGet:
		email = sanitizeForLog(strings.ToLower(r.URL.Query().Get("e")))
		token = r.URL.Query().Get("t")
	case http.MethodPost:
		// Parse both URL query (RFC 8058 one-click — Gmail sends the
		// e + t in the URL with empty body) AND form body (fallback
		// from the on-page confirmation form).
		_ = r.ParseForm()
		email = sanitizeForLog(strings.ToLower(firstNonEmpty(r.URL.Query().Get("e"), r.PostFormValue("e"))))
		token = firstNonEmpty(r.URL.Query().Get("t"), r.PostFormValue("t"))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if email == "" || token == "" {
		renderUnsubPage(w, http.StatusBadRequest, "Unsubscribe link incomplete", "The link you clicked is missing required parameters. Please use the link from a recent vibetradez.com email.", "", "")
		return
	}
	if !emailRegex.MatchString(email) {
		renderUnsubPage(w, http.StatusBadRequest, "Unsubscribe link invalid", "The email in this link doesn't look like a valid address. If you got it from a vibetradez.com email please reply and we'll unsubscribe you manually.", "", "")
		return
	}
	if !unsub.VerifyWithFallback(s.unsubscribeKey, s.unsubscribePrevKey, email, token) {
		renderUnsubPage(w, http.StatusForbidden, "Unsubscribe link invalid", "We couldn't verify this unsubscribe link. It may have been altered or the link may be from before a key rotation. Please use the link from a recent vibetradez.com email or reply to the email and we'll unsubscribe you manually.", "", "")
		return
	}

	if r.Method == http.MethodGet {
		// Render the confirmation page; user clicks the button to POST.
		renderUnsubPage(w, http.StatusOK, "Confirm unsubscribe", "Click below to remove this email from the vibetradez.com daily picks list. You can re-subscribe at any time from the website.", email, token)
		return
	}

	// POST — token verified, actually do it.
	if err := s.db.RemoveSubscriber(email); err != nil {
		log.Printf("Unsubscribe link removeSubscriber: %v", err)
		// Mirror the success-page copy so a stale link doesn't leak an
		// email-exists oracle to anyone holding a valid token.
		renderUnsubPage(w, http.StatusOK, "Unsubscribed", "You won't receive further updates from vibetradez.com.", "", "")
		return
	}
	log.Printf("Unsubscribed (link): %s", email)
	renderUnsubPage(w, http.StatusOK, "Unsubscribed", "You won't receive further updates from vibetradez.com. Sorry to see you go.", "", "")
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

/*
renderUnsubPage emits the unsubscribe HTML surface. When confirmEmail
+ confirmToken are non-empty, a POST-button form is rendered so the
user can complete the unsub with one click. The form posts to
/auth/unsubscribe with the email + token as form fields; the POST
handler does the actual subscribers.active flip.

All user-controlled values are HTML-escaped before emit — emailRegex
already gates the input to a safe character set but the escape is
defense-in-depth so a future regex relaxation doesn't open an XSS
surface in the form's value=" " attribute.
*/
func renderUnsubPage(w http.ResponseWriter, status int, title, body, confirmEmail, confirmToken string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	form := ""
	if confirmEmail != "" && confirmToken != "" {
		form = fmt.Sprintf(`
<form method="POST" action="/auth/unsubscribe" style="margin:24px 0 8px 0">
<input type="hidden" name="e" value="%s">
<input type="hidden" name="t" value="%s">
<button type="submit" style="background:#ef4444;color:#fff;border:none;border-radius:8px;padding:12px 28px;font-size:14px;font-weight:600;cursor:pointer">Unsubscribe %s</button>
</form>`, html.EscapeString(confirmEmail), html.EscapeString(confirmToken), html.EscapeString(confirmEmail))
	}
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s | VibeTradez</title>
<style>
  body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#0a0a0a;color:#e5e5e5;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;padding:24px}
  main{max-width:520px;background:#171717;border:1px solid #262626;border-radius:12px;padding:32px;text-align:center}
  h1{margin:0 0 12px 0;font-size:22px;font-weight:700}
  p{margin:0 0 16px 0;font-size:15px;line-height:1.55;color:#a3a3a3}
  a{color:#22c55e;text-decoration:none;font-weight:600}
</style>
</head><body><main>
<h1>%s</h1>
<p>%s</p>%s
<p><a href="https://vibetradez.com/">Back to vibetradez.com</a></p>
</main></body></html>`, html.EscapeString(title), html.EscapeString(title), html.EscapeString(body), form)
}

type serviceHealth struct {
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

type healthResponse struct {
	OK       bool                     `json:"ok"`
	Uptime   string                   `json:"uptime"`
	Services map[string]serviceHealth `json:"services"`
}

var serverStartTime = time.Now()

/*
handleLiveness is the lightweight liveness probe wired to the
docker-compose healthcheck. Returns 200 if the process is responsive
and the DB is reachable, 503 otherwise. No external upstream probes
(Schwab, Anthropic, signal scrapers) — those live on /health,
which the post-deploy healthcheck.yml workflow exercises. Without
this split, the compose healthcheck's 5s timeout would trip on the
first slow Schwab response and trigger a restart cascade.
*/
func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("db unreachable\n"))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	services := make(map[string]serviceHealth)
	allOK := true

	// Database
	dbStart := time.Now()
	if err := s.db.Ping(); err != nil {
		services["database"] = serviceHealth{Status: "fail", Detail: err.Error(), Latency: fmtLatency(time.Since(dbStart))}
		allOK = false
	} else {
		services["database"] = serviceHealth{Status: "ok", Detail: "PostgreSQL connected", Latency: fmtLatency(time.Since(dbStart))}
	}

	// Anthropic (Claude picker)
	anthropicStart := time.Now()
	anthropicHealth := s.checkAnthropic()
	anthropicHealth.Latency = fmtLatency(time.Since(anthropicStart))
	services["anthropic"] = anthropicHealth
	if anthropicHealth.Status == "fail" {
		allOK = false
	}

	/*
		Schwab Market Data Production — token freshness check. The token
		is shared with the Trading API, but this slot only proves the
		market-data side is reachable (which is what the live quotes /
		option chain code paths actually depend on).

		Severity is `fail` on any token issue (rejected refresh, not
		authorized). The token is shared with auto-execution, so when
		it's dead the morning cron silently produces picks no one can
		act on, the dashboard live cards die, and chart endpoints
		return empty. `warn` masked all of that behind a green-ish
		health row, so we now surface it as `fail` and let the deploy
		pipeline gate on it. "Not configured" stays `warn` since
		that's an env state, not a token state.
	*/
	if s.schwab != nil {
		if s.schwab.IsConnected() {
			tokStart := time.Now()
			if _, err := s.schwab.ValidToken(); err != nil {
				services["schwab_market_data"] = serviceHealth{
					Status:  "fail",
					Detail:  "refresh token rejected, visit /auth/schwab to re-authorize: " + err.Error(),
					Latency: fmtLatency(time.Since(tokStart)),
				}
				allOK = false
			} else {
				services["schwab_market_data"] = serviceHealth{Status: "ok", Detail: "Authenticated", Latency: fmtLatency(time.Since(tokStart))}
			}
		} else {
			services["schwab_market_data"] = serviceHealth{Status: "fail", Detail: "Configured but not authorized, visit /auth/schwab"}
			allOK = false
		}
	} else {
		services["schwab_market_data"] = serviceHealth{Status: "warn", Detail: "Not configured"}
	}

	/*
		Schwab Accounts and Trading Production — verifies the OAuth token
		has the Trading product scope by hitting the accountNumbers
		endpoint. Any token failure is `fail` and trips allOK so the deploy
		healthcheck blocks: this account trades live, so live orders WILL be
		attempted and they bounce without trading scope.
	*/
	tradingHealth := s.checkSchwabTrading(r.Context())
	services["schwab_trading"] = tradingHealth
	if tradingHealth.Status == "fail" {
		allOK = false
	}

	// API (self-check)
	services["api"] = serviceHealth{Status: "ok", Detail: fmt.Sprintf("Listening on :%s", s.port)}

	uptime := time.Since(serverStartTime).Truncate(time.Second).String()

	status := http.StatusOK
	if !allOK {
		status = http.StatusServiceUnavailable
	}

	writeJSON(w, status, healthResponse{
		OK:       allOK,
		Uptime:   uptime,
		Services: services,
	})
}

/*
isStubKey returns true for the placeholder ANTHROPIC_API_KEY used by the
local Docker stack so the server boots without making real API calls.
*/
func isStubKey(k string) bool {
	if k == "" {
		return false
	}
	if strings.HasPrefix(k, "stub-") || strings.HasPrefix(k, "sk_local") || strings.HasPrefix(k, "sk-local") {
		return true
	}
	return false
}

func (s *Server) checkAnthropic() serviceHealth {
	if s.anthropicKey == "" {
		return serviceHealth{Status: "fail", Detail: "API key not configured"}
	}
	if isStubKey(s.anthropicKey) {
		return serviceHealth{Status: "warn", Detail: "Local stub key — skipping live probe"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := anthropic.NewClient(anthropicoption.WithAPIKey(s.anthropicKey))
	// 1 max token + 1 char prompt is the cheapest possible probe.
	_, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_6,
		MaxTokens: 1,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("ok")),
		},
	})
	if err != nil {
		return serviceHealth{Status: "fail", Detail: err.Error()}
	}
	return serviceHealth{Status: "ok", Detail: "Anthropic API reachable"}
}

/*
checkSchwabTrading verifies the OAuth token covers the "Accounts and
Trading Production" Schwab product. Hits /trader/v1/accounts/
accountNumbers, the lightest endpoint on the Trader API surface.

Any token-related failure (not authorized, refresh rejected, 401/403
on the scoped probe, non-200 from Schwab) returns `fail`. A
green-looking row when the token is dead hid today's outage from the
deploy email. "Not configured" stays `warn` since that's an env state,
not a token state.
*/
func (s *Server) checkSchwabTrading(ctx context.Context) serviceHealth {
	if s.schwab == nil {
		return serviceHealth{Status: "warn", Detail: "Not configured"}
	}
	if !s.schwab.IsConnected() {
		return serviceHealth{Status: "fail", Detail: "Schwab OAuth not authorized, visit /auth/schwab"}
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = probeCtx // reserved for future use; AuthenticatedDo doesn't take ctx today

	start := time.Now()
	resp, err := s.schwab.AuthenticatedDo("GET", "https://api.schwabapi.com/trader/v1/accounts/accountNumbers", nil)
	if err != nil {
		return serviceHealth{Status: "fail", Detail: "request failed: " + err.Error(), Latency: fmtLatency(time.Since(start))}
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case 200:
		return serviceHealth{Status: "ok", Detail: "Trading scope active", Latency: fmtLatency(time.Since(start))}
	case 401, 403:
		/*
			Token doesn't have trading scope. Most common cause: app was
			market-data-only when the user authorized; trading product was
			added later but the token wasn't refreshed via /auth/schwab.
		*/
		return serviceHealth{
			Status:  "fail",
			Detail:  fmt.Sprintf("HTTP %d, token lacks trading scope; re-run /auth/schwab", resp.StatusCode),
			Latency: fmtLatency(time.Since(start)),
		}
	default:
		return serviceHealth{
			Status:  "fail",
			Detail:  fmt.Sprintf("HTTP %d, Schwab Trader API unreachable", resp.StatusCode),
			Latency: fmtLatency(time.Since(start)),
		}
	}
}

func fmtLatency(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dμs", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}

// ── Schwab OAuth ──

const schwabStateCookie = "vt_schwab_state"

/*
handleSchwabAuth kicks off the Schwab OAuth flow. Mints a 32-byte
state token, double-submits it as a host-scoped cookie (Path=/auth so
it's readable on /auth/callback), and includes it in the URL Schwab
will redirect to. Without this, a crafted /auth/callback?code=...
link can rebind the production trader to an attacker's Schwab
account if the operator ever clicks it.
*/
func (s *Server) handleSchwabAuth(w http.ResponseWriter, r *http.Request) {
	if s.schwab == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiResponse{OK: false, Message: "Schwab not configured"})
		return
	}

	b := make([]byte, 32)
	if _, err := crand.Read(b); err != nil {
		http.Error(w, "schwab auth init failed", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(b)

	http.SetCookie(w, &http.Cookie{
		// Path="/auth/callback" instead of "/auth": the cookie is read
		// only by handleSchwabCallback, so narrower scope keeps it out
		// of every /auth/google/* and /auth/unsubscribe request that
		// doesn't need it (cookie hygiene, no functional change).
		Name:     schwabStateCookie,
		Value:    state,
		Path:     "/auth/callback",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, s.schwab.AuthorizationURL(state), http.StatusFound)
}

func (s *Server) handleSchwabCallback(w http.ResponseWriter, r *http.Request) {
	if s.schwab == nil {
		http.Error(w, "Schwab not configured", http.StatusServiceUnavailable)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" {
		log.Printf("Schwab callback: no code param. Full query: %s", r.URL.RawQuery)
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	c, err := r.Cookie(schwabStateCookie)
	if err != nil || c.Value == "" || state == "" ||
		subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		log.Printf("Schwab callback: state mismatch (cookie_present=%v, query_state_present=%v)", err == nil && c != nil && c.Value != "", state != "")
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	// Burn the state cookie regardless of token-exchange outcome —
	// states are one-shot.
	http.SetCookie(w, &http.Cookie{
		// Path="/auth/callback" instead of "/auth": the cookie is read
		// only by handleSchwabCallback, so narrower scope keeps it out
		// of every /auth/google/* and /auth/unsubscribe request that
		// doesn't need it (cookie hygiene, no functional change).
		Name:     schwabStateCookie,
		Value:    "",
		Path:     "/auth/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	log.Printf("Schwab callback: received code (%d chars), exchanging for tokens...", len(code))
	if err := s.schwab.ExchangeCode(code); err != nil {
		log.Printf("Schwab OAuth error: %v", err)
		http.Error(w, "OAuth token exchange failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Schwab OAuth: successfully connected")

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// Live-quote serving has been removed from this server: there is no
// /api/quotes/* route today (neither the old REST polling endpoint nor an
// SSE streaming handler). The dashboard renders marks from the portfolio
// snapshot endpoints. The schwab quote cache that remains is used
// server-side by the portfolio reader's liquidity checks, not over HTTP.

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
