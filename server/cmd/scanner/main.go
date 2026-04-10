package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"vibetradez.com/internal/config"
	"vibetradez.com/internal/email"
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
	// 2025
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
	// 2026
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

func isMarketOpen() (bool, string) {
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")

	// Check for holiday
	if holiday, exists := marketHolidays[today]; exists {
		return false, holiday
	}

	// Check for weekend (should already be handled by cron, but double-check)
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false, "Weekend"
	}

	return true, ""
}

func todayDate() string {
	loc, _ := time.LoadLocation("America/New_York")
	return time.Now().In(loc).Format("2006-01-02")
}

// isLocalStubKey detects the placeholder API keys used by the local Docker
// stack so the cron-driven analyzers / validators can be safely skipped.
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

	if cfg.ResendAPIKey == "" {
		log.Fatal("RESEND_API_KEY is required")
	}
	if cfg.OpenAIAPIKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	db, err := store.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Seed subscribers from EMAIL_RECIPIENTS env var (for backward compatibility)
	if len(cfg.EmailRecipients) > 0 {
		for _, email := range cfg.EmailRecipients {
			if err := db.AddSubscriber(email, ""); err != nil {
				log.Printf("Warning: failed to seed subscriber %s: %v", email, err)
			}
		}
		log.Printf("Seeded %d subscribers from EMAIL_RECIPIENTS", len(cfg.EmailRecipients))
	}

	// Initialize Schwab client (optional — live data features degrade gracefully).
	var schwabClient *schwab.Client
	if cfg.SchwabAppKey != "" && cfg.SchwabSecret != "" {
		schwabClient = schwab.NewClient(cfg.SchwabAppKey, cfg.SchwabSecret, cfg.SchwabCallbackURL, db)
		if schwabClient.IsConnected() {
			log.Println("Schwab: connected (tokens loaded)")
		} else {
			log.Printf("Schwab: configured but not authorized — visit https://vibetradez.com/auth/schwab to connect")
		}
	} else {
		log.Println("Schwab: not configured (SCHWAB_APP_KEY / SCHWAB_SECRET not set)")
	}

	scraper := sentiment.NewScraper()
	analyzer := trades.NewAnalyzer(cfg.OpenAIAPIKey, cfg.OpenAIModel, schwabClient)
	emailClient := email.NewClient(cfg.ResendAPIKey)
	log.Printf("OpenAI: model=%s", cfg.OpenAIModel)

	// Claude validator is optional. If ANTHROPIC_API_KEY is missing or set
	// to a local stub, validations are skipped and trades persist with
	// claude_score = 0 and an empty rationale.
	var validator *trades.Validator
	switch {
	case cfg.AnthropicAPIKey == "":
		log.Println("Anthropic: not configured (ANTHROPIC_API_KEY not set) — Claude validation disabled")
	case isLocalStubKey(cfg.AnthropicAPIKey):
		log.Println("Anthropic: local stub key detected — Claude validation disabled")
	default:
		validator = trades.NewValidator(cfg.AnthropicAPIKey, cfg.AnthropicModel, schwabClient)
		log.Printf("Anthropic: configured — Claude validation enabled (model=%s)", cfg.AnthropicModel)
	}

	openJob := func() {
		if open, reason := isMarketOpen(); !open {
			log.Printf("Skipping morning analysis: Market closed (%s)", reason)
			return
		}
		runTradeAnalysis(cfg, db, scraper, analyzer, validator, emailClient)
	}

	closeJob := func() {
		if open, reason := isMarketOpen(); !open {
			log.Printf("Skipping EOD summary: Market closed (%s)", reason)
			return
		}
		runEndOfDayAnalysis(cfg, db, analyzer, emailClient)
	}

	weeklyJob := func() {
		runWeeklyEmail(cfg, db, emailClient)
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("Failed to load timezone: %v", err)
	}

	c := cron.New(cron.WithLocation(loc))

	_, err = c.AddFunc(cfg.CronScheduleOpen, openJob)
	if err != nil {
		log.Fatalf("Failed to add market open cron job: %v", err)
	}

	_, err = c.AddFunc(cfg.CronScheduleClose, closeJob)
	if err != nil {
		log.Fatalf("Failed to add market close cron job: %v", err)
	}

	_, err = c.AddFunc(cfg.CronScheduleWeekly, weeklyJob)
	if err != nil {
		log.Fatalf("Failed to add weekly email cron job: %v", err)
	}

	c.Start()

	// Start HTTP API server in background
	srv := server.New(db, schwabClient, emailClient, cfg.EmailFrom, cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.AnthropicAPIKey, cfg.AnthropicModel, cfg.AdminKey, cfg.ServerPort)
	go srv.Start()

	log.Printf("Options trade scanner started")
	log.Printf("Database: PostgreSQL")
	log.Printf("API server: :%s", cfg.ServerPort)
	log.Printf("Market open schedule: %s (ET)", cfg.CronScheduleOpen)
	log.Printf("Market close schedule: %s (ET)", cfg.CronScheduleClose)
	log.Printf("Weekly email schedule: %s (ET)", cfg.CronScheduleWeekly)

	// Log current subscriber count
	if subs, err := db.GetActiveSubscribers(); err == nil {
		log.Printf("Active subscribers: %d", len(subs))
	}

	// Startup e2e verification: render all templates with sample data and send a test email
	log.Println("Running startup verification...")
	if err := sendStartupTestEmail(cfg, db, schwabClient, emailClient); err != nil {
		log.Printf("Startup verification FAILED: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Startup verification failed: %v", err))
	} else {
		log.Println("Startup verification passed, test email sent")
	}

	// Run immediately on startup if RUN_ON_START is set
	if os.Getenv("RUN_ON_START") == "true" {
		log.Println("Running initial analysis...")
		openJob()
	}

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	c.Stop()
}

