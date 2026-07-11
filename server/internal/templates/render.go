package templates

import (
	"bytes"
	"embed"
	"html/template"
	"strconv"
	"strings"
)

//go:embed schwab_reauth.html analysis_window.html live_trading_update.html options_only_update.html persona_update.html fable_return_update.html farewell_update.html
var templateFS embed.FS

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
// pages, that day's risk framing, readable transcripts). BaseURL is the
// public site root (no trailing slash); TranscriptURL points at the day's
// session.
type LiveTradingUpdateData struct {
	BaseURL       string
	TranscriptURL string
}

func RenderLiveTradingUpdate(d LiveTradingUpdateData) (string, error) {
	return renderOne("live_trading_update.html", d)
}

// ── One-time options-only product update ──

// OptionsOnlyUpdateData backs the one-time product-update email announcing
// the pivot to options-only trading: equity buys are disabled, any leftover
// stock is liquidated to cash, and single-contract / single-name sizing caps
// now apply. BaseURL is the public site root (no trailing slash);
// TranscriptURL points at the day's session.
type OptionsOnlyUpdateData struct {
	BaseURL       string
	TranscriptURL string
}

func RenderOptionsOnlyUpdate(d OptionsOnlyUpdateData) (string, error) {
	return renderOne("options_only_update.html", d)
}

// ── One-time persona-update product update ──

// PersonaUpdateData backs the one-time product-update email announcing the
// trading method: each session the agent researches a slate of ten candidate
// option plays, ranks them by realistic payoff, and trades only its top three
// (aggressive but grounded, diversified across names, within the same
// per-contract / per-underlying caps). BaseURL is the public site root (no
// trailing slash); TranscriptURL points at the day's session.
type PersonaUpdateData struct {
	BaseURL       string
	TranscriptURL string
}

func RenderPersonaUpdate(d PersonaUpdateData) (string, error) {
	return renderOne("persona_update.html", d)
}

// ── One-time Fable-return product update ──

// FableReturnWin is one recent profitable round trip shown in the win
// breakdown table of the Fable-return email.
type FableReturnWin struct {
	Label      string // human instrument label, e.g. "NVDA $190 calls"
	ClosedLong string // long-form close date, e.g. "June 27, 2026"
	HoldDays   int
	Pnl        float64 // realized P&L in dollars (positive)
	Pct        float64 // realized P&L percent
}

// FableReturnData backs the one-time product-update email announcing that
// the agent is back on Claude Fable 5 and reporting live performance: the
// current account value, the SPY buy-and-hold benchmark gap, how much of
// that gap has been recovered since its worst point, and a breakdown of
// recent winning round trips (with the honest net including losses).
// BaseURL is the public site root (no trailing slash).
type FableReturnData struct {
	BaseURL       string
	TranscriptURL string
	AsOf          string // long-form date the numbers were read
	AccountEquity float64
	Benchmark     float64 // SPY buy-and-hold on the same starting equity
	Gap           float64 // AccountEquity - Benchmark (negative = behind)
	GapClosed     float64 // improvement from the worst gap to now (>= 0)
	Behind        bool    // Gap < 0
	Wins          []FableReturnWin
	LossCount     int     // losing round trips in the same window
	LossTotal     float64 // sum of those losses (negative or zero)
	NetRealized   float64 // wins + losses over the window
	WindowLabel   string  // human window description, e.g. "the last two weeks"
}

func RenderFableReturnUpdate(d FableReturnData) (string, error) {
	return renderOne("fable_return_update.html", d)
}

// ── One-time farewell letter (decommission) ──

// FarewellData backs the one-time farewell email announcing the platform's
// decommission: the operator has taken a research-intensive role and can no
// longer run the service, so every position closes at the next Monday open
// and the account winds down. It carries the final honest accounting (start
// vs end equity, the SPY buy-and-hold gap, round-trip record) so the letter
// can joke about exactly how much money was lost. Abs* fields are the
// magnitudes of their signed counterparts, for "lost $X" phrasing where the
// usd helpers' sign prefix would read wrong. BaseURL is the public site root
// (no trailing slash).
type FarewellData struct {
	BaseURL       string
	AsOf          string  // long-form date the letter was sent
	StartDate     string  // long-form date of the first equity-curve point
	StartEquity   float64 // equity at the first curve point
	CurrentEquity float64 // equity at the latest curve point
	TotalPnl      float64 // CurrentEquity - StartEquity
	AbsTotalPnl   float64 // |TotalPnl|
	Lost          bool    // TotalPnl < 0
	Benchmark     float64 // SPY buy-and-hold on the same starting equity
	Gap           float64 // CurrentEquity - Benchmark
	AbsGap        float64 // |Gap|
	Behind        bool    // Gap < 0
	TradingDays   int     // equity-curve points with usable data
	Trips         int     // completed round trips, all time
	Wins          int     // trips with positive realized P&L
	Losses        int     // trips with negative realized P&L
}

func RenderFarewellUpdate(d FarewellData) (string, error) {
	return renderOne("farewell_update.html", d)
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
