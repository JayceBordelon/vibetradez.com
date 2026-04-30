package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"sort"
	"time"
)

//go:embed email.html summary.html test.html error.html weekly.html execute_receipt.html execute_close_receipt.html execute_close_failed.html rollout_auto_execution_live.html rollout_claude_only.html
var templateFS embed.FS

type Trade struct {
	Symbol         string
	ContractType   string
	StrikePrice    float64
	Expiration     string
	DTE            int
	EstimatedPrice float64
	Thesis         string
	SentimentScore float64
	CurrentPrice   float64
	TargetPrice    float64
	StopLoss       float64
	RiskLevel      string
	Catalyst       string
	MentionCount   int
	Rank           int
	Score          int
	Rationale      string
}

/*
YesterdayRecap is a tiny digest of the previous trading day's results
surfaced at the top of the morning email so subscribers see how the
last batch performed before reading today's picks.
*/
type YesterdayRecap struct {
	Date        string
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
	Subject      string
	Date         string
	Trades       []Trade
	ModelName    string
	Yesterday    *YesterdayRecap
	TopPick      *Trade
	DashboardURL string
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
	Score          int
}

type SummaryEmailData struct {
	Subject     string
	Date        string
	Trades      []SummaryTrade
	TotalTrades int
	Winners     int
	Losers      int
	TotalPnL    float64
	Top3Pnl     float64
	AvgScore    float64
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

func RenderEmail(trades []Trade, modelName string, yesterday *YesterdayRecap) (string, error) {
	tmpl, err := template.New("email.html").Funcs(funcMap).ParseFS(templateFS, "email.html")
	if err != nil {
		return "", err
	}

	var topPick *Trade
	for i := range trades {
		t := &trades[i]
		if topPick == nil || t.Score > topPick.Score {
			topPick = t
		}
	}

	data := EmailData{
		Subject:      "Today's Top Options Plays",
		Date:         time.Now().Format("Monday, Jan 2, 2006"),
		Trades:       trades,
		ModelName:    modelName,
		Yesterday:    yesterday,
		TopPick:      topPick,
		DashboardURL: "https://vibetradez.com/dashboard",
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

type HealthCheck struct {
	Name    string
	Status  string
	Detail  string
	Latency string
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

// ── Auto-execution emails ──

type ExecuteReceiptData struct {
	Subject            string
	Date               string
	Mode               string
	Symbol             string
	ContractType       string
	StrikePrice        float64
	Expiration         string
	OCCSymbol          string
	FillPrice          float64
	Quantity           int
	OrderID            string
	SchwabPositionsURL string
}

type ExecuteCloseReceiptData struct {
	Subject            string
	Date               string
	Mode               string
	Symbol             string
	ContractType       string
	StrikePrice        float64
	Expiration         string
	OpenPrice          float64
	ClosePrice         float64
	RealizedPnL        float64
	SchwabPositionsURL string
}

type ExecuteCloseFailedData struct {
	Subject            string
	Date               string
	Symbol             string
	ContractType       string
	StrikePrice        float64
	Expiration         string
	OCCSymbol          string
	ErrorMessage       string
	SchwabPositionsURL string
}

func RenderExecuteReceipt(d ExecuteReceiptData) (string, error) {
	return renderOne("execute_receipt.html", d)
}
func RenderExecuteCloseReceipt(d ExecuteCloseReceiptData) (string, error) {
	return renderOne("execute_close_receipt.html", d)
}
func RenderExecuteCloseFailed(d ExecuteCloseFailedData) (string, error) {
	return renderOne("execute_close_failed.html", d)
}

/*
RenderRolloutAutoExecutionLive renders the v1 rollout email
announcing the auto-execution feature. Static content, no
parameters, just a Subject string for the <title> tag.
*/
func RenderRolloutAutoExecutionLive() (string, error) {
	return renderOne("rollout_auto_execution_live.html", map[string]string{
		"Subject": "VibeTradez can now execute trades",
	})
}

/*
RenderRolloutClaudeOnly renders the v2 rollout email announcing the
removal of the dual-model pipeline in favour of a single Claude-only
picker. Static content, no parameters beyond the Subject string.
*/
func RenderRolloutClaudeOnly() (string, error) {
	return renderOne("rollout_claude_only.html", map[string]string{
		"Subject": "We benched ChatGPT. Claude takes the floor.",
	})
}

func renderOne(name string, data any) (string, error) {
	tmpl, err := template.New(name).Funcs(funcMap).ParseFS(templateFS, name)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

/*
VerifyTemplates exercises all email templates with sample data to catch rendering errors.
Returns a HealthCheck for template rendering.
*/
func VerifyTemplates() HealthCheck {
	start := time.Now()

	sampleTrades := []Trade{
		{
			Symbol: "SPY", ContractType: "CALL", StrikePrice: 500,
			Expiration: "2026-04-01", DTE: 5, EstimatedPrice: 1.50,
			Thesis: "Startup verification", SentimentScore: 0.5,
			CurrentPrice: 498, TargetPrice: 505, StopLoss: 0.75,
			RiskLevel: "MEDIUM",
			Catalyst:  "System test", MentionCount: 42,
			Rank: 1, Score: 9,
			Rationale: "Sample bullish rationale.",
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
	if _, err := RenderEmail(sampleTrades, "Claude", sampleYesterday); err != nil {
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
		Subject: "Weekly Report", WeekRange: "Mar 25 to Mar 29, 2026",
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

/*
topNPnl picks the N highest-scoring summaries by the given score
selector and sums their per-contract P&L. Used by the EOD email to
backtest "what if you had only followed the top picks today".
*/
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
		return fmt.Sprintf("%dus", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}

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

/*
RenderErrorEmail renders an error notification email. Kept intentionally simple
(no loops, no comparisons) to minimize the chance of this template itself failing.
*/
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
		totalPnL += t.PriceChange * 100
	}

	top3Pnl := topNPnl(summaryTrades, 3, func(t SummaryTrade) float64 { return float64(t.Score) })

	var scoreSum float64
	var scoreCount int
	for _, t := range summaryTrades {
		if t.Score > 0 {
			scoreSum += float64(t.Score)
			scoreCount++
		}
	}
	var avgScore float64
	if scoreCount > 0 {
		avgScore = scoreSum / float64(scoreCount)
	}

	data := SummaryEmailData{
		Subject:     "End of Day Trade Summary",
		Date:        time.Now().Format("Monday, Jan 2, 2006"),
		Trades:      summaryTrades,
		TotalTrades: len(summaryTrades),
		Winners:     winners,
		Losers:      losers,
		TotalPnL:    totalPnL,
		Top3Pnl:     top3Pnl,
		AvgScore:    avgScore,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
