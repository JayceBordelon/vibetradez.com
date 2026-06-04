package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"vibetradez.com/internal/config"
	"vibetradez.com/internal/email"
	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/portfolio"
	"vibetradez.com/internal/portfoliowire"
	"vibetradez.com/internal/store"
	"vibetradez.com/internal/templates"
)

/*
runPortfolioSession is the daily portfolio-agent cron body (~9:45 ET). It
runs one decision session, then persists the committed moves, the holds,
and the overall stance. Moves are fire-and-forget LIMIT orders (the broker
fills asynchronously); we record them as 'submitted'. Persistence is here,
not in the executor adapter, because the dispatcher's recorded decisions
carry the rationale the executor never sees.
*/
func runPortfolioSession(db *store.Store, agent *portfolio.Agent) {
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
	defer cancel()

	date := todayDate()
	result, err := agent.Run(ctx)
	if err != nil {
		log.Printf("portfolio: session error: %v", err)
		// Still fall through: a partial result may carry moves that did
		// reach the broker before the failure.
	}
	if result == nil {
		return
	}

	if serr := db.SavePortfolioSession(date, "live", result.Stance, result.Summary, result.ActionItems); serr != nil {
		log.Printf("portfolio: persist session stance: %v", serr)
	}
	for _, d := range result.Decisions {
		if _, serr := db.SavePortfolioDecision(decisionRow(date, "agent", "live", d)); serr != nil {
			log.Printf("portfolio: persist decision (%s %s): %v", d.Action, d.Symbol, serr)
		}
	}
	log.Printf("portfolio: session complete (%d decisions, mode=live)", len(result.Decisions))

	// Persist the day's reasoning + tool-call transcript so it's viewable
	// at /transcript/<date>/portfolio. It is captured verbatim (this is one
	// public account, so balances are shown, not redacted).
	// The single subscriber recap goes out at the EOD snapshot cron (16:00),
	// not here, so it reflects the full day and the closing book.
	saveTranscript(db, date, "portfolio", result.Transcript)
}

/*
sendDailyRecapEmail builds and sends the single end-of-day recap to
subscribers at the 16:00 close. It pulls the whole day from the record:
Claude's session summary + stance, every move it committed with the reason,
the closing book and headline numbers (equity, day change, invested, cash,
unrealized), and a link to the full tool-by-tool transcript on the site.
Subscribers are watchers, so this is informational. Best-effort: a render
or send failure is logged, never fatal. snap is the closing snapshot the
EOD cron already fetched (so we don't hit the broker twice).
*/
func sendDailyRecapEmail(cfg *config.Config, db *store.Store, emailClient *email.Client, date string, snap portfolio.Snapshot, dayChangePct float64, hasDayChange bool) {
	recipients := getRecipients(db)
	if len(recipients) == 0 {
		return
	}

	_, stance, summary, actionItems, _, _ := db.GetPortfolioSession(date)
	decisions, derr := db.GetPortfolioDecisions(date)
	if derr != nil {
		log.Printf("portfolio recap: read decisions: %v", derr)
	}

	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	data := templates.PortfolioUpdateData{
		Date:             date,
		DateLong:         longDate(date),
		Equity:           snap.Equity,
		DayChangePct:     dayChangePct,
		HasDayChange:     hasDayChange,
		SettledCash:      snap.SettledCash,
		Summary:          strings.TrimSpace(summary),
		Stance:           strings.TrimSpace(stance),
		ActionItems:      strings.TrimSpace(actionItems),
		ActionItemsLabel: actionItemsLabel(date),
		DrawdownHalted:   portfolio.DefaultCaps().DrawdownHalted(snap),
		Moves:            movesFromRows(decisions),
		Holdings:         holdingsForEmail(snap.Positions),
		TranscriptURL:    base + "/transcripts/" + date,
		DashboardURL:     base + "/dashboard",
	}
	if snap.Equity > 0 {
		invested := snap.Equity - snap.SettledCash - snap.UnsettledCash
		data.InvestedPct = invested / snap.Equity * 100
	}
	for _, p := range snap.Positions {
		data.UnrealizedPnL += p.MarkValue - p.CostBasis
	}

	html, err := templates.RenderPortfolioUpdate(data)
	if err != nil {
		log.Printf("portfolio recap: render email: %v", err)
		return
	}
	subject := fmt.Sprintf("VibeTradez daily recap: %s", data.DateLong)
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, html, unsubURLBuilder(cfg))
	log.Printf("portfolio recap: email sent to %d/%d subscribers", res.Succeeded, res.Total)
	if res.Failed > 0 {
		log.Printf("portfolio recap: email failures: %s", res.FailureDetail())
	}
}

