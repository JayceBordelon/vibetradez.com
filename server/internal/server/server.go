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
	"math"
	"math/rand/v2"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"

	"vibetradez.com/internal/authclient"
	"vibetradez.com/internal/email"
	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/quotes"
	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/sentiment"
	"vibetradez.com/internal/store"
	"vibetradez.com/internal/trades"
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
	auth           *authclient.Client
	scraper        *sentiment.Scraper
	emailClient    *email.Client
	emailFrom      string
	anthropicKey   string
	anthropicModel string
	sessionCookie  string
	sessionTTL     time.Duration
	// SSO consumer config — identifies this app to auth.jaycebordelon.com.
	ssoPublicURL   string // https://auth.jaycebordelon.com (browser-facing)
	ssoClientID    string
	ssoRedirectURI string
	mux            *http.ServeMux
	port           string
	// Auto-execution (paper or live). nil = trading disabled at startup.
	executor      *exec.Service
	executorEmail string // email allowlist for /api/execution/* (single user)

	// Unsubscribe HMAC key + previous keys for rotation + the public-
	// facing URL used to build email links. The /auth/unsubscribe
	// handler validates ?t=<token> via unsub.VerifyWithFallback so
	// links signed with the previous key (still in subscriber inboxes
	// post-rotation) keep working.
	unsubscribeKey     []byte
	unsubscribePrevKey [][]byte
	publicBaseURL      string

	// Live-quotes hub. Set via SetHub after construction since the hub
	// needs the DB (which lives in this server) at build time.
	hub *quotes.Hub
}

// SetHub wires the live-quotes streaming hub. Must be called after New
// and before Start. Idempotent.
func (s *Server) SetHub(h *quotes.Hub) { s.hub = h }

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