func checkHealth(name string, fn func() error) templates.HealthCheck {
	start := time.Now()
	err := fn()
	latency := time.Since(start)
	var lat string
	if latency < time.Millisecond {
		lat = fmt.Sprintf("%dμs", latency.Microseconds())
	} else {
		lat = fmt.Sprintf("%dms", latency.Milliseconds())
	}
	if err != nil {
		return templates.HealthCheck{Name: name, Status: "fail", Detail: err.Error(), Latency: lat}
	}
	return templates.HealthCheck{Name: name, Status: "ok", Detail: "Connected", Latency: lat}
}

func sendStartupTestEmail(cfg *config.Config, db *store.Store, schwabClient *schwab.Client, emailClient *email.Client) error {
	var checks []templates.HealthCheck

	// 1. Database connectivity
	c := checkHealth("PostgreSQL Database", func() error {
		dates, err := db.GetTradeDates(1)
		if err != nil {
			return err
		}
		_ = dates
		return nil
	})
	c.Detail = "Query OK"
	checks = append(checks, c)

	// 2. Template rendering (exercises all 4 templates)
	checks = append(checks, templates.VerifyTemplates())

	// 3. Reddit sentiment scraper
	c = checkHealth("Reddit Sentiment Scraper", func() error {
		scraper := sentiment.NewScraper()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := scraper.GetTrendingTickers(ctx, 5)
		return err
	})
	if c.Status == "ok" {
		c.Detail = "Reddit API reachable"
	}
	checks = append(checks, c)

	// 4. OpenAI API reachability (lightweight models list call)
	c = checkHealth("OpenAI API", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/models/gpt-5.4", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return nil
	})
	if c.Status == "ok" {
		c.Detail = "gpt-5.4 accessible"
	}
	checks = append(checks, c)

	// 5. Schwab Market Data API
	if schwabClient != nil {
		c = checkHealth("Schwab Market Data", func() error {
			if !schwabClient.IsConnected() {
				return fmt.Errorf("not authorized — visit /auth/schwab")
			}
			_, err := schwabClient.ValidToken()
			return err
		})
		if c.Status == "ok" {
			c.Detail = "Authenticated"
		}
	} else {
		c = templates.HealthCheck{Name: "Schwab Market Data", Status: "warn", Detail: "Not configured", Latency: "-"}
	}
	checks = append(checks, c)

	// 6. Resend email API (verified by actually sending this email)
	checks = append(checks, templates.HealthCheck{
		Name: "Resend Email API", Status: "ok", Detail: "Delivering this email", Latency: "-",
	})

	// 6. HTTP server
	c = checkHealth("HTTP API Server", func() error {
		resp, err := http.Get("http://localhost:" + cfg.ServerPort + "/health")
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return nil
	})
	if c.Status == "ok" {
		c.Detail = fmt.Sprintf("Listening on :%s", cfg.ServerPort)
	}
	checks = append(checks, c)

	// 7. Cron scheduler
	checks = append(checks, templates.HealthCheck{
		Name: "Cron Scheduler", Status: "ok",
		Detail:  fmt.Sprintf("Open %s / Close %s / Weekly %s", cfg.CronScheduleOpen, cfg.CronScheduleClose, cfg.CronScheduleWeekly),
		Latency: "-",
	})

	// 8. Subscriber check
	subs, _ := db.GetActiveSubscribers()
	subCheck := templates.HealthCheck{
		Name: "Subscriber List", Status: "ok",
		Detail: fmt.Sprintf("%d active subscribers", len(subs)), Latency: "-",
	}
	if len(subs) == 0 {
		subCheck.Status = "warn"
		subCheck.Detail = "No active subscribers"
	}
	checks = append(checks, subCheck)

	// Tally results
	passCount, warnCount, failCount := 0, 0, 0
	for _, ch := range checks {
		switch ch.Status {
		case "ok":
			passCount++
		case "warn":
			warnCount++
		case "fail":
			failCount++
		}
	}

	data := templates.StatusEmailData{
		Subject:      "System Online",
		Date:         time.Now().Format("Monday, Jan 2, 2006 3:04 PM ET"),
		Checks:       checks,
		AllPassed:    failCount == 0,
		PassCount:    passCount,
		WarnCount:    warnCount,
		FailCount:    failCount,
		TotalChecks:  len(checks),
		Subscribers:  len(subs),
		CronOpen:     cfg.CronScheduleOpen,
		CronClose:    cfg.CronScheduleClose,
		CronWeekly:   cfg.CronScheduleWeekly,
		ServerPort:   cfg.ServerPort,
		DashboardURL: "https://vibetradez.com",
		Model:        "gpt-5.4",
	}

	htmlContent, err := templates.RenderTestEmail(data)
	if err != nil {
		return fmt.Errorf("template rendering: %w", err)
	}

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		return fmt.Errorf("no active subscribers")
	}

	status := "All Systems Go"
	if failCount > 0 {
		status = fmt.Sprintf("%d Check(s) Failed", failCount)
	}
	subject := fmt.Sprintf("VibeTradez Deploy — %s — %s", status, time.Now().Format("Jan 2, 3:04 PM"))
	if err := emailClient.SendTradeEmail(cfg.EmailFrom, recipients, subject, htmlContent); err != nil {
		return fmt.Errorf("email delivery: %w", err)
	}

	return nil
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

	subject := fmt.Sprintf("VibeTradez Alert — %s", time.Now().Format("Jan 2, 3:04 PM"))
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

