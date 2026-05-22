package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"vibetradez.com/internal/auth"
	"vibetradez.com/internal/calendar"
	"vibetradez.com/internal/config"
	"vibetradez.com/internal/email"
	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/execagent"
	"vibetradez.com/internal/quotes"
	"vibetradez.com/internal/rollouts"
	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/sentiment"
	"vibetradez.com/internal/server"
	"vibetradez.com/internal/store"
	"vibetradez.com/internal/templates"
	"vibetradez.com/internal/trades"
	"vibetradez.com/internal/unsub"

	"github.com/robfig/cron/v3"
)

// Market calendar moved to internal/calendar. The streaming hub + SSE
// handlers consume the same source. Update the lists there yearly each
// Q4 from NYSE's published schedule.

func isHalfDay() bool {
	today := time.Now().In(calendar.ETLocation).Format("2006-01-02")
	return calendar.IsHalfDay(today) != ""
}

func isMarketOpen() (bool, string) {
	now := time.Now().In(calendar.ETLocation)
	today := now.Format("2006-01-02")

	if holiday := calendar.IsHoliday(today); holiday != "" {
		return false, holiday
	}

	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false, "Weekend"
	}

	return true, ""
}

func todayDate() string {
	loc, _ := time.LoadLocation("America/New_York")
	return time.Now().In(loc).Format("2006-01-02")
}

/*
checkClockSkew probes Cloudflare's HTTP Date header (NTP-disciplined
within the millisecond) and compares against the local clock. Logs a
warning if drift exceeds 5 seconds. Run from a goroutine on startup so
a slow probe doesn't delay boot. Failures (network, parse) are silent.
*/
func checkClockSkew() {
	const maxAcceptableSkew = 5 * time.Second
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("HEAD", "https://1.1.1.1", nil)
	if err != nil {
		return
	}
	beforeReq := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("clock-skew probe: HEAD failed: %v (skipping)", err)
		return
	}
	rtt := time.Since(beforeReq)
	defer func() { _ = resp.Body.Close() }()

	dateHeader := resp.Header.Get("Date")
	if dateHeader == "" {
		log.Printf("clock-skew probe: no Date header in response (skipping)")
		return
	}
	remote, err := http.ParseTime(dateHeader)
	if err != nil {
		log.Printf("clock-skew probe: parse Date %q: %v (skipping)", dateHeader, err)
		return
	}
	estimatedRemoteAtReceive := remote.Add(rtt / 2)
	skew := time.Since(estimatedRemoteAtReceive)
	if skew < 0 {
		skew = -skew
	}
	if skew > maxAcceptableSkew {
		log.Printf("clock-skew WARNING: local clock differs from cloudflare by %s (threshold %s); the 9:25 picker, 9:30 executor, 9:35 dangling-LIMIT-cancel, and 3:55 close crons WILL fire at the wrong wall-clock time", skew.Truncate(time.Second), maxAcceptableSkew)
	} else {
		log.Printf("clock-skew probe: local clock within %s of cloudflare (rtt=%s)", skew.Truncate(time.Millisecond), rtt.Truncate(time.Millisecond))
	}
}

/*
isLocalStubKey detects the placeholder API keys used by the local Docker
stack so the cron can be safely skipped without making real API calls.
*/
/*
unsubURLBuilder returns the per-recipient unsubscribe URL builder
used by every subscriber-facing email send. Tokens are deterministic
(same email + same key always produces the same URL), so links from
old emails stay valid forever — exactly the property a one-click
unsubscribe needs.
*/
func unsubURLBuilder(cfg *config.Config) func(string) string {
	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	return func(email string) string {
		tok := unsub.Sign(cfg.UnsubscribeHMACKey, email)
		return fmt.Sprintf("%s/auth/unsubscribe?e=%s&t=%s",
			base, url.QueryEscape(email), url.QueryEscape(tok))
	}
}

func isLocalStubKey(k string) bool {
	if k == "" {
		return false
	}
	switch {
	case len(k) >= 5 && k[:5] == "stub-":
		return true
	case len(k) >= 8 && k[:8] == "sk_local":
		return true
	case len(k) >= 8 && k[:8] == "sk-local":
		return true
	}
	return false
}