func New(db *store.Store, schwabClient *schwab.Client, authClient *authclient.Client, scraper *sentiment.Scraper, emailClient *email.Client, emailFrom, anthropicKey, anthropicModel, sessionCookie string, sessionTTL time.Duration, ssoPublicURL, ssoClientID, ssoRedirectURI, port string, executor *exec.Service, executorEmail string, unsubscribeKey []byte, unsubscribePrevKey [][]byte, publicBaseURL string) *Server {
	s := &Server{
		db:                 db,
		schwab:             schwabClient,
		auth:               authClient,
		scraper:            scraper,
		emailClient:        emailClient,
		emailFrom:          emailFrom,
		anthropicKey:       anthropicKey,
		anthropicModel:     anthropicModel,
		sessionCookie:      sessionCookie,
		sessionTTL:         sessionTTL,
		ssoPublicURL:       strings.TrimRight(ssoPublicURL, "/"),
		ssoClientID:        ssoClientID,
		ssoRedirectURI:     ssoRedirectURI,
		mux:                http.NewServeMux(),
		port:               port,
		executor:           executor,
		executorEmail:      executorEmail,
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
		Tuned for the actual access pattern, not arbitrary defaults:
		  - subscribe: anti-spam (1/min — humans never re-subscribe)
		  - auth: anti-brute-force (10/min on OAuth start endpoints)
		  - execution: anti-DoS on the high-stakes confirm + cancel-all
		    endpoints (5/min each — even authenticated user wouldn't
		    legitimately hit them more than once per real decision)
	*/
	subscribeLimit := newIPLimiter(1, 3) // 1/min, 3-burst (initial signup)
	authLimit := newIPLimiter(10, 5)     // 10/min, 5-burst (OAuth retries)
	executionLimit := newIPLimiter(5, 3) // 5/min, 3-burst (HMAC tokens unbruteable; this is just DoS bound)

	// Unsubscribe rate limiter — public endpoint (no X-VT-Source gate)
	// hit via email links. 30/min/IP is generous for a single user but
	// blocks token-enumeration storms.
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
	s.mux.HandleFunc("/auth/sso/start", authLimit.middleware(s.handleSSOStart))
	s.mux.HandleFunc("/auth/sso/callback", s.handleSSOCallback)
	s.mux.HandleFunc("/auth/logout", s.handleLogout)

	// API routes — require internal header (requests must come from the website)
	s.mux.HandleFunc("/api/subscribe", requireInternal(subscribeLimit.middleware(s.handleSubscribe)))
	s.mux.HandleFunc("/api/unsubscribe", requireInternal(subscribeLimit.middleware(s.handleUnsubscribe)))
	s.mux.HandleFunc("/api/me", requireInternal(s.attachUser(s.handleMe)))
	s.mux.HandleFunc("/api/trades/today", requireInternal(s.handleTradesToday))
	s.mux.HandleFunc("/api/trades/dates", requireInternal(s.handleTradeDates))
	s.mux.HandleFunc("/api/trades/week", requireInternal(s.handleTradesWeek))
	s.mux.HandleFunc("/api/chart/", requireInternal(s.handleChart))
	s.mux.HandleFunc("/api/market/status", requireInternal(s.handleMarketStatus))
	/*
		SSE stream — intentionally NOT gated by requireInternal. The
		browser EventSource API cannot set custom headers, so a route
		behind X-VT-Source is unreachable from the dashboard. Cross-
		origin abuse is naturally blocked by the absence of CORS
		headers on the response: a malicious-origin EventSource fails
		the handshake. Read-only public market quotes anyway.
	*/
	s.mux.HandleFunc("/api/quotes/stream", s.handleQuoteStream)

	/*
		Auto-execution kill switch. Stack: requireInternal (trusted website
		origin) → executionLimit (per-IP rate cap) → attachUser (load
		session) → requireUser (signed in) → requireEmailAllowlist (single
		allowed email) → handler. All five gates must pass.
	*/
	if s.executor != nil {
		s.mux.HandleFunc("/api/execution/cancel-all",
			requireInternal(executionLimit.middleware(s.attachUser(s.requireUser(s.requireEmailAllowlist(s.executorEmail, s.executor.HandleCancelAll))))))
	}
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

type dashboardTrade struct {
	Trade   trades.Trade         `json:"trade"`
	Summary *trades.TradeSummary `json:"summary,omitempty"`
}

type dashboardResponse struct {
	Date   string           `json:"date"`
	Trades []dashboardTrade `json:"trades"`
	/*
		Executions surfaces every position taken (paper or live) on a
		trade from this date. Empty when no qualifying pick converted to
		an actual execution that day. The basket auto-executor can fire
		multiple per day (top 3 guaranteed phase + greedy fill from
		ranks 1..10), so the frontend matches each entry to its trade
		card by symbol + contract_type + strike to render badges and to
		source realized P&L from broker truth instead of Claudia's
		modeled summary numbers.
	*/
	Executions []*store.ExecutionView `json:"executions,omitempty"`
}

type weekDay struct {
	Date       string                 `json:"date"`
	Trades     []dashboardTrade       `json:"trades"`
	Executions []*store.ExecutionView `json:"executions,omitempty"`
}

type weekResponse struct {
	Start string    `json:"start"`
	End   string    `json:"end"`
	Days  []weekDay `json:"days"`
}

func (s *Server) handleTradeDates(w http.ResponseWriter, r *http.Request) {
	limit := 30
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 365 {
			limit = n
		}
	}
	dates, err := s.db.GetTradeDates(limit)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"dates": []string{}})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, map[string]any{"dates": dates})
}

