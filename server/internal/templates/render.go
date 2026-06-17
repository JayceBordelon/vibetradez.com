package templates

import (
	"bytes"
	"embed"
	"html/template"
	"strconv"
	"strings"
)

//go:embed schwab_reauth.html analysis_window.html live_trading_update.html options_only_update.html
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