func main() {
	cfg := config.Load()

	db, err := store.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if len(cfg.EmailRecipients) > 0 {
		for _, email := range cfg.EmailRecipients {
			// EnsureSubscriberExists preserves prior unsubscribes;
			// AddSubscriber would silently re-subscribe opted-out
			// addresses on every container restart.
			if err := db.EnsureSubscriberExists(email, ""); err != nil {
				log.Printf("Warning: failed to seed subscriber %s: %v", email, err)
			}
		}
		log.Printf("Seeded %d subscribers from EMAIL_RECIPIENTS", len(cfg.EmailRecipients))
	}

	var schwabClient *schwab.Client
	if cfg.SchwabAppKey != "" && cfg.SchwabSecret != "" {
		schwabClient = schwab.NewClient(cfg.SchwabAppKey, cfg.SchwabSecret, cfg.SchwabCallbackURL, cfg.SchwabTokenKey, db)
		if schwabClient.IsConnected() {
			log.Println("Schwab: connected (tokens loaded)")
		} else {
			log.Printf("Schwab: configured but not authorized, visit https://vibetradez.com/auth/schwab to connect")
		}
	} else {
		log.Println("Schwab: not configured (SCHWAB_APP_KEY / SCHWAB_SECRET not set)")
	}

	// Open the dedicated AUTH_DATABASE_URL pool (users + sessions),
	// wire it to the Google OAuth client, and hand the resulting
	// *auth.Service to the HTTP server.
	authStore, err := auth.NewStore(cfg.AuthDatabaseURL)
	if err != nil {
		log.Fatalf("auth store: %v", err)
	}
	defer func() { _ = authStore.Close() }()
	googleClient := auth.NewGoogleClient(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleCallbackURL)
	authService := auth.New(authStore, googleClient, auth.Options{
		SessionTTL: time.Duration(cfg.SessionTTLDays) * 24 * time.Hour,
	})

	scraper := sentiment.NewScraper()

	log.Println("Probing market signal sources...")
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 30*time.Second)
	for _, src := range scraper.ProbeAll(probeCtx) {
		if src.OK {
			log.Printf("  %s: OK (%d tickers, %s)", src.Name, src.Tickers, src.Latency.Truncate(time.Millisecond))
		} else {
			log.Printf("  %s: FAIL (%s, %s)", src.Name, src.Err, src.Latency.Truncate(time.Millisecond))
		}
	}
	probeCancel()

	emailClient := email.NewClient(cfg.ResendAPIKey)
	modelLabel := config.CurrentModelLabel

	/*
		Claude picker is the sole picker. When ANTHROPIC_API_KEY is the
		local stub the picker is left nil and cron jobs degrade to a no-op
		so the local Docker stack boots without making real API calls.
	*/
	var claudePicker *trades.ClaudePicker
	if isLocalStubKey(cfg.AnthropicAPIKey) {
		log.Printf("%s: local stub key detected, picking disabled", modelLabel)
	} else {
		claudePicker = trades.NewClaudePicker(cfg.AnthropicAPIKey, cfg.AnthropicModel, schwabClient)
		log.Printf("%s: configured (model=%s)", modelLabel, cfg.AnthropicModel)
	}

	/*
		Auto-execution wiring. Constructed only if TRADING_ENABLED. The
		trader implementation is paper unless TRADING_MODE is literally
		"live", see config.resolveTradingMode for the safety semantics.
	*/
	var executor *exec.Service
	if cfg.TradingEnabled {
		var trader exec.TraderClient
		if cfg.TradingMode == "live" {
			trader = exec.NewLiveTrader(schwabClient)
			log.Printf("execution: LIVE mode armed, real-money orders will be placed on confirmation")
		} else {
			trader = exec.NewPaperTrader(schwabClient)
			log.Printf("execution: PAPER mode, Schwab Trader API will NOT be called")
		}
		execCfg := exec.ServiceConfig{
			Mode:              cfg.TradingMode,
			Recipient:         cfg.ExecutionRecipient,
			EmailFrom:         cfg.EmailFrom,
			ModelLabel:        modelLabel,
			SchwabAccountHash: trader.AccountHash,
		}
		executor = exec.NewService(db, trader, emailClient, execCfg)
	}

	/*
		At-open execution agent. The 9:30 cron re-invokes Claude with
		the morning candidates + live tools (Schwab chain, equity quotes,
		web search) and a place_options_order tool that wraps
		executor.PlaceBuyToOpenAgent. Replaces the old deterministic
		resolver+selector path entirely.

		The agent is only constructed when both a picker model is
		available (otherwise the morning cron writes nothing for it to
		execute against) AND the executor service is enabled (otherwise
		PlaceBuyToOpenAgent has no broker to talk to). Local stub keys
		short-circuit both branches at the picker side.
	*/
	var execAgent *execagent.Agent
	if cfg.TradingEnabled && claudePicker != nil && executor != nil {
		execAgent = execagent.NewAgent(cfg.AnthropicAPIKey, cfg.AnthropicModel, schwabClient, executor)
		log.Printf("execagent: ready (model=%s)", cfg.AnthropicModel)
	}

	// 9:25 ET — ticker selection only. Claude picks symbols + direction +
	// contract intent (target_otm_pct, min_dte) against live Schwab equity
	// quotes; option-chain prices are intentionally NOT consumed at this
	// stage (US options don't trade premarket, so the chain still shows
	// yesterday's 4:00 PM close). The 5-minute pre-bell window lets the
	// tool-use loop finish before the 9:30 executor cron snaps each pick
	// to a real contract against the live post-open chain.
	openJob := func() {
		if open, reason := isMarketOpen(); !open {
			log.Printf("Skipping morning picker: Market closed (%s)", reason)
			return
		}
		runTradeAnalysis(cfg, db, scraper, claudePicker, schwabClient, emailClient, modelLabel, executor)
	}

	// 9:30:00 ET sharp — execute the basket at the open. Loads today's
	// saved picks, re-quotes via live Schwab ask, fires PlaceOrder for
	// each qualifying entry. Does NOT poll/wait/email — the per-minute
	// reconcile cron picks up fills, the 9:35 cron cancels dangling
	// LIMITs and sends one consolidated summary email.
	executeAtOpenJob := func() {
		if open, reason := isMarketOpen(); !open {
			log.Printf("Skipping execute-at-open: Market closed (%s)", reason)
			return
		}
		if execAgent == nil {
			log.Printf("Skipping execute-at-open: agent not configured (trading disabled or no picker)")
			return
		}
		runExecuteAtOpen(cfg, db, emailClient, execAgent)
	}

	// 9:35 ET — cancel any still-WORKING LIMITs from the 9:30 burst,
	// then send the single consolidated execution-summary email.
	cancelAndSummaryJob := func() {
		if open, reason := isMarketOpen(); !open {
			log.Printf("Skipping cancel-dangling-and-summary: Market closed (%s)", reason)
			return
		}
		if executor == nil {
			return
		}
		ctxBg := context.Background()
		executor.CancelDanglingOpens(ctxBg, todayDate())
		executor.SendExecutionSummary(ctxBg, todayDate())
	}

	closeJob := func() {
		if open, reason := isMarketOpen(); !open {
			log.Printf("Skipping EOD summary: Market closed (%s)", reason)
			return
		}
		runEndOfDayAnalysis(cfg, db, claudePicker, emailClient)
	}

	weeklyJob := func() {
		runWeeklyEmail(cfg, db, emailClient)
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("Failed to load timezone: %v", err)
	}

	c := cron.New(cron.WithLocation(loc))

	if _, err := c.AddFunc(cfg.CronScheduleOpen, openJob); err != nil {
		log.Fatalf("Failed to add picker cron job: %v", err)
	}
	if _, err := c.AddFunc(cfg.CronScheduleExecute, executeAtOpenJob); err != nil {
		log.Fatalf("Failed to add execute-at-open cron job: %v", err)
	}
	if _, err := c.AddFunc(cfg.CronScheduleCancelDangling, cancelAndSummaryJob); err != nil {
		log.Fatalf("Failed to add cancel-dangling-and-summary cron job: %v", err)
	}
	if _, err := c.AddFunc(cfg.CronScheduleClose, closeJob); err != nil {
		log.Fatalf("Failed to add market close cron job: %v", err)
	}
	if _, err := c.AddFunc(cfg.CronScheduleWeekly, weeklyJob); err != nil {
		log.Fatalf("Failed to add weekly email cron job: %v", err)
	}

	/*
		Daily Schwab refresh-token expiry warning. Schwab caps refresh
		tokens at 7 days from the original consent and does NOT rotate
		them on access-token refresh, so without a proactive nag the
		operator only discovers the dead token when the 9:30 ET cron
		hard-fails in front of subscribers. Fires at 12:00 ET (mid-day,
		when the operator is most likely at a desk to act on it), warns
		starting at 5-days-old, dedupes per ET date so a same-day
		restart doesn't double-send.
	*/
	if _, err := c.AddFunc("0 12 * * *", func() {
		checkSchwabReauth(cfg, db, emailClient)
	}); err != nil {
		log.Fatalf("Failed to add Schwab re-auth nag cron: %v", err)
	}

	if executor != nil {
		ctxBg := context.Background()

		/*
			Per-minute open-fill reconciliation. The morning cron's single
			post-PlaceOrder GetOrder call almost always sees WORKING (Schwab
			doesn't fill pre-market LIMITs instantly), so without this loop
			a real working order never flips to filled in our DB — meaning
			the dashboard hides it and the 3:55 close cron skips it. Window
			is 9:00-15:59 ET, M-F: covers normal-day opens through 3:55
			close, half-day opens through 12:55 close, and any late catch-up
			between fills and close. Cheap when there's nothing working.
		*/
		if _, err := c.AddFunc("* 9-15 * * 1-5", func() {
			if open, reason := isMarketOpen(); !open {
				log.Printf("Skipping open-order reconcile: %s", reason)
				return
			}
			executor.ReconcileOpenOrders(ctxBg, todayDate())
		}); err != nil {
			log.Fatalf("Failed to add open-order reconcile cron: %v", err)
		}

		if _, err := c.AddFunc("55 15 * * 1-5", func() {
			if open, reason := isMarketOpen(); !open {
				log.Printf("Skipping 3:55pm close: %s", reason)
				return
			}
			if isHalfDay() {
				log.Printf("Skipping 3:55pm close: half-day (12:55 close already fired)")
				return
			}
			executor.CloseAllPositionsForDate(ctxBg, todayDate())
			executor.SendCloseSummary(ctxBg, todayDate())
		}); err != nil {
			log.Fatalf("Failed to add 3:55pm close cron: %v", err)
		}

		if _, err := c.AddFunc("55 12 * * 1-5", func() {
			if open, reason := isMarketOpen(); !open {
				log.Printf("Skipping half-day close: %s", reason)
				return
			}
			if !isHalfDay() {
				return
			}
			executor.CloseAllPositionsForDate(ctxBg, todayDate())
			executor.SendCloseSummary(ctxBg, todayDate())
		}); err != nil {
			log.Fatalf("Failed to add 12:55pm half-day close cron: %v", err)
		}
		log.Printf("execution: cron registered (picker 9:25 ET, executor 9:30 ET, dangling-LIMIT-cancel 9:35 ET, open-reconcile every minute 9-15 ET, close 3:55pm or 12:55pm half-days)")
	}

	/*
		Live-quotes streaming hub. One Schwab WebSocket connection
		multiplexed to N browser SSE clients. Cron lifecycle:
		  - Start: 9:30 ET sharp on trading days (picker has finished
		    by then, so today's picks are in DB).
		  - Stop: 16:00 ET (or 13:00 ET on half-days).
		  - Holidays + weekends: never starts.
		When the hub isn't running, /api/quotes/stream returns 503 and
		the dashboard renders the closed-page.
	*/
	var streamClient *schwab.StreamClient
	if schwabClient != nil && schwabClient.IsConnected() {
		streamClient = schwab.NewStreamClient(schwabClient)
	}
	quotesHub := quotes.NewHub(streamClient, db)
	if _, err := c.AddFunc("30 9 * * 1-5", func() {
		today := todayDate()
		if !calendar.IsTradingDay(today) {
			log.Printf("quotes hub: skip start (%s is not a trading day)", today)
			return
		}
		if err := quotesHub.Start(context.Background()); err != nil {
			log.Printf("quotes hub: start: %v", err)
		}
	}); err != nil {
		log.Fatalf("Failed to add 9:30 quotes-hub start cron: %v", err)
	}
	if _, err := c.AddFunc("0 16 * * 1-5", func() {
		today := todayDate()
		// On half-days the 13:00 ET cron already stopped it; this is a
		// no-op then (Stop is idempotent).
		if calendar.IsHalfDay(today) != "" {
			return
		}
		quotesHub.Stop()
	}); err != nil {
		log.Fatalf("Failed to add 16:00 quotes-hub stop cron: %v", err)
	}
	if _, err := c.AddFunc("0 13 * * 1-5", func() {
		today := todayDate()
		if calendar.IsHalfDay(today) == "" {
			return
		}
		quotesHub.Stop()
	}); err != nil {
		log.Fatalf("Failed to add 13:00 quotes-hub half-day stop cron: %v", err)
	}
	log.Printf("quotes hub: cron registered (start 9:30 ET, stop 16:00 ET regular / 13:00 ET half-days)")

	c.Start()

	sessionTTL := time.Duration(cfg.SessionTTLDays) * 24 * time.Hour
	srv := server.New(db, schwabClient, authService, scraper, emailClient, cfg.EmailFrom, cfg.AnthropicAPIKey, cfg.AnthropicModel, cfg.SessionCookieName, sessionTTL, cfg.ServerPort, executor, cfg.UnsubscribeHMACKey, cfg.UnsubscribePrevHMACKeys, cfg.PublicBaseURL)
	srv.SetHub(quotesHub)
	go srv.Start()

	// If the process boots inside the 9:30-16:00 window on a trading
	// day (e.g. container restart mid-session), warm the hub up
	// immediately so the dashboard doesn't sit on a 503 until the next
	// 9:30 cron tick the following day.
	if st := calendar.CurrentStatus(time.Now()); st.Open {
		if err := quotesHub.Start(context.Background()); err != nil {
			log.Printf("quotes hub: warm start: %v", err)
		}
	}

	log.Printf("Options trade scanner started")
	log.Printf("Database: PostgreSQL")
	log.Printf("API server: :%s", cfg.ServerPort)
	log.Printf("Market open schedule: %s (ET)", cfg.CronScheduleOpen)
	log.Printf("Market close schedule: %s (ET)", cfg.CronScheduleClose)
	log.Printf("Weekly email schedule: %s (ET)", cfg.CronScheduleWeekly)

	go checkClockSkew()

	if subs, err := db.GetActiveSubscribers(); err == nil {
		log.Printf("Active subscribers: %d", len(subs))
	}

	go rollouts.Run(db, emailClient, cfg.EmailFrom, isLocalStubKey(cfg.ResendAPIKey), unsubURLBuilder(cfg))

	if os.Getenv("RUN_ON_START") == "true" {
		log.Println("Running initial analysis...")
		openJob()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	c.Stop()
}

