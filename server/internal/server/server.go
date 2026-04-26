package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"

	"vibetradez.com/internal/authclient"
	"vibetradez.com/internal/email"
	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/sentiment"
	"vibetradez.com/internal/store"
	"vibetradez.com/internal/trades"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type Server struct {
	db             *store.Store
	schwab         *schwab.Client
	auth           *authclient.Client
	scraper        *sentiment.Scraper
	emailClient    *email.Client
	emailFrom      string
	openaiKey      string
	openaiModel    string
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
}

type subscribeRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type unsubscribeRequest struct {
	Email string `json:"email"`
}

type apiResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func New(db *store.Store, schwabClient *schwab.Client, authClient *authclient.Client, scraper *sentiment.Scraper, emailClient *email.Client, emailFrom, openaiKey, openaiModel, anthropicKey, anthropicModel, sessionCookie string, sessionTTL time.Duration, ssoPublicURL, ssoClientID, ssoRedirectURI, port string, executor *exec.Service, executorEmail string) *Server {
	s := &Server{
		db:             db,
		schwab:         schwabClient,
		auth:           authClient,
		scraper:        scraper,
		emailClient:    emailClient,
		emailFrom:      emailFrom,
		openaiKey:      openaiKey,
		openaiModel:    openaiModel,
		anthropicKey:   anthropicKey,
		anthropicModel: anthropicModel,
		sessionCookie:  sessionCookie,
		sessionTTL:     sessionTTL,
		ssoPublicURL:   strings.TrimRight(ssoPublicURL, "/"),
		ssoClientID:    ssoClientID,
		ssoRedirectURI: ssoRedirectURI,
		mux:            http.NewServeMux(),
		port:           port,
		executor:       executor,
		executorEmail:  executorEmail,
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

	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/auth/schwab", authLimit.middleware(s.handleSchwabAuth))
	s.mux.HandleFunc("/auth/callback", s.handleSchwabCallback)
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
	s.mux.HandleFunc("/api/quotes/live", requireInternal(s.handleLiveQuotes))
	s.mux.HandleFunc("/api/model-comparison", requireInternal(s.handleModelComparison))

	/*
		Auto-execution endpoints. Stack: requireInternal (trusted website
		origin) → executionLimit (per-IP rate cap, defense vs DoS-flood)
		→ attachUser (load session) → requireUser (must be signed in) →
		requireEmailAllowlist (must be the one allowed email) → executor
		handler. All five gates must pass; any single failure returns
		401/403/429 before the handler runs.
	*/
	if s.executor != nil {
		s.mux.HandleFunc("/api/execution/confirm",
			requireInternal(executionLimit.middleware(s.attachUser(s.requireUser(s.requireEmailAllowlist(s.executorEmail, s.executor.HandleConfirm))))))
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
		Execution surfaces a position taken (paper or live) on a trade
		from this date. nil when no qualifying pick converted to an
		actual execution that day. Frontend matches by symbol+
		contract_type+strike to render the badge on the right card.
	*/
	Execution *store.ExecutionView `json:"execution,omitempty"`
}

type weekDay struct {
	Date      string               `json:"date"`
	Trades    []dashboardTrade     `json:"trades"`
	Execution *store.ExecutionView `json:"execution,omitempty"`
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

/*
pickerFilter is the global model filter selected from the nav bar.
'all' returns every union trade ranked by combined_score (default).
'openai' returns only trades where picked_by_openai = true, ranked by
gpt_score desc. 'claude' returns only trades where picked_by_claude
= true, ranked by claude_score desc.
*/
type pickerFilter string

const (
	pickerAll    pickerFilter = "all"
	pickerOpenAI pickerFilter = "openai"
	pickerClaude pickerFilter = "claude"
)

func parsePicker(r *http.Request) pickerFilter {
	switch r.URL.Query().Get("picker") {
	case "openai":
		return pickerOpenAI
	case "claude":
		return pickerClaude
	default:
		return pickerAll
	}
}

/*
applyPickerFilter narrows and re-orders a single day's trades according
to the selected picker. The all view leaves the order untouched (it's
already ranked by combined_score from the cron). The openai / claude
views drop trades the chosen model didn't pick and re-rank by that
model's individual score, then renumber the rank field 1..N so the
frontend can render the same way regardless of which view is active.
*/
func applyPickerFilter(picker pickerFilter, in []trades.Trade) []trades.Trade {
	if picker == pickerAll {
		return in
	}
	out := make([]trades.Trade, 0, len(in))
	for _, t := range in {
		if picker == pickerOpenAI && t.PickedByOpenAI {
			out = append(out, t)
		} else if picker == pickerClaude && t.PickedByClaude {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if picker == pickerOpenAI {
			return out[i].GPTScore > out[j].GPTScore
		}
		return out[i].ClaudeScore > out[j].ClaudeScore
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
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

	morningTrades = applyPickerFilter(parsePicker(r), morningTrades)

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
		Optional execution badge for transparency. Errors are non-fatal —
		the dashboard still renders without the badge if the lookup fails.
	*/
	exec, _ := s.db.GetExecutionForDate(date)

	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, dashboardResponse{Date: date, Trades: result, Execution: exec})
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
	picker := parsePicker(r)

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
		dayTrades := applyPickerFilter(picker, tradesMap[date])
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

		days = append(days, weekDay{Date: date, Trades: result, Execution: executionsMap[date]})
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

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)

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

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "email is required"})
		return
	}

	if err := s.db.RemoveSubscriber(req.Email); err != nil {
		writeJSON(w, http.StatusNotFound, apiResponse{OK: false, Message: err.Error()})
		return
	}

	log.Printf("Unsubscribed: %s", req.Email)
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Message: "unsubscribed successfully"})
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

	// OpenAI (GPT analyzer)
	openaiStart := time.Now()
	openaiHealth := s.checkOpenAI()
	openaiHealth.Latency = fmtLatency(time.Since(openaiStart))
	services["openai"] = openaiHealth
	if openaiHealth.Status == "fail" {
		allOK = false
	}

	// Anthropic (Claude validator)
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
	*/
	if s.schwab != nil {
		if s.schwab.IsConnected() {
			tokStart := time.Now()
			if _, err := s.schwab.ValidToken(); err != nil {
				services["schwab_market_data"] = serviceHealth{Status: "fail", Detail: err.Error(), Latency: fmtLatency(time.Since(tokStart))}
				allOK = false
			} else {
				services["schwab_market_data"] = serviceHealth{Status: "ok", Detail: "Authenticated", Latency: fmtLatency(time.Since(tokStart))}
			}
		} else {
			services["schwab_market_data"] = serviceHealth{Status: "warn", Detail: "Configured but not authorized"}
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
isStubKey returns true for the placeholder keys used by the local Docker
stack. The local runtime sets ANTHROPIC_API_KEY / OPENAI_API_KEY to a
stub value so the server boots without making real API calls.
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

func (s *Server) checkOpenAI() serviceHealth {
	if s.openaiKey == "" {
		return serviceHealth{Status: "fail", Detail: "API key not configured"}
	}
	if isStubKey(s.openaiKey) {
		return serviceHealth{Status: "warn", Detail: "Local stub key — skipping live probe"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := openai.NewClient(openaioption.WithAPIKey(s.openaiKey))
	if _, err := client.Models.List(ctx); err != nil {
		return serviceHealth{Status: "fail", Detail: err.Error()}
	}
	return serviceHealth{Status: "ok", Detail: "OpenAI API reachable"}
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
accountNumbers — the lightest endpoint on the Trader API surface.

Severity is conditional on trading mode (executor.Mode):
  - paper or executor nil: failures are `warn` (trading scope isn't
    load-bearing yet; the warning is just a heads-up that re-auth is
    required before flipping live).
  - live: failures are `fail` so deploys are gated — without trading
    scope the cron WILL try to place orders and they WILL bounce.
*/
func (s *Server) checkSchwabTrading(ctx context.Context) serviceHealth {
	failSeverity := "warn"
	if s.executor != nil && s.executor.Mode() == "live" {
		failSeverity = "fail"
	}

	if s.schwab == nil {
		return serviceHealth{Status: "warn", Detail: "Not configured"}
	}
	if !s.schwab.IsConnected() {
		return serviceHealth{Status: failSeverity, Detail: "Schwab OAuth not authorized — visit /auth/schwab"}
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = probeCtx // reserved for future use; AuthenticatedDo doesn't take ctx today

	start := time.Now()
	resp, err := s.schwab.AuthenticatedDo("GET", "https://api.schwabapi.com/trader/v1/accounts/accountNumbers", nil)
	if err != nil {
		return serviceHealth{Status: failSeverity, Detail: "request failed: " + err.Error(), Latency: fmtLatency(time.Since(start))}
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
			Status:  failSeverity,
			Detail:  fmt.Sprintf("HTTP %d — token lacks trading scope; re-run /auth/schwab", resp.StatusCode),
			Latency: fmtLatency(time.Since(start)),
		}
	default:
		return serviceHealth{
			Status:  failSeverity,
			Detail:  fmt.Sprintf("HTTP %d — Schwab Trader API unreachable", resp.StatusCode),
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

func (s *Server) handleSchwabAuth(w http.ResponseWriter, r *http.Request) {
	if s.schwab == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiResponse{OK: false, Message: "Schwab not configured"})
		return
	}
	http.Redirect(w, r, s.schwab.AuthorizationURL(), http.StatusFound)
}

func (s *Server) handleSchwabCallback(w http.ResponseWriter, r *http.Request) {
	if s.schwab == nil {
		http.Error(w, "Schwab not configured", http.StatusServiceUnavailable)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		log.Printf("Schwab callback: no code param. Full query: %s", r.URL.RawQuery)
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	log.Printf("Schwab callback: received code (%d chars), exchanging for tokens...", len(code))
	if err := s.schwab.ExchangeCode(code); err != nil {
		log.Printf("Schwab OAuth error: %v", err)
		http.Error(w, "OAuth token exchange failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Schwab OAuth: successfully connected")

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// ── Live Quotes ──

type liveQuoteEntry struct {
	LastPrice    float64 `json:"last_price"`
	OpenPrice    float64 `json:"open_price"`
	NetChange    float64 `json:"net_change"`
	NetChangePct float64 `json:"net_change_pct"`
	BidPrice     float64 `json:"bid_price"`
	AskPrice     float64 `json:"ask_price"`
	Volume       int64   `json:"volume"`
}

type liveOptionEntry struct {
	Bid          float64 `json:"bid"`
	Ask          float64 `json:"ask"`
	Last         float64 `json:"last"`
	Mark         float64 `json:"mark"`
	Volume       int     `json:"volume"`
	OpenInterest int     `json:"open_interest"`
	Delta        float64 `json:"delta"`
	Theta        float64 `json:"theta"`
	ImpliedVol   float64 `json:"implied_vol"`
}

type liveQuotesResponse struct {
	Connected  bool                       `json:"connected"`
	MarketOpen bool                       `json:"market_open"`
	AsOf       string                     `json:"as_of"`
	Quotes     map[string]liveQuoteEntry  `json:"quotes"`
	Options    map[string]liveOptionEntry `json:"options"`
}

func isMarketHours() bool {
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Now().In(loc)
	wd := now.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	hour, min := now.Hour(), now.Minute()
	minuteOfDay := hour*60 + min
	return minuteOfDay >= 9*60+30 && minuteOfDay <= 16*60
}

func (s *Server) handleLiveQuotes(w http.ResponseWriter, r *http.Request) {
	resp := liveQuotesResponse{
		AsOf:       time.Now().UTC().Format(time.RFC3339),
		MarketOpen: isMarketHours(),
		Quotes:     make(map[string]liveQuoteEntry),
		Options:    make(map[string]liveOptionEntry),
	}

	if s.schwab == nil || !s.schwab.IsConnected() {
		/*
			Local-dev convenience: when LOCAL_MOCK_QUOTES=1 and Schwab is
			unauthorized, synthesize plausible live marks for today's picks
			so the dashboard's Buy/Current cards exercise the live-data
			path without needing a real Schwab account. Production never
			sets this env var, so the empty-response branch still wins
			there.
		*/
		if os.Getenv("LOCAL_MOCK_QUOTES") == "1" {
			s.fillMockLiveQuotes(&resp)
		}
		w.Header().Set("Cache-Control", "public, max-age=5")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.Connected = true

	// Get today's trades to know which symbols to fetch.
	date, err := s.db.GetLatestTradeDate()
	if err != nil {
		w.Header().Set("Cache-Control", "public, max-age=5")
		writeJSON(w, http.StatusOK, resp)
		return
	}

	morningTrades, err := s.db.GetMorningTrades(date)
	if err != nil || len(morningTrades) == 0 {
		w.Header().Set("Cache-Control", "public, max-age=5")
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Collect unique symbols.
	symbolSet := make(map[string]bool)
	for _, t := range morningTrades {
		symbolSet[t.Symbol] = true
	}
	symbols := make([]string, 0, len(symbolSet))
	for sym := range symbolSet {
		symbols = append(symbols, sym)
	}

	// Fetch stock quotes (cached 15s).
	quotes, err := s.schwab.GetQuotes(symbols)
	if err != nil {
		log.Printf("Schwab quotes error: %v", err)
	} else {
		for sym, q := range quotes {
			resp.Quotes[sym] = liveQuoteEntry{
				LastPrice:    q.LastPrice,
				OpenPrice:    q.OpenPrice,
				NetChange:    q.NetChange,
				NetChangePct: q.NetPercentChange,
				BidPrice:     q.BidPrice,
				AskPrice:     q.AskPrice,
				Volume:       q.TotalVolume,
			}
		}
	}

	// Fetch option chain data for each trade's specific contract (cached 15s).
	for _, t := range morningTrades {
		chain, err := s.schwab.GetOptionChain(t.Symbol, t.ContractType, t.Expiration, t.Expiration, t.StrikePrice)
		if err != nil {
			continue
		}
		contract := schwab.FindContract(chain, t.ContractType, t.StrikePrice, t.Expiration)
		if contract == nil {
			continue
		}
		key := fmt.Sprintf("%s|%s|%.2f|%s", t.Symbol, t.ContractType, t.StrikePrice, t.Expiration)
		resp.Options[key] = liveOptionEntry{
			Bid:          contract.Bid,
			Ask:          contract.Ask,
			Last:         contract.Last,
			Mark:         contract.Mark,
			Volume:       contract.TotalVolume,
			OpenInterest: contract.OpenInterest,
			Delta:        contract.Delta,
			Theta:        contract.Theta,
			ImpliedVol:   contract.Volatility,
		}
	}

	w.Header().Set("Cache-Control", "public, max-age=10")
	writeJSON(w, http.StatusOK, resp)
}

/*
fillMockLiveQuotes synthesizes plausible live marks for today's morning
picks so the dashboard's Buy/Current cards have data to render in local
dev (Schwab OAuth not set up). Each ticker gets a stable per-trade drift
derived from its symbol so refreshes don't flicker wildly, plus a small
time-based jitter so the numbers visibly tick. Never invoked in
production: gated behind the LOCAL_MOCK_QUOTES env var.
*/
func (s *Server) fillMockLiveQuotes(resp *liveQuotesResponse) {
	date, err := s.db.GetLatestTradeDate()
	if err != nil {
		return
	}
	morningTrades, err := s.db.GetMorningTrades(date)
	if err != nil || len(morningTrades) == 0 {
		return
	}
	resp.Connected = true

	// Slow time-based oscillator so the price visibly drifts every refresh.
	tick := math.Sin(float64(time.Now().Unix()%600) / 600.0 * 2 * math.Pi)

	for _, t := range morningTrades {
		/*
			Stable drift in [-0.35, +0.35] derived from the symbol so each
			ticker has its own personality across refreshes.
		*/
		var hash uint64
		for _, c := range t.Symbol {
			hash = hash*31 + uint64(c)
		}
		drift := (float64(hash%1000)/1000.0)*0.7 - 0.35
		// Stock price: nudge ~1% off entry, plus tick.
		stockMove := t.CurrentPrice * (drift*0.01 + tick*0.005)
		stockNow := t.CurrentPrice + stockMove
		resp.Quotes[t.Symbol] = liveQuoteEntry{
			LastPrice:    stockNow,
			OpenPrice:    t.CurrentPrice,
			NetChange:    stockMove,
			NetChangePct: (stockMove / t.CurrentPrice) * 100,
			BidPrice:     stockNow - 0.02,
			AskPrice:     stockNow + 0.02,
			Volume:       1_000_000 + int64(hash%500_000),
		}
		/*
			Option mark: scale the move by a fake delta of ~0.5 for ATM,
			plus its own jitter so winners and losers diverge visibly.
		*/
		optMove := stockMove*0.5 + t.EstimatedPrice*tick*0.04 + t.EstimatedPrice*drift*0.1
		mark := math.Max(0.01, t.EstimatedPrice+optMove)
		key := fmt.Sprintf("%s|%s|%.2f|%s", t.Symbol, t.ContractType, t.StrikePrice, t.Expiration)
		resp.Options[key] = liveOptionEntry{
			Bid:          math.Max(0.01, mark-0.05),
			Ask:          mark + 0.05,
			Last:         mark,
			Mark:         mark,
			Volume:       int(500 + hash%2000),
			OpenInterest: int(2000 + hash%5000),
			Delta:        0.5 + drift*0.2,
			Theta:        -0.05 - math.Abs(drift)*0.05,
			ImpliedVol:   0.35 + math.Abs(drift)*0.2,
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