func (s *Server) handleTradesToday(w http.ResponseWriter, r *http.Request) {
	// Accept optional ?date= query param for historical browsing
	requestDate := r.URL.Query().Get("date")

	var date string
	var err error
	if requestDate != "" {
		date = requestDate
	} else {
		date, err = s.db.GetLatestTradeDate()
		if err != nil {
			/*
				No trade data yet (fresh DB, pre-cron). Return an empty
				trades slice (NEVER nil) so the frontend can safely call
				.filter / .map without a null guard and falls through to
				the EmptyState branch.
			*/
			writeJSON(w, http.StatusOK, dashboardResponse{Trades: []dashboardTrade{}})
			return
		}
	}

	morningTrades, err := s.db.GetMorningTrades(date)
	if err != nil {
		writeJSON(w, http.StatusOK, dashboardResponse{Date: date, Trades: []dashboardTrade{}})
		return
	}

	summaries, _ := s.db.GetEODSummaries(date)
	summaryMap := make(map[string]*trades.TradeSummary)
	for i := range summaries {
		key := summaries[i].Symbol + "|" + summaries[i].ContractType + "|" + fmt.Sprintf("%.2f", summaries[i].StrikePrice)
		summaryMap[key] = &summaries[i]
	}

	result := make([]dashboardTrade, len(morningTrades))
	for i, t := range morningTrades {
		key := t.Symbol + "|" + t.ContractType + "|" + fmt.Sprintf("%.2f", t.StrikePrice)
		result[i] = dashboardTrade{Trade: t, Summary: summaryMap[key]}
	}

	/*
		Optional execution badges for transparency. Errors are non-fatal —
		the dashboard still renders without badges if the lookup fails.
	*/
	execs, _ := s.db.GetExecutionsForDate(date)

	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, dashboardResponse{Date: date, Trades: result, Executions: execs})
}