// launchAnnouncementKey is the sent_emails ledger key for the one-time v2
// launch blast. Bump the suffix only if you ever intend a deliberate re-send.
const launchAnnouncementKey = "launch_announcement_v2"

/*
sendLaunchAnnouncement sends the one-time "letting Claude drive" email
announcing the v2 autonomous portfolio manager to the whole subscriber list.
It is fired by SEND_ANNOUNCEMENT=true on boot, but is idempotent: it records
its key in the sent_emails ledger after a successful send, so it goes out
exactly once and never re-blasts on a later boot even if the flag stays set.
Best-effort: a render or send failure is logged, never fatal.
*/
func sendLaunchAnnouncement(cfg *config.Config, db *store.Store, emailClient *email.Client) {
	if sent, err := db.EmailAlreadySent(launchAnnouncementKey); err != nil {
		log.Printf("announcement: could not read send ledger, skipping to be safe: %v", err)
		return
	} else if sent {
		log.Printf("announcement: already sent (key=%s), nothing to do", launchAnnouncementKey)
		return
	}

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		log.Printf("announcement: no subscribers, nothing to send")
		return
	}

	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	data := templates.AnnouncementData{BaseURL: base}
	// State the account's value as of the most recent market close (the latest
	// equity-curve point the EOD snapshot recorded).
	if pts, perr := db.GetEquityCurve("0000-01-01", "9999-12-31"); perr == nil && len(pts) > 0 {
		last := pts[len(pts)-1]
		data.StartingBalance = last.AccountEquity
		data.BalanceAsOf = longDate(last.Date)
	}
	html, err := templates.RenderAnnouncement(data)
	if err != nil {
		log.Printf("announcement: render email: %v", err)
		return
	}
	subject := "Fuck it. I'm letting Claude drive."
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, html, unsubURLBuilder(cfg))
	log.Printf("announcement: email sent to %d/%d subscribers", res.Succeeded, res.Total)
	if res.Failed > 0 {
		log.Printf("announcement: email failures: %s", res.FailureDetail())
	}
	// Record the send so it never goes out again, even if SEND_ANNOUNCEMENT
	// stays set across restarts. Only claim the key once at least one
	// recipient actually received it.
	if res.Succeeded > 0 {
		if err := db.MarkEmailSent(launchAnnouncementKey); err != nil {
			log.Printf("announcement: WARNING sent but failed to record in ledger (could re-send on next boot): %v", err)
		}
	}
}

// longDate turns a YYYY-MM-DD date into "June 4, 2026" for the email; falls
// back to the raw string if it doesn't parse.
func longDate(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.Format("January 2, 2006")
}

// actionItemsLabel frames the next-session plan: after a Friday (or weekend)
// the next session is next week, otherwise it's tomorrow.
func actionItemsLabel(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "Action items for next session"
	}
	switch t.Weekday() {
	case time.Friday, time.Saturday, time.Sunday:
		return "Next week's action items"
	default:
		return "Tomorrow's action items"
	}
}