/*
checkSchwabReauth emails the operator when the Schwab refresh token
is within 2 days of Schwab's hard 7-day cap. ExecutionRecipient only;
dedupes per ET calendar date via the reauth_nag_sent_on column so the
operator gets at most one nag per day no matter how the cron fires
or how often the container restarts. Skips silently when no issuance
timestamp is on file (legacy row pre-migration, or no token at all).
*/
func checkSchwabReauth(cfg *config.Config, db *store.Store, emailClient *email.Client) {
	const (
		schwabRefreshTokenLifetime = 7 * 24 * time.Hour
		warnThreshold              = 5 * 24 * time.Hour
	)

	if isLocalStubKey(cfg.ResendAPIKey) {
		return
	}
	if cfg.ExecutionRecipient == "" {
		return
	}

	issuedAt, lastNag, ok, err := db.GetRefreshTokenIssuedAt("schwab")
	if err != nil {
		log.Printf("schwab-reauth: failed to read issuance: %v", err)
		return
	}
	if !ok {
		return
	}

	age := time.Since(issuedAt)
	if age < warnThreshold {
		return
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("schwab-reauth: failed to load ET timezone: %v", err)
		return
	}
	todayET := time.Now().In(loc)
	if !lastNag.IsZero() && sameDate(lastNag.In(loc), todayET) {
		return
	}

	daysOld := int(age.Hours() / 24)
	remaining := schwabRefreshTokenLifetime - age
	daysRemaining := int(remaining.Hours() / 24)
	if remaining < 0 {
		daysRemaining = 0
	}

	expiresAt := issuedAt.Add(schwabRefreshTokenLifetime).In(loc)
	html, err := templates.RenderSchwabReauth(templates.SchwabReauthData{
		Subject:       "Schwab re-authorization needed",
		Date:          todayET.Format("Monday, Jan 2, 2006 3:04 PM"),
		IssuedAt:      issuedAt.In(loc).Format("Mon, Jan 2, 2006 3:04 PM MST"),
		ExpiresAt:     expiresAt.Format("Mon, Jan 2, 2006 3:04 PM MST"),
		DaysOld:       daysOld,
		DaysRemaining: daysRemaining,
		ReauthURL:     "https://vibetradez.com/auth/schwab",
	})
	if err != nil {
		log.Printf("schwab-reauth: render failed: %v", err)
		return
	}

	subject := fmt.Sprintf("VibeTradez: Schwab re-auth needed (%d day(s) remaining)", daysRemaining)
	if err := emailClient.SendTradeEmail(cfg.EmailFrom, []string{cfg.ExecutionRecipient}, subject, html); err != nil {
		log.Printf("schwab-reauth: send failed: %v", err)
		return
	}
	if err := db.MarkReauthNagSent("schwab", todayET); err != nil {
		log.Printf("schwab-reauth: failed to record nag sent: %v", err)
	}
	log.Printf("schwab-reauth: nag sent to %s (age=%dd remaining=%dd)", cfg.ExecutionRecipient, daysOld, daysRemaining)
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

/*
validatePickFields drops picks with malformed required fields. Claude
sometimes returns picks where required numeric fields decoded as zero
(missing JSON key, or a literal 0 from a hallucinated response), and
those rows silently propagate to the DB, the morning email, and the
auto-executor's quantity/budget math. Reject anything where:
  - Symbol is empty
  - ContractType is not CALL or PUT
  - StrikePrice, EstimatedPrice, or CurrentPrice is non-positive
  - DTE is negative
  - Expiration is not a 10-character YYYY-MM-DD string
*/
func validatePickFields(picks []trades.Trade) []trades.Trade {
	keep := make([]trades.Trade, 0, len(picks))
	for _, p := range picks {
		reasons := []string{}
		if p.Symbol == "" {
			reasons = append(reasons, "empty symbol")
		}
		if p.ContractType != "CALL" && p.ContractType != "PUT" {
			reasons = append(reasons, fmt.Sprintf("contract_type=%q", p.ContractType))
		}
		if p.CurrentPrice <= 0 {
			reasons = append(reasons, fmt.Sprintf("current_price=%v", p.CurrentPrice))
		}
		// Intent fields: TargetOTMPct can legitimately be 0 (ATM); we only
		// reject the negative case as malformed. MinDTE must be non-negative
		// (0DTE is fine).
		if p.TargetOTMPct < 0 {
			reasons = append(reasons, fmt.Sprintf("target_otm_pct=%.2f (negative)", p.TargetOTMPct))
		}
		if p.MinDTE < 0 {
			reasons = append(reasons, fmt.Sprintf("min_dte=%d (negative)", p.MinDTE))
		}
		// Contract specifics (strike/expiration/estimated_price/dte/stop_loss)
		// are intentionally NOT validated here: they're nil at picker time
		// and only filled by the 9:30:00 ET at-open contract-resolution path.
		if len(reasons) > 0 {
			log.Printf("pick validation: DROPPING #%d %s (%s)", p.Rank, p.Symbol, strings.Join(reasons, ", "))
			continue
		}
		keep = append(keep, p)
	}
	return keep
}

/*
renumberRanks restores 1..N contiguous rank ordering after picks are
dropped by validation. Without this the basket selector and the
morning email's "#3 of 10" labels both surface gaps that confuse the
reader. Stable in the order picks arrive: callers rely on the picker
already emitting picks sorted by Claude's intended rank.
*/
func renumberRanks(picks []trades.Trade) []trades.Trade {
	for i := range picks {
		picks[i].Rank = i + 1
	}
	return picks
}

/*
validatePicksAgainstSpot reconciles each pick's reported current_price
against a fresh Schwab quote. If the picker's current_price drifts > 25%
from the live spot at validation time (typical cause: Claude pulled spot
from web search or a stale tool result), we replace current_price in
place with the Schwab number so the 9:30 at-open agent has the correct
underlying when it reads target_otm_pct off the candidate.

We do NOT drop picks here anymore: the at-open agent reads the live
chain and picks the strike directly, so the worst-case downstream
effect of a slightly stale current_price is the agent anchoring its
strike search a percent or two off the picker's intended OTM band.
That's tolerable.

Uses GetQuotesUncached so the guard is an independent probe, not a read
of the same 45s-cached batch the picker tools warmed. A nil schwab
client (local dev) is a no-op pass-through, as is a GetQuotes failure.
*/
func validatePicksAgainstSpot(picks []trades.Trade, schwabClient *schwab.Client) []trades.Trade {
	const maxDrift = 0.25

	if schwabClient == nil || len(picks) == 0 {
		return picks
	}
	seen := make(map[string]bool, len(picks))
	symbols := make([]string, 0, len(picks))
	for _, p := range picks {
		if !seen[p.Symbol] {
			seen[p.Symbol] = true
			symbols = append(symbols, p.Symbol)
		}
	}
	quotes, err := schwabClient.GetQuotesUncached(symbols)
	if err != nil {
		log.Printf("pick validation: GetQuotesUncached failed, passing picks through unchecked: %v", err)
		return picks
	}

	for i := range picks {
		p := &picks[i]
		q, ok := quotes[p.Symbol]
		if !ok || q.LastPrice <= 0 {
			continue
		}
		distance := (p.CurrentPrice - q.LastPrice) / q.LastPrice
		if distance < 0 {
			distance = -distance
		}
		if distance > maxDrift {
			log.Printf("pick validation: rank=%d %s current_price=$%.2f drifted %.0f%% from spot=$%.2f, replacing with Schwab spot",
				p.Rank, p.Symbol, p.CurrentPrice, distance*100, q.LastPrice)
			p.CurrentPrice = q.LastPrice
		}
	}
	return picks
}

func sendErrorNotification(cfg *config.Config, db *store.Store, emailClient *email.Client, errMsg string) {
	htmlContent, err := templates.RenderErrorEmail(errMsg)
	if err != nil {
		log.Printf("Failed to render error email (giving up): %v", err)
		return
	}

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		log.Println("No active subscribers for error notification")
		return
	}

	subject := fmt.Sprintf("VibeTradez Alert (%s)", time.Now().Format("Jan 2, 3:04 PM"))
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, htmlContent, unsubURLBuilder(cfg))
	if res.Failed > 0 {
		log.Printf("Error notification email: %d/%d failed (%s)", res.Failed, res.Total, res.FailureDetail())
	}
}

