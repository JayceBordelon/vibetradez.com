package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"vibetradez.com/internal/authclient"
	"vibetradez.com/internal/config"
	"vibetradez.com/internal/email"
	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/rollouts"
	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/sentiment"
	"vibetradez.com/internal/server"
	"vibetradez.com/internal/store"
	"vibetradez.com/internal/templates"
	"vibetradez.com/internal/trades"

	"github.com/robfig/cron/v3"
)

// US Market Holidays (NYSE/NASDAQ closed)
var marketHolidays = map[string]string{
	"2025-01-01": "New Year's Day",
	"2025-01-20": "MLK Day",
	"2025-02-17": "Presidents Day",
	"2025-04-18": "Good Friday",
	"2025-05-26": "Memorial Day",
	"2025-06-19": "Juneteenth",
	"2025-07-04": "Independence Day",
	"2025-09-01": "Labor Day",
	"2025-11-27": "Thanksgiving",
	"2025-12-25": "Christmas",
	"2026-01-01": "New Year's Day",
	"2026-01-19": "MLK Day",
	"2026-02-16": "Presidents Day",
	"2026-04-03": "Good Friday",
	"2026-05-25": "Memorial Day",
	"2026-06-19": "Juneteenth",
	"2026-07-03": "Independence Day (Observed)",
	"2026-09-07": "Labor Day",
	"2026-11-26": "Thanksgiving",
	"2026-12-25": "Christmas",
}

/*
US Market Half-Days (1pm ET early close instead of 4pm).
On these dates the auto-execution close cron must fire at 12:55pm
instead of 3:55pm. Update list yearly, NYSE publishes the schedule
in November of the prior year.
*/
var marketHalfDays = map[string]string{
	"2025-11-28": "Day after Thanksgiving",
	"2025-12-24": "Christmas Eve",
	"2026-11-27": "Day after Thanksgiving",
	"2026-12-24": "Christmas Eve",
}

func isHalfDay() bool {
	loc, _ := time.LoadLocation("America/New_York")
	today := time.Now().In(loc).Format("2006-01-02")
	_, ok := marketHalfDays[today]
	return ok
}

func isMarketOpen() (bool, string) {
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")

	if holiday, exists := marketHolidays[today]; exists {
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
		log.Printf("clock-skew WARNING: local clock differs from cloudflare by %s (threshold %s); the 3:55pm close cron and 5-minute confirmation window WILL fire at the wrong wall-clock time", skew.Truncate(time.Second), maxAcceptableSkew)
	} else {
		log.Printf("clock-skew probe: local clock within %s of cloudflare (rtt=%s)", skew.Truncate(time.Millisecond), rtt.Truncate(time.Millisecond))
	}
}