// movesFromRows maps persisted decision rows to the email's move list.
func movesFromRows(rows []store.PortfolioDecisionRow) []templates.PortfolioMove {
	out := make([]templates.PortfolioMove, 0, len(rows))
	for _, d := range rows {
		m := templates.PortfolioMove{Notional: d.Notional, Rationale: d.Rationale}
		at := portfolio.AssetType(d.AssetType)
		switch d.Action {
		case "buy_equity", "buy_option":
			m.Action = "Buy"
			m.IsBuy = true
			m.Label = instrumentLabel(at, d.Symbol, d.Underlying)
		case "sell_equity", "sell_option":
			m.Action = "Sell"
			m.IsSell = true
			m.Label = instrumentLabel(at, d.Symbol, d.Underlying)
		default:
			m.Action = "Hold"
			m.IsHold = true
			if d.Symbol != "" {
				m.Label = instrumentLabel(at, d.Symbol, d.Underlying)
			}
		}
		out = append(out, m)
	}
	return out
}

func holdingsForEmail(positions []portfolio.Position) []templates.PortfolioHolding {
	out := make([]templates.PortfolioHolding, 0, len(positions))
	for _, p := range positions {
		label := p.Symbol
		if p.AssetType == portfolio.AssetOption {
			label = optionLabel(p.Underlying, p.ContractType, p.Strike, p.Expiration)
		}
		out = append(out, templates.PortfolioHolding{
			Label:         label,
			MarketValue:   p.MarkValue,
			UnrealizedPnL: p.MarkValue - p.CostBasis,
		})
	}
	return out
}

// instrumentLabel renders a friendly position label. For options it decodes
// the OCC symbol to "UNDERLYING STRIKE C/P EXP"; for equity it's the ticker.
func instrumentLabel(assetType portfolio.AssetType, symbol, underlying string) string {
	if assetType == portfolio.AssetOption {
		if _, exp, ctype, strike, err := exec.DecodeOCCSymbol(symbol); err == nil {
			return optionLabel(underlying, ctype, strike, exp)
		}
	}
	return symbol
}

func optionLabel(underlying, contractType string, strike float64, expiration string) string {
	letter := "C"
	if contractType == "PUT" {
		letter = "P"
	}
	return fmt.Sprintf("%s %g%s %s", underlying, strike, letter, expiration)
}

/*
runPortfolioRisk is the intraday risk-cron body (every 15 min, market
hours). It re-reads the live snapshot and surfaces two conditions: the
drawdown breaker (new buys halted below the high-water floor) and held
options under the DTE floor (which the expiry rule should close or roll).

Scaffold scope: this DETECTS and logs. Wiring the actual de-risking sells
(close/roll the flagged options, force-flatten on a deep breaker trip) is a
follow-up so the auto-sell path gets its own review before it can move real
positions. The detection here is the safe half.
*/
func runPortfolioRisk(reader *portfoliowire.Reader, caps portfolio.Caps) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	snap, err := reader.Snapshot(ctx)
	if err != nil {
		log.Printf("portfolio risk: snapshot error: %v", err)
		return
	}
	if caps.DrawdownHalted(snap) {
		log.Printf("portfolio risk: DRAWDOWN BREAKER tripped (equity $%.2f vs high-water $%.2f, halt at -%.0f%%). New buys are blocked at the tool layer; de-risking still allowed.",
			snap.Equity, snap.HighWaterMark, caps.DrawdownHaltPct*100)
	}
	for _, p := range caps.HeldOptionsBelowDTE(snap) {
		log.Printf("portfolio risk: held option %s is at %d DTE (under the %d-day floor); expiry rule should close or roll it. [auto-close not yet wired]",
			p.Symbol, p.DTE, caps.MinHeldOptionDTE)
	}
}