func getRecipients(db *store.Store) []string {
	emails, err := db.GetActiveEmails()
	if err != nil {
		log.Printf("Error getting subscribers: %v", err)
		return nil
	}
	return emails
}

func runTradeAnalysis(cfg *config.Config, db *store.Store, scraper *sentiment.Scraper, claudePicker *trades.ClaudePicker, schwabClient *schwab.Client, emailClient *email.Client, modelLabel string, executor *exec.Service) {
	if claudePicker == nil {
		log.Println("Skipping trade analysis: Claude picker not configured (local stub or missing key)")
		return
	}

	/*
		Wall-clock budget for the entire morning analysis: sentiment scrape +
		Claude's full multi-round conversation + DB write + email render +
		auto-execution submit. Set to 60 minutes so Claude has room to walk
		the option chains for many tickers without ever hitting a deadline,
		while still bounding worst-case if the API is wedged.
	*/
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	log.Println("Starting trade analysis...")

	log.Println("Scraping market signals...")
	sentimentData, err := scraper.GetTrendingTickers(ctx, 20)
	if err != nil {
		log.Printf("Warning: error getting sentiment data: %v", err)
		sentimentData = nil
	}
	log.Printf("Found %d trending tickers", len(sentimentData))

	log.Printf("Analyzing trades with %s...", modelLabel)
	topTrades, err := claudePicker.GetTopTrades(ctx, sentimentData)
	if err != nil {
		log.Printf("Trade picker failed: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Trade picker failed: %v", err))
		return
	}
	if len(topTrades) == 0 {
		log.Println("No trades generated, skipping email")
		return
	}
	log.Printf("%s produced %d picks", modelLabel, len(topTrades))

	originalCount := len(topTrades)
	topTrades = validatePickFields(topTrades)
	topTrades = validatePicksAgainstSpot(topTrades, schwabClient)
	topTrades = renumberRanks(topTrades)
	if dropped := originalCount - len(topTrades); dropped > 0 {
		log.Printf("pick validation: dropped %d/%d picks (field validation + strike-spot)", dropped, originalCount)
		if dropped*2 > originalCount {
			sendErrorNotification(cfg, db, emailClient, fmt.Sprintf(
				"Pick validation dropped %d of %d picks (>50%%). Likely picker saw stale quotes or returned malformed JSON. Check Claude tool logs for the morning run.",
				dropped, originalCount))
		}
	}
	if len(topTrades) == 0 {
		log.Println("All picks dropped by validation, skipping email")
		return
	}

	date := todayDate()
	if err := db.SaveMorningTrades(date, topTrades); err != nil {
		if errors.Is(err, store.ErrTradesAlreadyExecuted) {
			// The 9:30 cron fired twice (e.g. RUN_ON_START left on, container
			// restart at 9:29:55 ET) and the first run's picks already have
			// execution rows. Rewriting trade ids would leave the
			// executions table pointing at deleted rows. Bail entirely — the
			// first run's email is already out and the basket is in flight.
			log.Printf("Morning cron re-fired for %s; first-run executions exist — skipping save + email + executor", date)
			return
		}
		log.Printf("Error saving trades to database: %v", err)
		return
	}
	log.Printf("Saved %d trades to database for %s", len(topTrades), date)

	/*
		The picker no longer fires the executor inline. Picks are saved
		here; the 9:30:00 ET executeAtOpenJob (runExecuteAtOpen) loads
		them and submits the basket against fresh live Schwab quotes.
		Decoupling buys the 5-minute pre-open window for Claude's tool
		loop without blocking the at-open execution path on it.
	*/
	_ = ctx // kept for back-compat with callers that still pass it
	_ = executor

	templateTrades := make([]templates.Trade, len(topTrades))
	for i, t := range topTrades {
		templateTrades[i] = templates.Trade{
			Symbol:           t.Symbol,
			ContractType:     t.ContractType,
			StrikePrice:      t.Strike(),
			Expiration:       t.ExpirationStr(),
			DTE:              t.DTEVal(),
			EstimatedPrice:   t.EstPrice(),
			Thesis:           t.Thesis,
			SentimentScore:   t.SentimentScore,
			CurrentPrice:     t.CurrentPrice,
			TargetPrice:      t.TargetPrice,
			StopLoss:         t.StopLossVal(),
			RiskLevel:        t.RiskLevel,
			Catalyst:         t.Catalyst,
			MentionCount:     t.MentionCount,
			Rank:             t.Rank,
			Score:            t.Score,
			Rationale:        t.Rationale,
			TargetOTMPct:     t.TargetOTMPct,
			MinDTE:           t.MinDTE,
			ContractResolved: t.ContractResolved(),
		}
	}

	yesterdayRecap := buildYesterdayRecap(db, date)
	htmlContent, err := templates.RenderEmail(templateTrades, modelLabel, yesterdayRecap)
	if err != nil {
		log.Printf("Error rendering email: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Email template rendering failed: %v", err))
		return
	}
	subject := fmt.Sprintf("Options Trades for %s", time.Now().Format("Monday, Jan 2"))

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		log.Println("No active subscribers, skipping email send")
		return
	}

	log.Printf("Sending email to %d subscribers...", len(recipients))
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, htmlContent, unsubURLBuilder(cfg))
	if res.Failed == res.Total && res.Total > 0 {
		log.Printf("Morning email delivery failed: 0/%d delivered (%s)", res.Total, res.FailureDetail())
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Morning email delivery failed: 0/%d landed", res.Total))
		return
	}
	if res.Failed > 0 {
		log.Printf("Morning email partial: %d/%d failed (%s)", res.Failed, res.Total, res.FailureDetail())
	}

	log.Println("Trade analysis complete and email sent!")
}

