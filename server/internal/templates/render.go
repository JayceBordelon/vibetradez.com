package templates

import (
	"bytes"
	"embed"
	"html/template"
	"strconv"
	"strings"
)

//go:embed portfolio_update.html schwab_reauth.html analysis_window.html live_trading_update.html
var templateFS embed.FS

// ── Daily portfolio-update email ──

// PortfolioMove is one row in the daily portfolio-update email: a buy,
// sell, or hold the model committed, with its rationale. IsBuy/IsSell/IsHold
// drive the badge color (html/template can't switch on a string cleanly).
type PortfolioMove struct {
	Action    string // "Buy" | "Sell" | "Hold"
	Label     string // "AAPL" or "MDT 79C 2026-04-17"; empty for a hold
	Notional  float64
	Rationale string
	IsBuy     bool
	IsSell    bool
	IsHold    bool
}

// PortfolioHolding is one row in the email's current-holdings table.
type PortfolioHolding struct {
	Label         string
	MarketValue   float64
	UnrealizedPnL float64
}

/*
PortfolioUpdateData backs the daily subscriber portfolio-update email. It
describes the single managed brokerage account: the day's stance, the moves
the model made with their reasons, and the current book.
*/
type PortfolioUpdateData struct {
	Date     string // raw YYYY-MM-DD (links)
	DateLong string // "June 4, 2026" (display)
	Equity   float64
	// DayChangePct is today's move vs the prior close; HasDayChange is false
	// when there isn't a prior curve point to compare against.
	DayChangePct  float64
	HasDayChange  bool
	SettledCash   float64
	InvestedPct   float64
	UnrealizedPnL float64 // total open unrealized across the book
	// Summary is today's synopsis (write_summary); ActionItems is the plan for
	// the next session, headed by ActionItemsLabel ("Tomorrow's…"/"Next week's…").
	// Stance is the shorter positioning note backing the synopsis.
	Summary          string
	ActionItems      string
	ActionItemsLabel string
	Stance           string
	Moves            []PortfolioMove
	Holdings         []PortfolioHolding
	TranscriptURL    string
	DashboardURL     string
}

// RenderPortfolioUpdate renders the daily subscriber portfolio-update email.
func RenderPortfolioUpdate(d PortfolioUpdateData) (string, error) {
	return renderOne("portfolio_update.html", d)
}

// ── Schwab re-auth operator nag ──

type SchwabReauthData struct {
	Subject       string
	Date          string
	IssuedAt      string
	ExpiresAt     string
	DaysOld       int
	DaysRemaining int
	ReauthURL     string
}

func RenderSchwabReauth(d SchwabReauthData) (string, error) {
	return renderOne("schwab_reauth.html", d)
}

// ── One-time analysis-window product update ──

// AnalysisWindowData backs the one-time product-update email announcing the
// longer daily session window (one hour, up from nine minutes), the
// fail-safe no-action stance, and the richer transcripts. BaseURL is the
// public site root (no trailing slash). TranscriptURL and TranscriptDateLong
// point at the day whose session hit the old limit and ended early.
type AnalysisWindowData struct {
	BaseURL            string
	TranscriptURL      string
	TranscriptDateLong string
}

func RenderAnalysisWindow(d AnalysisWindowData) (string, error) {
	return renderOne("analysis_window.html", d)
}

// ── One-time live-trading product update ──

// LiveTradingUpdateData backs the one-time product-update email announcing
// that trading runs indefinitely and walking through the day's full change
// list (real-time UI, P&L decomposition, executions ledger, per-trade
// pages, the two-rule cap sheet, readable transcripts). BaseURL is the
// public site root (no trailing slash); TranscriptURL points at the day's
// session.
type LiveTradingUpdateData struct {
	BaseURL       string
	TranscriptURL string
}

func RenderLiveTradingUpdate(d LiveTradingUpdateData) (string, error) {
	return renderOne("live_trading_update.html", d)
}

// emailFuncs gives templates locale-correct currency + percent helpers so
// emails match the site (thousands separators, signed P&L).
var emailFuncs = template.FuncMap{
	"usd":  func(v float64) string { return "$" + groupThousands(strconv.FormatFloat(absf(v), 'f', 2, 64), v < 0) },
	"usd0": func(v float64) string { return "$" + groupThousands(strconv.FormatFloat(absf(v), 'f', 0, 64), v < 0) },
	"pnl": func(v float64) string {
		body := "$" + groupThousands(strconv.FormatFloat(absf(v), 'f', 2, 64), false)
		if v > 0 {
			return "+" + body
		}
		if v < 0 {
			return "-" + body
		}
		return body
	},
	"pct1": func(v float64) string {
		s := strconv.FormatFloat(v, 'f', 1, 64)
		if v > 0 {
			return "+" + s + "%"
		}
		return s + "%"
	},
	"pct0": func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) + "%" },
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// groupThousands inserts comma separators into the integer part of a
// already-formatted decimal string ("1480.42" -> "1,480.42").
func groupThousands(s string, neg bool) string {
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i:]
	}
	n := len(intPart)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, c := range intPart {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	b.WriteString(frac)
	return b.String()
}

func renderOne(name string, data any) (string, error) {
	tmpl, err := template.New(name).Funcs(emailFuncs).ParseFS(templateFS, name)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
