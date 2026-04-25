package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"sort"
	"time"
)

//go:embed email.html summary.html test.html error.html weekly.html
var templateFS embed.FS

type Trade struct {
	Symbol          string
	ContractType    string
	StrikePrice     float64
	Expiration      string
	DTE             int
	EstimatedPrice  float64
	Thesis          string
	SentimentScore  float64
	CurrentPrice    float64
	TargetPrice     float64
	StopLoss        float64
	ProfitTarget    float64
	RiskLevel       string
	Catalyst        string
	MentionCount    int
	Rank            int
	GPTScore        int
	GPTRationale    string
	ClaudeScore     int
	ClaudeRationale string
	CombinedScore   float64
	PickedByOpenAI  bool
	PickedByClaude  bool
	GPTVerdict      string
	ClaudeVerdict   string
}

// YesterdayRecap is a tiny digest of the previous trading day's results
// surfaced at the top of the morning email so subscribers see how the
// last batch performed before reading today's picks.
type YesterdayRecap struct {
	Date        string // formatted, e.g. "Apr 24"
	TotalPnL    float64
	Winners     int
	Losers      int
	TotalTrades int
	BestSymbol  string
	BestPnL     float64
	WorstSymbol string
	WorstPnL    float64
}

type EmailData struct {
	Subject         string
	Date            string
	Trades          []Trade
	GPTModelName    string
	ClaudeModelName string
	Yesterday       *YesterdayRecap // nil = no recap available (first send, EOD cron failed, etc.)
	ClaudeTopPick   *Trade          // Claude's #1 by ClaudeScore; nil if Claude picked nothing
	GPTTopPick      *Trade          // ChatGPT's #1 by GPTScore; nil if GPT picked nothing
	DashboardURL    string
}

type SummaryTrade struct {
	Symbol         string
	ContractType   string
	StrikePrice    float64
	Expiration     string
	EntryPrice     float64
	ClosingPrice   float64
	PriceChange    float64
	PctChange      float64
	StockOpen      float64
	StockClose     float64
	StockPctChange float64
	Result         string
	Notes          string
	Rank           int
	GPTScore       int
	ClaudeScore    int
	CombinedScore  float64
}

type SummaryEmailData struct {
	Subject     string
	Date        string
	Trades      []SummaryTrade
	TotalTrades int
	Winners     int
	Losers      int
	TotalPnL    float64
	// Dual-model attribution: P&L by which model picked the top 3 trades
	// (sorted by each model's score). Lets the EOD email surface a tiny
	// leaderboard answering "which model would you have made more money
	// listening to today?".
	GPTTop3Pnl       float64
	ClaudeTop3Pnl    float64
	CombinedTop3Pnl  float64
	GPTAvgScore      float64
	ClaudeAvgScore   float64
	AgreementPercent float64
}

type WeeklyDayData struct {
	Date        string
	DayName     string
	TotalTrades int
	Winners     int
	Losers      int
	DayPnL      float64
	BestTrade   string
	BestPnL     float64
	WorstTrade  string
	WorstPnL    float64
	Trades      []SummaryTrade
}

type WeeklyEmailData struct {
	Subject       string
	WeekRange     string
	Days          []WeeklyDayData
	TotalTrades   int
	TotalWinners  int
	TotalLosers   int
	TotalPnL      float64
	WinRate       float64
	TotalInvested float64
	TotalReturn   float64
	BestTrade     string
	BestPnL       float64
	WorstTrade    string
	WorstPnL      float64
	DashboardURL  string
	// Per-model backtest aggregates over the week, mirroring the
	// /api/model-comparison response. The weekly email surfaces these so
	// subscribers see which model's ranking would have produced the most
	// P&L if followed in isolation.
	GPTTotalPnl      float64
	GPTWinRate       float64
	ClaudeTotalPnl   float64
	ClaudeWinRate    float64
	CombinedTotalPnl float64
	CombinedWinRate  float64
	AgreementPercent float64
}