func runTradeAnalysis(cfg *config.Config, db *store.Store, scraper *sentiment.Scraper, analyzer *trades.Analyzer, validator *trades.Validator, emailClient *email.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	log.Println("Starting trade analysis...")

	// Get sentiment data from WSB
	log.Println("Scraping WSB sentiment...")
	sentimentData, err := scraper.GetTrendingTickers(ctx, 20)
	if err != nil {
		log.Printf("Warning: error getting sentiment data: %v", err)
		sentimentData = nil
	}
	log.Printf("Found %d trending tickers", len(sentimentData))

	// Get top 10 trades from OpenAI (works with or without sentiment data —
	// OpenAI will use web search to find trending tickers when Reddit fails)
	log.Println("Analyzing trades with OpenAI...")
	topTrades, err := analyzer.GetTopTrades(ctx, sentimentData)
	if err != nil {
		log.Printf("Error analyzing trades: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Trade analysis failed: %v", err))
		return
	}
	log.Printf("Generated %d trade recommendations", len(topTrades))

	if len(topTrades) == 0 {
		log.Println("No trades generated, skipping email")
		return
	}

	// Deduplicate tickers (keep first occurrence)
	seen := make(map[string]bool)
	var uniqueTrades []trades.Trade
	for _, t := range topTrades {
		if !seen[t.Symbol] {
			seen[t.Symbol] = true
			uniqueTrades = append(uniqueTrades, t)
		}
	}
	topTrades = uniqueTrades

	// Run Claude validation in parallel-friendly fashion (sequential here for
	// simpler error handling). Claude scrutinizes each pick independently and
	// returns a 1-10 score + rationale + concerns. We then merge those scores
	// onto the trades, compute a combined score (simple average with Claude
	// as the tiebreaker), and reorder ranks accordingly.
	if validator != nil {
		log.Println("Running Claude validation...")
		validations, vErr := validator.ValidateTrades(ctx, topTrades)
		if vErr != nil {
			log.Printf("Warning: Claude validation failed: %v — continuing with GPT-only ranks", vErr)
		} else {
			byTicker := make(map[string]trades.Validation, len(validations))
			for _, v := range validations {
				byTicker[v.Symbol] = v
			}
			for i := range topTrades {
				if v, ok := byTicker[topTrades[i].Symbol]; ok {
					topTrades[i].ClaudeScore = v.Score
					topTrades[i].ClaudeRationale = v.Rationale
				}
				topTrades[i].CombinedScore = float64(topTrades[i].GPTScore+topTrades[i].ClaudeScore) / 2.0
			}
			sort.SliceStable(topTrades, func(i, j int) bool {
				if topTrades[i].CombinedScore != topTrades[j].CombinedScore {
					return topTrades[i].CombinedScore > topTrades[j].CombinedScore
				}
				return topTrades[i].ClaudeScore > topTrades[j].ClaudeScore
			})
			for i := range topTrades {
				topTrades[i].Rank = i + 1
			}
			log.Printf("Claude validation complete: %d trades scored and reranked", len(validations))
		}
	} else {
		// Without Claude, mirror the GPT score into combined for consistency.
		for i := range topTrades {
			topTrades[i].CombinedScore = float64(topTrades[i].GPTScore)
		}
	}

	// Persist to database
	date := todayDate()
	if err := db.SaveMorningTrades(date, topTrades); err != nil {
		log.Printf("Error saving trades to database: %v", err)
		return
	}
	log.Printf("Saved %d trades to database for %s", len(topTrades), date)

	// Convert to template trades
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
			ProfitTarget:   t.ProfitTarget,
			RiskLevel:      t.RiskLevel,
			Catalyst:       t.Catalyst,
			MentionCount:   t.MentionCount,
			Rank:           t.Rank,
		}
	}

	// Render email template
	htmlContent, err := templates.RenderEmail(templateTrades)
	if err != nil {
		log.Printf("Error rendering email: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Email template rendering failed: %v", err))
		return
	}
	subject := fmt.Sprintf("Options Trades for %s", time.Now().Format("Monday, Jan 2"))

	// Get recipients from database
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

	// Get trades (with ranks) for the same date range
	tradesMap, _ := db.GetTradesForDateRange(startDate, endDate)

	// Get sorted dates
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

		// Build rank lookup for this day
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
	weekRange := fmt.Sprintf("%s - %s", startTime.Format("Jan 2"), endTime.Format("Jan 2, 2006"))

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
		DashboardURL:  "https://vibetradez.com",
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