/*
runPortfolioEODSnapshot is the 16:00 ET cron body. It records one
equity-curve point for the day: account equity, cash, the running
high-water mark, and the SPY close for the benchmark. Upserted by date, so
a re-run overwrites cleanly.
*/
func runPortfolioEODSnapshot(cfg *config.Config, db *store.Store, emailClient *email.Client, reader *portfoliowire.Reader) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	snap, err := reader.Snapshot(ctx)
	if err != nil {
		log.Printf("portfolio EOD snapshot: snapshot error: %v", err)
		return
	}
	hwm := snap.HighWaterMark
	if snap.Equity > hwm {
		hwm = snap.Equity
	}
	spyClose := fetchSPYClose(reader)
	point := store.EquityCurvePoint{
		Date:          todayDate(),
		AccountEquity: snap.Equity,
		SettledCash:   snap.SettledCash,
		UnsettledCash: snap.UnsettledCash,
		HighWaterMark: hwm,
		SPYClose:      spyClose,
	}
	if err := db.SaveEquityCurvePoint(point); err != nil {
		log.Printf("portfolio EOD snapshot: persist equity curve: %v", err)
		return
	}

	// Persist the held book so the dashboard can fall back to it when the
	// live broker has no positions (broker down or unreachable).
	posRows := make([]store.PortfolioPositionRow, 0, len(snap.Positions))
	for _, p := range snap.Positions {
		var strike *float64
		if p.AssetType == portfolio.AssetOption {
			s := p.Strike
			strike = &s
		}
		posRows = append(posRows, store.PortfolioPositionRow{
			Date:         todayDate(),
			Symbol:       p.Symbol,
			Underlying:   p.Underlying,
			AssetType:    string(p.AssetType),
			ContractType: p.ContractType,
			Strike:       strike,
			Expiration:   p.Expiration,
			Quantity:     p.Quantity,
			MarketValue:  p.MarkValue,
			CostBasis:    p.CostBasis,
		})
	}
	if err := db.SavePositionsSnapshot(todayDate(), posRows); err != nil {
		log.Printf("portfolio EOD snapshot: persist positions: %v", err)
	}
	log.Printf("portfolio EOD snapshot: equity $%.2f, high-water $%.2f, SPY %.2f, %d positions", snap.Equity, hwm, spyClose, len(posRows))

	// Day change vs the prior close (the curve now ends with today's point).
	dayChangePct, hasDayChange := 0.0, false
	if pts, err := db.GetEquityCurve("0000-00-00", todayDate()); err == nil && len(pts) >= 2 {
		prev := pts[len(pts)-2].AccountEquity
		if prev > 0 {
			dayChangePct = (snap.Equity - prev) / prev * 100
			hasDayChange = true
		}
	}

	// The single subscriber recap for the day goes out here, at the close.
	sendDailyRecapEmail(cfg, db, emailClient, todayDate(), snap, dayChangePct, hasDayChange)
}

// fetchSPYClose reads SPY's latest/close mark for the benchmark column.
// Returns 0 on error (the curve point still persists; a 0 SPY just leaves a
// gap in the benchmark line for that day).
func fetchSPYClose(reader *portfoliowire.Reader) float64 {
	quotes, err := reader.GetQuotes([]string{"SPY"})
	if err != nil {
		log.Printf("portfolio EOD snapshot: SPY quote error: %v", err)
		return 0
	}
	q := quotes["SPY"]
	if q.LastPrice > 0 {
		return q.LastPrice
	}
	return q.ClosePrice
}

/*
decisionRow maps a portfolio.Decision (the in-process record the agent's
tool layer produced) to the store row. For option moves it decodes the OCC
symbol to recover the strike / expiration / contract type the Decision
doesn't carry. Buys/sells are recorded as 'submitted' (fire-and-forget
LIMITs); holds carry only their rationale.
*/
func decisionRow(date, source, mode string, d portfolio.Decision) store.PortfolioDecisionRow {
	row := store.PortfolioDecisionRow{
		Date:          date,
		Source:        source,
		Action:        string(d.Action),
		AssetType:     string(d.AssetType),
		Symbol:        d.Symbol,
		Underlying:    d.Underlying,
		Quantity:      d.Quantity,
		LimitPrice:    d.LimitPrice,
		Notional:      d.Notional,
		Mode:          mode,
		SchwabOrderID: d.OrderID,
		ExecutionID:   d.ExecutionID,
		Rationale:     d.Rationale,
	}
	if d.Action == portfolio.ActionHold {
		return row
	}
	row.Status = "submitted"
	if d.AssetType == portfolio.AssetOption {
		if _, exp, ctype, strike, err := exec.DecodeOCCSymbol(d.Symbol); err == nil {
			row.ContractType = ctype
			row.Expiration = exp
			row.Strike = &strike
		}
	}
	return row
}