var funcMap = template.FuncMap{
	"mul": func(a, b float64) float64 { return a * b },
	"div": func(a, b float64) float64 {
		if b == 0 {
			return 0
		}
		return a / b
	},
	"sub": func(a, b any) any {
		switch av := a.(type) {
		case int:
			if bv, ok := b.(int); ok {
				return av - bv
			}
		case float64:
			if bv, ok := b.(float64); ok {
				return av - bv
			}
		}
		return 0
	},
	"add": func(a, b any) any {
		switch av := a.(type) {
		case int:
			if bv, ok := b.(int); ok {
				return av + bv
			}
		case float64:
			if bv, ok := b.(float64); ok {
				return av + bv
			}
		}
		return 0
	},
	"abs": func(a float64) float64 {
		if a < 0 {
			return -a
		}
		return a
	},
	"gt": func(a, b any) bool {
		switch av := a.(type) {
		case float64:
			if bv, ok := b.(float64); ok {
				return av > bv
			}
		case int:
			if bv, ok := b.(int); ok {
				return av > bv
			}
		}
		return false
	},
	"lt": func(a, b any) bool {
		switch av := a.(type) {
		case float64:
			if bv, ok := b.(float64); ok {
				return av < bv
			}
		case int:
			if bv, ok := b.(int); ok {
				return av < bv
			}
		}
		return false
	},
}