func buildYesterdayRecap(db *store.Store, todayDate string) *templates.YesterdayRecap {
	dates, err := db.GetTradeDates(10)
	if err != nil {
		log.Printf("buildYesterdayRecap: GetTradeDates: %v", err)
		return nil
	}
	for _, d := range dates {
		if d == todayDate {
			continue
		}
		summaries, err := db.GetEODSummaries(d)
		if err != nil {
			log.Printf("buildYesterdayRecap: GetEODSummaries(%s): %v", d, err)
			continue
		}
		if len(summaries) == 0 {
			continue
		}
		recap := &templates.YesterdayRecap{
			TotalTrades: len(summaries),
		}
		if t, err := time.Parse("2006-01-02", d); err == nil {
			recap.Date = t.Format("Jan 2")
		} else {
			recap.Date = d
		}
		bestPnL := -1e18
		worstPnL := 1e18
		for _, s := range summaries {
			pnl := (s.ClosingPrice - s.EntryPrice) * 100
			recap.TotalPnL += pnl
			if pnl > 0 {
				recap.Winners++
			} else if pnl < 0 {
				recap.Losers++
			}
			if pnl > bestPnL {
				bestPnL = pnl
				recap.BestSymbol = s.Symbol
				recap.BestPnL = pnl
			}
			if pnl < worstPnL {
				worstPnL = pnl
				recap.WorstSymbol = s.Symbol
				recap.WorstPnL = pnl
			}
		}
		return recap
	}
	return nil
}