func (s *Server) handleTradesWeek(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	if start == "" || end == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "start and end query params required"})
		return
	}

	tradesMap, err := s.db.GetTradesForDateRange(start, end)
	if err != nil {
		/*
			Always return an empty array for days (never nil) so the
			frontend can safely call .map without a null guard.
		*/
		writeJSON(w, http.StatusOK, weekResponse{Start: start, End: end, Days: []weekDay{}})
		return
	}

	summariesMap, _ := s.db.GetSummariesForDateRange(start, end)
	executionsMap, _ := s.db.GetExecutionsForDateRange(start, end)

	// Collect all dates that have trades
	dateSet := make(map[string]bool)
	for d := range tradesMap {
		dateSet[d] = true
	}
	var dates []string
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	days := []weekDay{}
	for _, date := range dates {
		dayTrades := tradesMap[date]
		daySummaries := summariesMap[date]

		summaryMap := make(map[string]*trades.TradeSummary)
		for i := range daySummaries {
			key := daySummaries[i].Symbol + "|" + daySummaries[i].ContractType + "|" + fmt.Sprintf("%.2f", daySummaries[i].StrikePrice)
			summaryMap[key] = &daySummaries[i]
		}

		result := make([]dashboardTrade, len(dayTrades))
		for i, t := range dayTrades {
			key := t.Symbol + "|" + t.ContractType + "|" + fmt.Sprintf("%.2f", t.StrikePrice)
			result[i] = dashboardTrade{Trade: t, Summary: summaryMap[key]}
		}

		days = append(days, weekDay{Date: date, Trades: result, Executions: executionsMap[date]})
	}

	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, weekResponse{Start: start, End: end, Days: days})
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
(Schwab, Anthropic, sentiment scrapers) — those live on /health,
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
		endpoint. Severity is conditional on trading mode:
		  - executor nil OR mode=paper: failure is `warn` (trading scope
		    isn't load-bearing yet, just a heads-up that re-auth is needed
		    before flipping to live).
		  - mode=live: failure is `fail` and trips allOK so the deploy
		    healthcheck blocks (because live orders WILL be attempted and
		    they'll bounce without trading scope).
	*/
	tradingHealth := s.checkSchwabTrading(r.Context())
	services["schwab_trading"] = tradingHealth
	if tradingHealth.Status == "fail" {
		allOK = false
	}

	// Market signal sources
	signalCtx, signalCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer signalCancel()
	probeResults := s.scraper.ProbeAll(signalCtx)
	okCount := 0
	sourceNames := make([]string, 0, len(probeResults))
	for _, src := range probeResults {
		if src.OK {
			okCount++
		}
		sourceNames = append(sourceNames, src.Name)
	}
	switch {
	case okCount == len(probeResults):
		services["market_signals"] = serviceHealth{
			Status: "ok",
			Detail: fmt.Sprintf("%d/%d sources healthy (%s)", okCount, len(probeResults), strings.Join(sourceNames, ", ")),
		}
	case okCount > 0:
		var failed []string
		for _, src := range probeResults {
			if !src.OK {
				failed = append(failed, src.Name)
			}
		}
		services["market_signals"] = serviceHealth{
			Status: "warn",
			Detail: fmt.Sprintf("%d/%d sources healthy (down: %s)", okCount, len(probeResults), strings.Join(failed, ", ")),
		}
	default:
		services["market_signals"] = serviceHealth{
			Status: "fail",
			Detail: "All market signal sources unreachable",
		}
		allOK = false
	}

	// Morning picks — surfaces single-model degraded runs (e.g. Claude
	// timed out at 9:35 ET) that the cron logs but nothing else gates on.
	services["morning_picks"] = s.checkMorningPicks()
	if services["morning_picks"].Status == "fail" {
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

/*
checkMorningPicks reports whether today's 9:30 ET trade-picking cron
ran cleanly. Reads today's saved trades and counts how many came from
each picker. Severity:
  - ok:   both pickers produced ≥1 pick today, OR the morning run is
    not yet expected (weekend, or before ~9:45 ET on a weekday)
  - warn: exactly one picker produced 0 picks (single-model run), or
    no trades exist for today after the expected run time (also
    covers market holidays — false-positive warns are tolerable
    since warn doesn't trip allOK)

Reserved as warn rather than fail so a stale morning issue can't block
a fresh deploy from going out — a deploy is often the fix.
*/
func (s *Server) checkMorningPicks() serviceHealth {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return serviceHealth{Status: "warn", Detail: "Could not load ET timezone: " + err.Error()}
	}
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")

	// Weekend: cron doesn't fire, no trades expected.
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return serviceHealth{Status: "ok", Detail: "No morning run scheduled (weekend)"}
	}

	// Cron fires at 9:30 ET; pickers take 1–10 minutes. Give 15 min slack.
	expectedBy := time.Date(now.Year(), now.Month(), now.Day(), 9, 45, 0, 0, loc)
	morningRunExpected := !now.Before(expectedBy)

	tradesToday, err := s.db.GetMorningTrades(today)
	if err != nil {
		return serviceHealth{Status: "warn", Detail: "Could not query today's trades: " + err.Error()}
	}

	if len(tradesToday) == 0 {
		if !morningRunExpected {
			return serviceHealth{Status: "ok", Detail: "Morning run not yet expected (cron at 9:30 ET)"}
		}
		return serviceHealth{Status: "warn", Detail: fmt.Sprintf("No trades saved for %s (cron failure or market holiday)", today)}
	}

	return serviceHealth{Status: "ok", Detail: fmt.Sprintf("Claude produced %d picks today", len(tradesToday))}
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
on the scoped probe, non-200 from Schwab) returns `fail` regardless
of trading mode. Paper vs live is irrelevant for the "is the token
healthy" question, and a green-looking row when the token is dead
hid today's outage from the deploy email. "Not configured" stays
`warn` since that's an env state, not a token state.
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

// ── Chart Data ──

func (s *Server) handleChart(w http.ResponseWriter, r *http.Request) {
	// Extract symbol from /api/chart/{symbol}
	symbol := strings.TrimPrefix(r.URL.Path, "/api/chart/")
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "symbol required"})
		return
	}

	// Default: 5 days of 5-min candles for intraday view
	periodType := r.URL.Query().Get("periodType")
	if periodType == "" {
		periodType = "day"
	}
	period := 5
	if p := r.URL.Query().Get("period"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			period = n
		}
	}
	frequencyType := r.URL.Query().Get("frequencyType")
	if frequencyType == "" {
		frequencyType = "minute"
	}
	frequency := 5
	if f := r.URL.Query().Get("frequency"); f != "" {
		if n, err := strconv.Atoi(f); err == nil && n > 0 {
			frequency = n
		}
	}

	// If Schwab is connected, use real market data.
	if s.schwab != nil && s.schwab.IsConnected() {
		candles, err := s.schwab.GetPriceHistory(symbol, periodType, period, frequencyType, frequency)
		if err != nil {
			log.Printf("Chart data error for %s: %v", symbol, err)
			writeJSON(w, http.StatusBadGateway, apiResponse{OK: false, Message: "failed to fetch chart data"})
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=15")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"symbol":  symbol,
			"candles": candles,
		})
		return
	}

	/*
		Schwab not available — generate synthetic candles from the trade's
		current_price so local dev still renders a chart.
	*/
	candles := s.syntheticCandles(symbol, period, frequency)
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"symbol":  symbol,
		"candles": candles,
	})
}