func RenderEmail(trades []Trade, gptModelName, claudeModelName string, yesterday *YesterdayRecap) (string, error) {
	tmpl, err := template.New("email.html").Funcs(funcMap).ParseFS(templateFS, "email.html")
	if err != nil {
		return "", err
	}

	// Pick each model's #1 conviction trade independently — Claude's top is
	// the highest-ClaudeScore trade Claude actually picked, ditto for GPT.
	// They may resolve to the same underlying ticker on consensus days; the
	// template renders both sections regardless so each model leads with its
	// own rationale.
	var claudeTop, gptTop *Trade
	for i := range trades {
		t := &trades[i]
		if t.PickedByClaude && (claudeTop == nil || t.ClaudeScore > claudeTop.ClaudeScore) {
			claudeTop = t
		}
		if t.PickedByOpenAI && (gptTop == nil || t.GPTScore > gptTop.GPTScore) {
			gptTop = t
		}
	}

	data := EmailData{
		Subject:         "Today's Top Options Plays",
		Date:            time.Now().Format("Monday, Jan 2, 2006"),
		Trades:          trades,
		GPTModelName:    gptModelName,
		ClaudeModelName: claudeModelName,
		Yesterday:       yesterday,
		ClaudeTopPick:   claudeTop,
		GPTTopPick:      gptTop,
		DashboardURL:    "https://vibetradez.com/dashboard",
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

type HealthCheck struct {
	Name    string
	Status  string // "ok", "warn", "fail"
	Detail  string
	Latency string // e.g. "12ms"
}

type StatusEmailData struct {
	Subject      string
	Date         string
	Checks       []HealthCheck
	AllPassed    bool
	PassCount    int
	WarnCount    int
	FailCount    int
	TotalChecks  int
	Subscribers  int
	CronOpen     string
	CronClose    string
	CronWeekly   string
	ServerPort   string
	DashboardURL string
	Model        string
}

type ErrorEmailData struct {
	Subject string
	Date    string
	Error   string
}

// VerifyTemplates exercises all email templates with sample data to catch rendering errors.
// Returns a HealthCheck for template rendering.
func VerifyTemplates() HealthCheck {
	start := time.Now()

	sampleTrades := []Trade{
		{
			Symbol: "SPY", ContractType: "CALL", StrikePrice: 500,
			Expiration: "2026-04-01", DTE: 5, EstimatedPrice: 1.50,
			Thesis: "Startup verification", SentimentScore: 0.5,
			CurrentPrice: 498, TargetPrice: 505, StopLoss: 0.75,
			ProfitTarget: 3.00, RiskLevel: "MEDIUM",
			Catalyst: "System test", MentionCount: 42,
			Rank: 1, GPTScore: 9, ClaudeScore: 8, CombinedScore: 8.5,
			PickedByOpenAI: true, PickedByClaude: true,
			GPTRationale: "Sample bullish rationale.", ClaudeRationale: "Sample bullish rationale.",
			GPTVerdict: "Concur on direction.", ClaudeVerdict: "Aligned, slight strike concern.",
		},
	}
	sampleYesterday := &YesterdayRecap{
		Date:        "Apr 24",
		TotalPnL:    142.50,
		Winners:     3,
		Losers:      2,
		TotalTrades: 5,
		BestSymbol:  "SPY",
		BestPnL:     85.00,
		WorstSymbol: "QQQ",
		WorstPnL:    -22.00,
	}
	if _, err := RenderEmail(sampleTrades, "ChatGPT", "Claude", sampleYesterday); err != nil {
		return HealthCheck{Name: "Email Templates", Status: "fail", Detail: err.Error(), Latency: fmtLatency(start)}
	}

	sampleSummaries := []SummaryTrade{
		{
			Symbol: "SPY", ContractType: "CALL", StrikePrice: 500,
			Expiration: "2026-04-01", EntryPrice: 1.50, ClosingPrice: 2.10,
			StockOpen: 498, StockClose: 503, Notes: "Startup verification",
		},
	}
	if _, err := RenderSummaryEmail(sampleSummaries); err != nil {
		return HealthCheck{Name: "Email Templates", Status: "fail", Detail: err.Error(), Latency: fmtLatency(start)}
	}

	sampleWeekly := WeeklyEmailData{
		Subject: "Weekly Report", WeekRange: "Mar 25 - Mar 29, 2026",
		Days: []WeeklyDayData{
			{
				Date: "2026-03-25", DayName: "Monday",
				TotalTrades: 1, Winners: 1, DayPnL: 60.0,
				BestTrade: "SPY", BestPnL: 60.0,
				WorstTrade: "SPY", WorstPnL: 60.0,
				Trades: sampleSummaries,
			},
		},
		TotalTrades: 1, TotalWinners: 1, TotalPnL: 60.0,
		WinRate: 100.0, TotalInvested: 150.0, TotalReturn: 210.0,
		BestTrade: "SPY", BestPnL: 60.0, WorstTrade: "SPY", WorstPnL: 60.0,
		DashboardURL: "https://vibetradez.com/dashboard",
	}
	if _, err := RenderWeeklyEmail(sampleWeekly); err != nil {
		return HealthCheck{Name: "Email Templates", Status: "fail", Detail: err.Error(), Latency: fmtLatency(start)}
	}

	return HealthCheck{Name: "Email Templates", Status: "ok", Detail: "All 4 templates rendered", Latency: fmtLatency(start)}
}

// topNPnl picks the N highest-scoring summaries by the given score
// selector and sums their per-contract P&L. Used by both the EOD and
// weekly emails to backtest "what if you had only followed this model
// today / this week" without re-fetching the trades table.
func topNPnl(trades []SummaryTrade, n int, score func(SummaryTrade) float64) float64 {
	if len(trades) == 0 {
		return 0
	}
	sorted := make([]SummaryTrade, len(trades))
	copy(sorted, trades)
	sort.SliceStable(sorted, func(i, j int) bool {
		return score(sorted[i]) > score(sorted[j])
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	var total float64
	for _, t := range sorted {
		total += (t.ClosingPrice - t.EntryPrice) * 100
	}
	return total
}

func fmtLatency(start time.Time) string {
	d := time.Since(start)
	if d < time.Millisecond {
		return fmt.Sprintf("%dμs", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}

// RenderTestEmail renders the startup health check email with the provided data.
func RenderTestEmail(data StatusEmailData) (string, error) {
	tmpl, err := template.New("test.html").Funcs(funcMap).ParseFS(templateFS, "test.html")
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RenderErrorEmail renders an error notification email. Kept intentionally simple
// (no loops, no comparisons) to minimize the chance of this template itself failing.
func RenderErrorEmail(errMsg string) (string, error) {
	tmpl, err := template.New("error.html").Funcs(funcMap).ParseFS(templateFS, "error.html")
	if err != nil {
		return "", err
	}

	data := ErrorEmailData{
		Subject: "System Alert",
		Date:    time.Now().Format("Monday, Jan 2, 2006 3:04 PM"),
		Error:   errMsg,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func RenderWeeklyEmail(data WeeklyEmailData) (string, error) {
	tmpl, err := template.New("weekly.html").Funcs(funcMap).ParseFS(templateFS, "weekly.html")
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func RenderSummaryEmail(summaryTrades []SummaryTrade) (string, error) {
	tmpl, err := template.New("summary.html").Funcs(funcMap).ParseFS(templateFS, "summary.html")
	if err != nil {
		return "", err
	}

	winners, losers := 0, 0
	totalPnL := 0.0
	for i := range summaryTrades {
		t := &summaryTrades[i]
		t.PriceChange = t.ClosingPrice - t.EntryPrice
		if t.EntryPrice > 0 {
			t.PctChange = (t.PriceChange / t.EntryPrice) * 100
		}
		if t.StockOpen > 0 {
			t.StockPctChange = ((t.StockClose - t.StockOpen) / t.StockOpen) * 100
		}
		if t.PriceChange > 0 {
			t.Result = "PROFIT"
			winners++
		} else if t.PriceChange < 0 {
			t.Result = "LOSS"
			losers++
		} else {
			t.Result = "FLAT"
		}
		totalPnL += t.PriceChange * 100 // per contract
	}

	// Per-model attribution: replay this single day's picks under each
	// model's ranking and aggregate the top-3 P&L for each, the same way
	// /api/model-comparison does for longer windows. Lets the EOD email
	// surface a tiny "which model would you have made more money
	// listening to today" leaderboard.
	gptTop3Pnl := topNPnl(summaryTrades, 3, func(t SummaryTrade) float64 { return float64(t.GPTScore) })
	claudeTop3Pnl := topNPnl(summaryTrades, 3, func(t SummaryTrade) float64 { return float64(t.ClaudeScore) })
	combinedTop3Pnl := topNPnl(summaryTrades, 3, func(t SummaryTrade) float64 { return t.CombinedScore })

	var gptScoreSum, claudeScoreSum float64
	var gptScoreCount, claudeScoreCount int
	var agree, dualScored int
	for _, t := range summaryTrades {
		if t.GPTScore > 0 {
			gptScoreSum += float64(t.GPTScore)
			gptScoreCount++
		}
		if t.ClaudeScore > 0 {
			claudeScoreSum += float64(t.ClaudeScore)
			claudeScoreCount++
		}
		if t.GPTScore > 0 && t.ClaudeScore > 0 {
			dualScored++
			diff := t.GPTScore - t.ClaudeScore
			if diff < 0 {
				diff = -diff
			}
			if diff <= 1 {
				agree++
			}
		}
	}
	var gptAvg, claudeAvg, agreementPct float64
	if gptScoreCount > 0 {
		gptAvg = gptScoreSum / float64(gptScoreCount)
	}
	if claudeScoreCount > 0 {
		claudeAvg = claudeScoreSum / float64(claudeScoreCount)
	}
	if dualScored > 0 {
		agreementPct = (float64(agree) / float64(dualScored)) * 100
	}

	data := SummaryEmailData{
		Subject:          "End of Day Trade Summary",
		Date:             time.Now().Format("Monday, Jan 2, 2006"),
		Trades:           summaryTrades,
		TotalTrades:      len(summaryTrades),
		Winners:          winners,
		Losers:           losers,
		TotalPnL:         totalPnL,
		GPTTop3Pnl:       gptTop3Pnl,
		ClaudeTop3Pnl:    claudeTop3Pnl,
		CombinedTop3Pnl:  combinedTop3Pnl,
		GPTAvgScore:      gptAvg,
		ClaudeAvgScore:   claudeAvg,
		AgreementPercent: agreementPct,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