func currentWeekRange() (string, string) {
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Now().In(loc)
	weekday := now.Weekday()
	daysFromMonday := int(weekday - time.Monday)
	if daysFromMonday < 0 {
		daysFromMonday += 7
	}
	monday := now.AddDate(0, 0, -daysFromMonday)
	friday := monday.AddDate(0, 0, 4)
	return monday.Format("2006-01-02"), friday.Format("2006-01-02")
}

/*
runExecuteAtOpen is the 9:30:00 ET cron entry point. Loads today's
saved candidates from the DB (written by runTradeAnalysis at 9:25),
hands them to the execagent, and lets Claude decide which to actually
trade and what specific contract spec to fire each one as.

Fire-and-forget: the agent's place_options_order tool returns the
Schwab order id immediately and the per-minute ReconcileOpenOrders
cron handles fill landing. The 9:35 CancelDanglingOpens +
SendExecutionSummary cron sequence reconciles the basket and emails
the operator one consolidated summary.

Error semantics: on agent failure (Anthropic API down, parse error,
tool layer refusing every order), the function logs and emails the
operator an error notification. Skip rows the agent persisted during
its run are preserved so the summary email still surfaces what the
agent thought before it crashed.
*/
func runExecuteAtOpen(cfg *config.Config, db *store.Store, emailClient *email.Client, agent *execagent.Agent) {
	/*
		8-minute budget. Claude's at-open run typically takes 30-90s
		(3 candidates × {chain fetch + 1-2 quote calls + optional
		web search} + final JSON). The 9:35 cancel-dangling cron is the
		downstream safety net for anything still WORKING at the broker.
	*/
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	date := todayDate()
	candidates, err := db.GetMorningTrades(date)
	if err != nil {
		log.Printf("execute-at-open: load morning trades: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("execute-at-open: load morning trades for %s: %v", date, err))
		return
	}
	if len(candidates) == 0 {
		log.Printf("execute-at-open: no saved candidates for %s (picker skipped or didn't finish in time)", date)
		return
	}

	log.Printf("execute-at-open: dispatching agent for %d candidates on %s", len(candidates), date)
	result, runErr := agent.Run(ctx, execagent.CandidatesFromTrades(candidates))
	if runErr != nil {
		log.Printf("execute-at-open: agent run error: %v", runErr)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("execute-at-open agent run for %s failed: %v", date, runErr))
		// Fall through to log whatever decisions did persist (the agent
		// returns a partial result on conversation errors so any orders
		// it managed to place before the failure are still visible).
	}

	if result == nil {
		return
	}

	buys, skips := 0, 0
	for _, d := range result.Decisions {
		switch d.Action {
		case execagent.ActionBuy:
			buys++
			log.Printf("execute-at-open: BUY rank=%d %s strike=$%.2f exp=%s limit=$%.2f order=%s",
				d.Rank, d.Symbol, d.Strike, d.Expiration, d.LimitPrice, d.OrderID)
		case execagent.ActionSkip:
			skips++
			log.Printf("execute-at-open: SKIP rank=%d %s reason=%q", d.Rank, d.Symbol, d.SkipReason)
		}
	}
	log.Printf("execute-at-open: agent done (%d buys, %d skips, note=%q)", buys, skips, result.FinalNote)
}

