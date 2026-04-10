package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"

	"vibetradez.com/internal/email"
	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/store"
	"vibetradez.com/internal/templates"
	"vibetradez.com/internal/trades"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type Server struct {
	db             *store.Store
	schwab         *schwab.Client
	emailClient    *email.Client
	emailFrom      string
	openaiKey      string
	openaiModel    string
	anthropicKey   string
	anthropicModel string
	adminKey       string
	mux            *http.ServeMux
	port           string
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

func New(db *store.Store, schwabClient *schwab.Client, emailClient *email.Client, emailFrom, openaiKey, openaiModel, anthropicKey, anthropicModel, adminKey, port string) *Server {
	s := &Server{
		db:             db,
		schwab:         schwabClient,
		emailClient:    emailClient,
		emailFrom:      emailFrom,
		openaiKey:      openaiKey,
		openaiModel:    openaiModel,
		anthropicKey:   anthropicKey,
		anthropicModel: anthropicModel,
		adminKey:       adminKey,
		mux:            http.NewServeMux(),
		port:           port,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/auth/schwab", s.handleSchwabAuth)
	s.mux.HandleFunc("/auth/callback", s.handleSchwabCallback)
	s.mux.HandleFunc("/admin/announce", s.handleAnnounce)

	// API routes — require internal header (requests must come from the website)
	s.mux.HandleFunc("/api/subscribe", requireInternal(s.handleSubscribe))
	s.mux.HandleFunc("/api/unsubscribe", requireInternal(s.handleUnsubscribe))
	s.mux.HandleFunc("/api/trades/today", requireInternal(s.handleTradesToday))
	s.mux.HandleFunc("/api/trades/dates", requireInternal(s.handleTradeDates))
	s.mux.HandleFunc("/api/trades/week", requireInternal(s.handleTradesWeek))
	s.mux.HandleFunc("/api/chart/", requireInternal(s.handleChart))
	s.mux.HandleFunc("/api/quotes/live", requireInternal(s.handleLiveQuotes))
	s.mux.HandleFunc("/api/model-comparison", requireInternal(s.handleModelComparison))
}

func (s *Server) Start() {
	addr := ":" + s.port
	log.Printf("API server listening on %s", addr)
	if err := http.ListenAndServe(addr, s.mux); err != nil {
		log.Fatalf("API server error: %v", err)
	}
}

// requireInternal rejects requests to /api/* that don't include the internal header.
// This prevents direct external API access — callers must go through the website.
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
}

type weekDay struct {
	Date   string           `json:"date"`
	Trades []dashboardTrade `json:"trades"`
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
			writeJSON(w, http.StatusOK, dashboardResponse{})
			return
		}
	}

	morningTrades, err := s.db.GetMorningTrades(date)
	if err != nil {
		writeJSON(w, http.StatusOK, dashboardResponse{Date: date})
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

	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, dashboardResponse{Date: date, Trades: result})
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
		// Always return an empty array for days (never nil) so the
		// frontend can safely call .map without a null guard.
		writeJSON(w, http.StatusOK, weekResponse{Start: start, End: end, Days: []weekDay{}})
		return
	}

	summariesMap, _ := s.db.GetSummariesForDateRange(start, end)

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

		days = append(days, weekDay{Date: date, Trades: result})
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

	// Schwab Market Data
	if s.schwab != nil {
		if s.schwab.IsConnected() {
			tokStart := time.Now()
			if _, err := s.schwab.ValidToken(); err != nil {
				services["schwab"] = serviceHealth{Status: "fail", Detail: err.Error(), Latency: fmtLatency(time.Since(tokStart))}
				allOK = false
			} else {
				services["schwab"] = serviceHealth{Status: "ok", Detail: "Authenticated", Latency: fmtLatency(time.Since(tokStart))}
			}
		} else {
			services["schwab"] = serviceHealth{Status: "warn", Detail: "Configured but not authorized"}
		}
	} else {
		services["schwab"] = serviceHealth{Status: "warn", Detail: "Not configured"}
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

// isStubKey returns true for the placeholder keys used by the local Docker
// stack. The local runtime sets ANTHROPIC_API_KEY / OPENAI_API_KEY to a
// stub value so the server boots without making real API calls.
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

	if s.schwab == nil || !s.schwab.IsConnected() {
		writeJSON(w, http.StatusServiceUnavailable, apiResponse{OK: false, Message: "Schwab not connected"})
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ── Admin: Broadcast Announcement ──

type announceRequest struct {
	Subject  string                          `json:"subject"`
	Badge    string                          `json:"badge"`
	Headline string                          `json:"headline"`
	Sections []templates.AnnouncementSection `json:"sections"`
	CTAText  string                          `json:"cta_text"`
	CTAURL   string                          `json:"cta_url"`
}

func (s *Server) handleAnnounce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Message: "method not allowed"})
		return
	}

	// Require admin key
	key := r.Header.Get("X-Admin-Key")
	if s.adminKey == "" || key != s.adminKey {
		writeJSON(w, http.StatusUnauthorized, apiResponse{OK: false, Message: "unauthorized"})
		return
	}

	var req announceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "invalid JSON"})
		return
	}

	if req.Subject == "" || req.Headline == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "subject and headline are required"})
		return
	}

	data := templates.AnnouncementData{
		Subject:  req.Subject,
		Badge:    req.Badge,
		Headline: req.Headline,
		Sections: req.Sections,
		CTAText:  req.CTAText,
		CTAURL:   req.CTAURL,
	}
	if data.Badge == "" {
		data.Badge = "Announcement"
	}

	htmlContent, err := templates.RenderAnnouncementEmail(data)
	if err != nil {
		log.Printf("Error rendering announcement: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Message: "template render failed"})
		return
	}

	recipients, err := s.db.GetActiveEmails()
	if err != nil || len(recipients) == 0 {
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Message: "no active subscribers"})
		return
	}

	if err := s.emailClient.SendTradeEmail(s.emailFrom, recipients, req.Subject, htmlContent); err != nil {
		log.Printf("Error sending announcement: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Message: "email delivery failed"})
		return
	}

	log.Printf("Announcement sent to %d subscribers: %s", len(recipients), req.Subject)
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Message: fmt.Sprintf("sent to %d subscribers", len(recipients))})
}