func runEndOfDayAnalysis(cfg *config.Config, db *store.Store, analyzer *trades.Analyzer, emailClient *email.Client) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Printf("Starting end-of-day analysis for %d trades...", len(savedTrades))

	summaries, err := analyzer.GetEndOfDayAnalysis(ctx, savedTrades)
	if err != nil {
		log.Printf("Error getting EOD analysis: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("EOD analysis failed: %v", err))
		return
	}
	log.Printf("Got %d trade summaries", len(summaries))

	// Persist summaries to database
	if err := db.SaveEODSummaries(date, summaries); err != nil {
		log.Printf("Error saving summaries to database: %v", err)
	}

	// Build rank lookup from morning trades
	rankMap := make(map[string]int)
	for _, t := range savedTrades {
		key := t.Symbol + "|" + t.ContractType + "|" + fmt.Sprintf("%.2f", t.StrikePrice)
		rankMap[key] = t.Rank
	}

	// Convert to template summary trades
	templateSummaries := make([]templates.SummaryTrade, len(summaries))
	for i, s := range summaries {
		key := s.Symbol + "|" + s.ContractType + "|" + fmt.Sprintf("%.2f", s.StrikePrice)
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
			Rank:         rankMap[key],
		}
	}

	htmlContent, err := templates.RenderSummaryEmail(templateSummaries)
	if err != nil {
		log.Printf("Error rendering summary email: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("Summary email rendering failed: %v", err))
		return
	}
	subject := fmt.Sprintf("EOD Summary for %s", time.Now().Format("Monday, Jan 2"))

	// Get recipients from database
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