func runWeeklyEmail(cfg *config.Config, db *store.Store, emailClient *email.Client) {
	loc, _ := time.LoadLocation("America/New_York")
	startDate, endDate := currentWeekRange()

	summariesMap, err := db.GetSummariesForDateRange(startDate, endDate)
	if err != nil {
		log.Printf("Error getting weekly summaries: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Weekly email failed: %v", err))
		return
	}

	tradesMap, _ := db.GetTradesForDateRange(startDate, endDate)

	var dates []string
	for d := range summariesMap {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	var days []templates.WeeklyDayData
	totalTrades, totalWinners, totalLosers := 0, 0, 0
	totalPnL, totalInvested, totalReturn := 0.0, 0.0, 0.0
	bestTrade, worstTrade := "", ""
	bestPnL, worstPnL := 0.0, 0.0
	firstTrade := true

	for _, date := range dates {
		summaries := summariesMap[date]
		if len(summaries) == 0 {
			continue
		}

		dayRankMap := make(map[string]int)
		if dayTrades, ok := tradesMap[date]; ok {
			for _, t := range dayTrades {
				if !t.ContractResolved() {
					continue
				}
				key := t.Symbol + "|" + t.ContractType + "|" + fmt.Sprintf("%.2f", t.Strike())
				dayRankMap[key] = t.Rank
			}
		}

		dayTrades := make([]templates.SummaryTrade, len(summaries))
		dayWinners, dayLosers := 0, 0
		dayPnL := 0.0
		dayBest, dayWorst := "", ""
		dayBestPnL, dayWorstPnL := 0.0, 0.0
		dayFirstTrade := true

		for i, s := range summaries {
			pnlPerContract := (s.ClosingPrice - s.EntryPrice) * 100
			pctChange := 0.0
			if s.EntryPrice > 0 {
				pctChange = ((s.ClosingPrice - s.EntryPrice) / s.EntryPrice) * 100
			}
			stockPct := 0.0
			if s.StockOpen > 0 {
				stockPct = ((s.StockClose - s.StockOpen) / s.StockOpen) * 100
			}

			result := "FLAT"
			if pnlPerContract > 0 {
				result = "PROFIT"
				dayWinners++
			} else if pnlPerContract < 0 {
				result = "LOSS"
				dayLosers++
			}

			summaryKey := s.Symbol + "|" + s.ContractType + "|" + fmt.Sprintf("%.2f", s.StrikePrice)
			dayTrades[i] = templates.SummaryTrade{
				Symbol: s.Symbol, ContractType: s.ContractType,
				StrikePrice: s.StrikePrice, Expiration: s.Expiration,
				EntryPrice: s.EntryPrice, ClosingPrice: s.ClosingPrice,
				PriceChange:    s.ClosingPrice - s.EntryPrice,
				PctChange:      pctChange,
				StockOpen:      s.StockOpen,
				StockClose:     s.StockClose,
				StockPctChange: stockPct,
				Result:         result,
				Notes:          s.Notes,
				Rank:           dayRankMap[summaryKey],
			}

			dayPnL += pnlPerContract
			totalInvested += s.EntryPrice * 100
			totalReturn += s.ClosingPrice * 100

			if dayFirstTrade || pnlPerContract > dayBestPnL {
				dayBest = s.Symbol
				dayBestPnL = pnlPerContract
			}
			if dayFirstTrade || pnlPerContract < dayWorstPnL {
				dayWorst = s.Symbol
				dayWorstPnL = pnlPerContract
			}
			dayFirstTrade = false
		}

		t, _ := time.ParseInLocation("2006-01-02", date, loc)

		days = append(days, templates.WeeklyDayData{
			Date:        date,
			DayName:     t.Format("Monday"),
			TotalTrades: len(summaries),
			Winners:     dayWinners,
			Losers:      dayLosers,
			DayPnL:      dayPnL,
			BestTrade:   dayBest,
			BestPnL:     dayBestPnL,
			WorstTrade:  dayWorst,
			WorstPnL:    dayWorstPnL,
			Trades:      dayTrades,
		})

		totalTrades += len(summaries)
		totalWinners += dayWinners
		totalLosers += dayLosers
		totalPnL += dayPnL

		if firstTrade || dayBestPnL > bestPnL {
			bestTrade = dayBest
			bestPnL = dayBestPnL
		}
		if firstTrade || dayWorstPnL < worstPnL {
			worstTrade = dayWorst
			worstPnL = dayWorstPnL
		}
		firstTrade = false
	}

	if totalTrades == 0 {
		log.Println("No completed trades this week, skipping weekly email")
		return
	}

	startTime, _ := time.ParseInLocation("2006-01-02", startDate, loc)
	endTime, _ := time.ParseInLocation("2006-01-02", endDate, loc)
	weekRange := fmt.Sprintf("%s to %s", startTime.Format("Jan 2"), endTime.Format("Jan 2, 2006"))

	winRate := 0.0
	if totalWinners+totalLosers > 0 {
		winRate = float64(totalWinners) / float64(totalWinners+totalLosers) * 100
	}

	data := templates.WeeklyEmailData{
		Subject:       "Weekly Trading Report",
		WeekRange:     weekRange,
		Days:          days,
		TotalTrades:   totalTrades,
		TotalWinners:  totalWinners,
		TotalLosers:   totalLosers,
		TotalPnL:      totalPnL,
		WinRate:       winRate,
		TotalInvested: totalInvested,
		TotalReturn:   totalReturn,
		BestTrade:     bestTrade,
		BestPnL:       bestPnL,
		WorstTrade:    worstTrade,
		WorstPnL:      worstPnL,
		DashboardURL:  "https://vibetradez.com/dashboard",
	}

	htmlContent, err := templates.RenderWeeklyEmail(data)
	if err != nil {
		log.Printf("Error rendering weekly email: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Weekly email rendering failed: %v", err))
		return
	}

	subject := fmt.Sprintf("Weekly Report: %s", weekRange)

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		log.Println("No active subscribers, skipping weekly email")
		return
	}

	log.Printf("Sending weekly email to %d subscribers...", len(recipients))
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, htmlContent, unsubURLBuilder(cfg))
	if res.Failed == res.Total && res.Total > 0 {
		log.Printf("Weekly email delivery failed: 0/%d delivered (%s)", res.Total, res.FailureDetail())
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Weekly email delivery failed: 0/%d landed", res.Total))
		return
	}
	if res.Failed > 0 {
		log.Printf("Weekly email partial: %d/%d failed (%s)", res.Failed, res.Total, res.FailureDetail())
	}

	log.Println("Weekly email sent!")
}