/*
syntheticCandles generates realistic-looking OHLCV candles for local dev
when Schwab is not connected. It looks up the symbol's current_price from
the trades table to anchor the simulation at the right price level.
*/
func (s *Server) syntheticCandles(symbol string, days, freqMinutes int) []schwab.Candle {
	// Look up a base price from the most recent trade for this symbol.
	basePrice := 150.0 // fallback
	row := s.db.DB().QueryRow(
		`SELECT current_price FROM trades WHERE symbol = $1 ORDER BY date DESC LIMIT 1`,
		symbol,
	)
	if err := row.Scan(&basePrice); err != nil || basePrice <= 0 {
		basePrice = 150.0
	}

	// Deterministic seed from symbol so the chart is stable across refreshes.
	seed := uint64(0)
	for _, c := range symbol {
		seed = seed*31 + uint64(c)
	}
	rng := rand.New(rand.NewPCG(seed, seed^0xdeadbeef))

	// Generate candles: ~78 five-minute bars per trading day (9:30-16:00).
	barsPerDay := 390 / freqMinutes
	totalBars := days * barsPerDay

	now := time.Now()
	// Walk back to find the start date (skip weekends).
	tradingDays := make([]time.Time, 0, days)
	d := now
	for len(tradingDays) < days {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			tradingDays = append(tradingDays, d)
		}
		d = d.AddDate(0, 0, -1)
	}
	// Reverse so oldest is first.
	for i, j := 0, len(tradingDays)-1; i < j; i, j = i+1, j-1 {
		tradingDays[i], tradingDays[j] = tradingDays[j], tradingDays[i]
	}

	candles := make([]schwab.Candle, 0, totalBars)
	price := basePrice * (0.97 + rng.Float64()*0.06) // start near base

	for _, day := range tradingDays {
		marketOpen := time.Date(day.Year(), day.Month(), day.Day(), 9, 30, 0, 0, time.Local)
		for bar := 0; bar < barsPerDay; bar++ {
			t := marketOpen.Add(time.Duration(bar*freqMinutes) * time.Minute)

			// Random walk with mean reversion toward basePrice.
			drift := (basePrice - price) * 0.002
			volatility := basePrice * 0.003
			move := drift + volatility*(rng.Float64()-0.5)*2

			open := price
			close := price + move
			high := math.Max(open, close) + rng.Float64()*volatility*0.5
			low := math.Min(open, close) - rng.Float64()*volatility*0.5
			vol := int64(50000 + rng.IntN(200000))

			// Round to 2 decimals.
			open = math.Round(open*100) / 100
			close = math.Round(close*100) / 100
			high = math.Round(high*100) / 100
			low = math.Round(low*100) / 100

			candles = append(candles, schwab.Candle{
				Time:   t.Unix(),
				Open:   open,
				High:   high,
				Low:    low,
				Close:  close,
				Volume: vol,
			})

			price = close
		}
	}

	return candles
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
		// of every /auth/sso/* and /auth/unsubscribe request that
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
		// of every /auth/sso/* and /auth/unsubscribe request that
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

// Live quotes are streamed via /api/quotes/stream (SSE). The handler
// lives in quotes_stream.go; cache + REST polling have been deleted.

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