/*
isLocalStubKey detects the placeholder API keys used by the local Docker
stack so the cron can be safely skipped without making real API calls.
*/
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
			if err := db.AddSubscriber(email, ""); err != nil {
				log.Printf("Warning: failed to seed subscriber %s: %v", email, err)
			}
		}
		log.Printf("Seeded %d subscribers from EMAIL_RECIPIENTS", len(cfg.EmailRecipients))
	}

	var schwabClient *schwab.Client
	if cfg.SchwabAppKey != "" && cfg.SchwabSecret != "" {
		schwabClient = schwab.NewClient(cfg.SchwabAppKey, cfg.SchwabSecret, cfg.SchwabCallbackURL, db)
		if schwabClient.IsConnected() {
			log.Println("Schwab: connected (tokens loaded)")
		} else {
			log.Printf("Schwab: configured but not authorized, visit https://vibetradez.com/auth/schwab to connect")
		}
	} else {
		log.Println("Schwab: not configured (SCHWAB_APP_KEY / SCHWAB_SECRET not set)")
	}

	authClient := authclient.New(cfg.AuthBaseURL, cfg.AuthClientID, cfg.AuthClientSecret, cfg.AuthRedirectURI)

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
			OptionAsk:         schwabClient.OptionAsk,
		}
		executor = exec.NewService(db, trader, emailClient, execCfg)
	}

	openJob := func() {
		if open, reason := isMarketOpen(); !open {
			log.Printf("Skipping morning analysis: Market closed (%s)", reason)
			return
		}
		runTradeAnalysis(cfg, db, scraper, claudePicker, schwabClient, emailClient, modelLabel, executor)
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
		log.Fatalf("Failed to add market open cron job: %v", err)
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
		operator only discovers the dead token when the 9:25 ET cron
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
		}); err != nil {
			log.Fatalf("Failed to add 12:55pm half-day close cron: %v", err)
		}
		log.Printf("execution: cron registered (open-reconcile every minute 9-15 ET, close 3:55pm or 12:55pm half-days)")
	}

	c.Start()

	sessionTTL := time.Duration(cfg.SessionTTLDays) * 24 * time.Hour
	srv := server.New(db, schwabClient, authClient, scraper, emailClient, cfg.EmailFrom, cfg.AnthropicAPIKey, cfg.AnthropicModel, cfg.SessionCookieName, sessionTTL, cfg.AuthPublicURL, cfg.AuthClientID, cfg.AuthRedirectURI, cfg.ServerPort, executor, cfg.ExecutionRecipient)
	go srv.Start()

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

	go rollouts.Run(db, emailClient, cfg.EmailFrom, isLocalStubKey(cfg.ResendAPIKey))

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
validatePicksAgainstSpot drops picks whose strike is more than 25%
away from current Schwab spot. Claude has produced internally
inconsistent picks where its `current_price` field disagrees wildly
with what Schwab's live feed reports (e.g., picking a $115 strike on
a name Schwab shows at $750), which then surfaces on the dashboard as
absurd "Current" marks like +44,000% gains on a deep-ITM contract the
picker thought was slightly OTM. Cross-checking against the same
Schwab data source the dashboard reads from catches that class of bug
before the morning email ships or the executor tries to act on it. A
nil schwab client (local dev) is a no-op pass-through, as is a
GetQuotes failure (we prefer shipping unvalidated picks to dropping
everything on a transient probe error).
*/
func validatePicksAgainstSpot(picks []trades.Trade, schwabClient *schwab.Client) []trades.Trade {
	const maxStrikeDistance = 0.25

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
	quotes, err := schwabClient.GetQuotes(symbols)
	if err != nil {
		log.Printf("pick validation: GetQuotes failed, passing picks through unchecked: %v", err)
		return picks
	}

	keep := make([]trades.Trade, 0, len(picks))
	for _, p := range picks {
		q, ok := quotes[p.Symbol]
		if !ok || q.LastPrice <= 0 {
			log.Printf("pick validation: no spot for %s, keeping #%d", p.Symbol, p.Rank)
			keep = append(keep, p)
			continue
		}
		distance := (p.StrikePrice - q.LastPrice) / q.LastPrice
		if distance < 0 {
			distance = -distance
		}
		if distance > maxStrikeDistance {
			log.Printf("pick validation: DROPPING #%d %s %s $%.2f (spot=$%.2f, %.0f%% from spot, claude_current=$%.2f)",
				p.Rank, p.Symbol, p.ContractType, p.StrikePrice, q.LastPrice, distance*100, p.CurrentPrice)
			continue
		}
		keep = append(keep, p)
	}
	return keep
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
	if err := emailClient.SendTradeEmail(cfg.EmailFrom, recipients, subject, htmlContent); err != nil {
		log.Printf("Failed to send error notification email: %v", err)
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
	topTrades = validatePicksAgainstSpot(topTrades, schwabClient)
	if dropped := originalCount - len(topTrades); dropped > 0 {
		log.Printf("pick validation: dropped %d/%d picks with strikes too far from spot", dropped, originalCount)
		if dropped*2 > originalCount {
			sendErrorNotification(cfg, db, emailClient, fmt.Sprintf(
				"Pick validation dropped %d of %d picks (>50%%) for strike-spot divergence. Likely picker saw stale quotes. Check Claude tool logs for the morning run.",
				dropped, originalCount))
		}
	}
	if len(topTrades) == 0 {
		log.Println("All picks dropped by validation, skipping email")
		return
	}

	date := todayDate()
	if err := db.SaveMorningTrades(date, topTrades); err != nil {
		log.Printf("Error saving trades to database: %v", err)
		return
	}
	log.Printf("Saved %d trades to database for %s", len(topTrades), date)

	/*
		Auto-execution gate: only runs if TRADING_ENABLED. The selector
		walks the top MaxBasketRank ranks greedily — including each pick
		whose modeled premium fits in the remaining MaxDailyBasketUSD
		budget, skip-and-continue when a contract is too big — and the
		service places those orders sequentially after a Schwab cash
		check. Score is not a gate; rank order is the only signal.
	*/
	if executor != nil {
		picks := exec.QualifyingPicks(topTrades)
		if len(picks) > 0 {
			summary := make([]string, 0, len(picks))
			for _, p := range picks {
				summary = append(summary, fmt.Sprintf("rank=%d %s %s@%.2f", p.Rank, p.Symbol, p.ContractType, p.EstimatedPrice))
			}
			log.Printf("execution: basket of %d pick(s) selected [%s] (mode=%s)", len(picks), strings.Join(summary, ", "), executor.Mode())
			if _, err := executor.HandleQualifyingPicks(ctx, picks); err != nil {
				log.Printf("execution: handle qualifying picks: %v", err)
			}
		} else {
			log.Printf("execution: no qualifying picks today (top-%d ranks missing, all premium > $%.2f cap, or basket cap $%.2f exhausted)",
				exec.MaxBasketRank, exec.MaxContractPremium, exec.MaxDailyBasketUSD)
		}
	}

	templateTrades := make([]templates.Trade, len(topTrades))
	for i, t := range topTrades {
		templateTrades[i] = templates.Trade{
			Symbol:         t.Symbol,
			ContractType:   t.ContractType,
			StrikePrice:    t.StrikePrice,
			Expiration:     t.Expiration,
			DTE:            t.DTE,
			EstimatedPrice: t.EstimatedPrice,
			Thesis:         t.Thesis,
			SentimentScore: t.SentimentScore,
			CurrentPrice:   t.CurrentPrice,
			TargetPrice:    t.TargetPrice,
			StopLoss:       t.StopLoss,
			RiskLevel:      t.RiskLevel,
			Catalyst:       t.Catalyst,
			MentionCount:   t.MentionCount,
			Rank:           t.Rank,
			Score:          t.Score,
			Rationale:      t.Rationale,
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
	if err := emailClient.SendTradeEmail(cfg.EmailFrom, recipients, subject, htmlContent); err != nil {
		log.Printf("Error sending email: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Email delivery failed: %v", err))
		return
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
				key := t.Symbol + "|" + t.ContractType + "|" + fmt.Sprintf("%.2f", t.StrikePrice)
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
	if err := emailClient.SendTradeEmail(cfg.EmailFrom, recipients, subject, htmlContent); err != nil {
		log.Printf("Error sending weekly email: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Weekly email delivery failed: %v", err))
		return
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

	if len(savedTrades) == 0 {
		log.Println("Skipping EOD summary: no morning trades found for today")
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
		key := t.Symbol + "|" + t.ContractType + "|" + fmt.Sprintf("%.2f", t.StrikePrice)
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
	if err := emailClient.SendTradeEmail(cfg.EmailFrom, recipients, subject, htmlContent); err != nil {
		log.Printf("Error sending EOD email: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("EOD email delivery failed: %v", err))
		return
	}

	log.Println("EOD summary complete and email sent!")
}