func runEndOfDayAnalysis(cfg *config.Config, db *store.Store, claudePicker *trades.ClaudePicker, emailClient *email.Client) {
	if claudePicker == nil {
		log.Println("Skipping EOD analysis: Claude picker not configured (local stub or missing key)")
		return
	}

	date := todayDate()

	savedTrades, err := db.GetMorningTrades(date)
	if err != nil {
		log.Printf("Error loading morning trades from database: %v", err)
		return
	}

	// Drop picks whose contract never resolved at open. EOD analysis runs
	// per-contract (Claude re-quotes each specific strike); a pick without
	// a strike/expiration has nothing to re-quote.
	resolvedTrades := savedTrades[:0:0]
	for _, t := range savedTrades {
		if t.ContractResolved() {
			resolvedTrades = append(resolvedTrades, t)
		}
	}
	savedTrades = resolvedTrades

	if len(savedTrades) == 0 {
		log.Println("Skipping EOD summary: no resolved morning trades found for today")
		return
	}

	/*
		EOD analysis is a fixed-shape task (re-quote N contracts, compute
		close prices) so the wall-clock budget is much shorter than the
		morning picker, but still generous enough to absorb retries against
		a slow or rate-limited Schwab.
	*/
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	log.Printf("Starting end-of-day analysis for %d trades...", len(savedTrades))

	summaries, err := claudePicker.GetEndOfDayAnalysis(ctx, savedTrades)
	if err != nil {
		log.Printf("Error getting EOD analysis: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("EOD analysis failed: %v", err))
		return
	}
	log.Printf("Got %d trade summaries", len(summaries))

	if err := db.SaveEODSummaries(date, summaries); err != nil {
		log.Printf("Error saving summaries to database: %v", err)
	}

	type morningMeta struct {
		Rank  int
		Score int
	}
	morningByKey := make(map[string]morningMeta)
	for _, t := range savedTrades {
		if !t.ContractResolved() {
			continue
		}
		key := t.Symbol + "|" + t.ContractType + "|" + fmt.Sprintf("%.2f", t.Strike())
		morningByKey[key] = morningMeta{
			Rank:  t.Rank,
			Score: t.Score,
		}
	}

	templateSummaries := make([]templates.SummaryTrade, len(summaries))
	for i, s := range summaries {
		key := s.Symbol + "|" + s.ContractType + "|" + fmt.Sprintf("%.2f", s.StrikePrice)
		meta := morningByKey[key]
		templateSummaries[i] = templates.SummaryTrade{
			Symbol:       s.Symbol,
			ContractType: s.ContractType,
			StrikePrice:  s.StrikePrice,
			Expiration:   s.Expiration,
			EntryPrice:   s.EntryPrice,
			ClosingPrice: s.ClosingPrice,
			StockOpen:    s.StockOpen,
			StockClose:   s.StockClose,
			Notes:        s.Notes,
			Rank:         meta.Rank,
			Score:        meta.Score,
		}
	}

	htmlContent, err := templates.RenderSummaryEmail(templateSummaries)
	if err != nil {
		log.Printf("Error rendering summary email: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Summary email rendering failed: %v", err))
		return
	}
	subject := fmt.Sprintf("EOD Summary for %s", time.Now().Format("Monday, Jan 2"))

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		log.Println("No active subscribers, skipping EOD email send")
		return
	}

	log.Printf("Sending EOD summary email to %d subscribers...", len(recipients))
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, htmlContent, unsubURLBuilder(cfg))
	if res.Failed == res.Total && res.Total > 0 {
		log.Printf("EOD email delivery failed: 0/%d delivered (%s)", res.Total, res.FailureDetail())
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("EOD email delivery failed: 0/%d landed", res.Total))
		return
	}
	if res.Failed > 0 {
		log.Printf("EOD email partial: %d/%d failed (%s)", res.Failed, res.Total, res.FailureDetail())
	}

	log.Println("EOD summary complete and email sent!")
}
