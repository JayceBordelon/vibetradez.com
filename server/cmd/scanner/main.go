package main

import (
	"context"
	"fmt"
	"log"
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

	// Claude picker is optional. When ANTHROPIC_API_KEY is missing or
	// set to a local stub the cron pipeline degenerates to OpenAI-only:
	// the union has only GPT picks, picked_by_claude stays false, and
	// the model filter on the dashboard shows an empty Claude column.
	var claudePicker *trades.ClaudePicker
	switch {
	case cfg.AnthropicAPIKey == "":
		log.Println("Anthropic: not configured (ANTHROPIC_API_KEY not set) — Claude picking disabled")
	case isLocalStubKey(cfg.AnthropicAPIKey):
		log.Println("Anthropic: local stub key detected — Claude picking disabled")
	default:
		claudePicker = trades.NewClaudePicker(cfg.AnthropicAPIKey, cfg.AnthropicModel, schwabClient)
		log.Printf("Anthropic: configured — Claude picking enabled (model=%s)", cfg.AnthropicModel)
	}

	openJob := func() {
		if open, reason := isMarketOpen(); !open {
			log.Printf("Skipping morning analysis: Market closed (%s)", reason)
			return
		}
		runTradeAnalysis(cfg, db, scraper, analyzer, claudePicker, emailClient)
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

// unionPicks merges two independent pick sets (one per model) into a
// single union of unique trades. When both models picked the same ticker
// the row carries both models' scores and rationales and both picked_by
// flags are set; the combined score is the average of the two non-zero
// scores. When only one model picked a ticker the other model's score
// stays at zero (no second-pass scoring) and the combined score is just
// that model's score. Final ranks are computed by combined score desc,
// with picks where BOTH models agreed boosted ahead of single-model
// picks at the same combined score so consensus picks bubble to the top.
func unionPicks(openaiTrades, claudeTrades []trades.Trade) []trades.Trade {
	bySymbol := make(map[string]*trades.Trade)

	upsert := func(t trades.Trade) {
		key := t.Symbol
		if existing, ok := bySymbol[key]; ok {
			// Merge: keep the row already in the map and overlay the
			// fields the other model contributed.
			if t.PickedByOpenAI {
				existing.PickedByOpenAI = true
				existing.GPTScore = t.GPTScore
				existing.GPTRationale = t.GPTRationale
			}
			if t.PickedByClaude {
				existing.PickedByClaude = true
				existing.ClaudeScore = t.ClaudeScore
				existing.ClaudeRationale = t.ClaudeRationale
			}
			// Prefer the more detailed contract data from whichever side
			// supplied non-zero values, falling through to existing.
			if existing.EstimatedPrice == 0 && t.EstimatedPrice != 0 {
				existing.EstimatedPrice = t.EstimatedPrice
				existing.StrikePrice = t.StrikePrice
				existing.Expiration = t.Expiration
				existing.DTE = t.DTE
				existing.ContractType = t.ContractType
			}
			if existing.Thesis == "" && t.Thesis != "" {
				existing.Thesis = t.Thesis
			}
			if existing.Catalyst == "" && t.Catalyst != "" {
				existing.Catalyst = t.Catalyst
			}
			if existing.CurrentPrice == 0 && t.CurrentPrice != 0 {
				existing.CurrentPrice = t.CurrentPrice
			}
		} else {
			tc := t
			bySymbol[key] = &tc
		}
	}

	for _, t := range openaiTrades {
		upsert(t)
	}
	for _, t := range claudeTrades {
		upsert(t)
	}

	out := make([]trades.Trade, 0, len(bySymbol))
	for _, t := range bySymbol {
		// Combined score is the average of the model scores that exist.
		// A consensus pick (both > 0) gets a real average; a single-model
		// pick gets just that model's score.
		var sum float64
		var n int
		if t.GPTScore > 0 {
			sum += float64(t.GPTScore)
			n++
		}
		if t.ClaudeScore > 0 {
			sum += float64(t.ClaudeScore)
			n++
		}
		if n > 0 {
			t.CombinedScore = sum / float64(n)
		}
		out = append(out, *t)
	}

	sort.SliceStable(out, func(i, j int) bool {
		// Primary: consensus picks (both models independently chose the
		// same ticker) ALWAYS rank above single-model-only picks. If two
		// models agree on a trade it carries more conviction than any
		// single model acting alone.
		ci := out[i].PickedByOpenAI && out[i].PickedByClaude
		cj := out[j].PickedByOpenAI && out[j].PickedByClaude
		if ci != cj {
			return ci
		}
		// Secondary: within the same consensus tier, sort by combined
		// score descending.
		if out[i].CombinedScore != out[j].CombinedScore {
			return out[i].CombinedScore > out[j].CombinedScore
		}
		// Tiebreak: stable order — symbol alphabetical.
		return out[i].Symbol < out[j].Symbol
	})

	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

func runTradeAnalysis(cfg *config.Config, db *store.Store, scraper *sentiment.Scraper, analyzer *trades.Analyzer, claudePicker *trades.ClaudePicker, emailClient *email.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
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

	// Both pickers run the SAME workflow with the same prompt against
	// the same raw sentiment data. They each independently produce up to
	// 10 ranked trades. The cron then unions both pick sets so the
	// dashboard can show every trade either model picked, with per-pick
	// attribution to whichever model(s) actually picked it.
	log.Println("Analyzing trades with OpenAI...")
	openaiTrades, err := analyzer.GetTopTrades(ctx, sentimentData)
	if err != nil {
		log.Printf("Error analyzing trades with OpenAI: %v", err)
		sendErrorNotification(cfg, db, emailClient, fmt.Sprintf("OpenAI analysis failed: %v", err))
		return
	}
	log.Printf("OpenAI produced %d picks", len(openaiTrades))

	var claudeTrades []trades.Trade
	if claudePicker != nil {
		log.Println("Analyzing trades with Claude...")
		ct, cErr := claudePicker.GetTopTrades(ctx, sentimentData)
		if cErr != nil {
			log.Printf("Warning: Claude picking failed: %v — falling back to OpenAI-only", cErr)
		} else {
			claudeTrades = ct
			log.Printf("Claude produced %d picks", len(claudeTrades))
		}
	}

	topTrades := unionPicks(openaiTrades, claudeTrades)
	if len(topTrades) == 0 {
		log.Println("No trades generated, skipping email")
		return
	}
	log.Printf("Union pick set: %d unique trades (openai=%d, claude=%d)",
		len(topTrades), len(openaiTrades), len(claudeTrades))

	// Persist to database
	date := todayDate()
	if err := db.SaveMorningTrades(date, topTrades); err != nil {
		log.Printf("Error saving trades to database: %v", err)
		return
	}
	log.Printf("Saved %d trades to database for %s", len(topTrades), date)

	// Convert to template trades, carrying the dual-model scores and
	// rationales through so the morning email can render the same
	// per-pick analysis the website shows.
	templateTrades := make([]templates.Trade, len(topTrades))
	for i, t := range topTrades {
		templateTrades[i] = templates.Trade{
			Symbol:          t.Symbol,
			ContractType:    t.ContractType,
			StrikePrice:     t.StrikePrice,
			Expiration:      t.Expiration,
			DTE:             t.DTE,
			EstimatedPrice:  t.EstimatedPrice,
			Thesis:          t.Thesis,
			SentimentScore:  t.SentimentScore,
			CurrentPrice:    t.CurrentPrice,
			TargetPrice:     t.TargetPrice,
			StopLoss:        t.StopLoss,
			ProfitTarget:    t.ProfitTarget,
			RiskLevel:       t.RiskLevel,
			Catalyst:        t.Catalyst,
			MentionCount:    t.MentionCount,
			Rank:            t.Rank,
			GPTScore:        t.GPTScore,
			GPTRationale:    t.GPTRationale,
			ClaudeScore:     t.ClaudeScore,
			ClaudeRationale: t.ClaudeRationale,
			CombinedScore:   t.CombinedScore,
			PickedByOpenAI:  t.PickedByOpenAI,
			PickedByClaude:  t.PickedByClaude,
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

	// Build a per-contract lookup that carries the rank AND each model's
	// score from the morning save, so the EOD email can attribute every
	// summary back to which model rated it highly.
	type morningMeta struct {
		Rank          int
		GPTScore      int
		ClaudeScore   int
		CombinedScore float64
	}
	morningByKey := make(map[string]morningMeta)
	for _, t := range savedTrades {
		key := t.Symbol + "|" + t.ContractType + "|" + fmt.Sprintf("%.2f", t.StrikePrice)
		morningByKey[key] = morningMeta{
			Rank:          t.Rank,
			GPTScore:      t.GPTScore,
			ClaudeScore:   t.ClaudeScore,
			CombinedScore: t.CombinedScore,
		}
	}

	// Convert to template summary trades, carrying the dual-model scores
	// alongside the realised P&L so the EOD email can show a per-model
	// attribution column and the leaderboard at the top.
	templateSummaries := make([]templates.SummaryTrade, len(summaries))
	for i, s := range summaries {
		key := s.Symbol + "|" + s.ContractType + "|" + fmt.Sprintf("%.2f", s.StrikePrice)
		meta := morningByKey[key]
		templateSummaries[i] = templates.SummaryTrade{
			Symbol:        s.Symbol,
			ContractType:  s.ContractType,
			StrikePrice:   s.StrikePrice,
			Expiration:    s.Expiration,
			EntryPrice:    s.EntryPrice,
			ClosingPrice:  s.ClosingPrice,
			StockOpen:     s.StockOpen,
			StockClose:    s.StockClose,
			Notes:         s.Notes,
			Rank:          meta.Rank,
			GPTScore:      meta.GPTScore,
			ClaudeScore:   meta.ClaudeScore,
			CombinedScore: meta.CombinedScore,
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
