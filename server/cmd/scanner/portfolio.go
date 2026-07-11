package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"vibetradez.com/internal/config"
	"vibetradez.com/internal/email"
	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/portfolio"
	"vibetradez.com/internal/portfoliowire"
	"vibetradez.com/internal/store"
	"vibetradez.com/internal/templates"
	"vibetradez.com/internal/trades"
)

/*
runPortfolioSession is one portfolio-agent cron body, fired once per slot
(open ~9:45, midday ~12:30, pre-close ~15:30 ET). It runs one decision
session, then persists the committed moves, the holds,
and the overall stance. Moves are fire-and-forget LIMIT orders (the broker
fills asynchronously); we record them as 'submitted'. Persistence is here,
not in the executor adapter, because the dispatcher's recorded decisions
carry the rationale the executor never sees.
*/
func runPortfolioSession(db *store.Store, agent *portfolio.Agent, slot string) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	date := todayDate()
	result, err := agent.Run(ctx, slot)
	if err != nil {
		log.Printf("portfolio: session error: %v", err)
		// Still fall through: a partial result may carry moves that did
		// reach the broker before the failure.
	}
	if result == nil {
		return
	}

	// Fail safe. A session that errors out (deadline, API failure, malformed
	// response) before the agent commits anything would otherwise persist a
	// blank day: empty stance, no decisions, nothing on the dashboard or in
	// the recap. Record a clear no-action result instead, so the failure is
	// visible rather than silent. Any moves that already reached the broker
	// are kept (the guard requires zero decisions), and a real stance the
	// agent did produce is never overwritten.
	if err != nil && result.Stance == "" && len(result.Decisions) == 0 {
		result.Stance = "Session did not complete. No trades were placed."
		if result.Summary == "" {
			result.Summary = "This session ended before a decision was reached, so the book is unchanged and no orders were submitted in it."
		}
	}

	if serr := db.SavePortfolioSession(date, "live", result.Stance, result.Summary, result.ActionItems); serr != nil {
		log.Printf("portfolio: persist session stance: %v", serr)
	}
	for _, d := range result.Decisions {
		if _, serr := db.SavePortfolioDecision(decisionRow(date, "agent", "live", d)); serr != nil {
			log.Printf("portfolio: persist decision (%s %s): %v", d.Action, d.Symbol, serr)
		}
	}
	if err != nil {
		// The session aborted (deadline, API failure, malformed response). Don't
		// log "complete" — that reads as a clean no-trade day and is exactly what
		// masked the 2026-06-18 container_id crash in the logs. The underlying
		// error is already logged above; this terminal line must reflect failure.
		log.Printf("portfolio: session ENDED WITH ERROR (%d decisions persisted before failure, mode=live)", len(result.Decisions))
	} else {
		log.Printf("portfolio: session complete (%d decisions, mode=live)", len(result.Decisions))
	}

	// Persist the day's reasoning + tool-call transcript so it's viewable
	// at /transcript/<date>/portfolio. It is captured verbatim (this is one
	// public account, so balances are shown, not redacted). The recap email is
	// authored and sent by the model itself (send_recap_email) at the end of a
	// session it traded, not from here.
	saveSessionTranscript(db, date, slot, result.Transcript)
}

/*
recapEmailSender implements portfolio.RecapSender. The model now authors the
recap email body itself (via the send_recap_email tool) and this delivers it
to every subscriber — replacing the old templated end-of-day recap. Claude
writes one per session, but only when it actually CLOSED something (a sell —
buy-only and hold-only sessions email nobody) and at most once per session
(both gated in the tool layer). Best-effort and per-recipient (so one bad
address can't sink the batch); a per-recipient unsubscribe link is
guaranteed even if the model forgot it.
*/
type recapEmailSender struct {
	cfg    *config.Config
	db     *store.Store
	client *email.Client
}

func (s *recapEmailSender) SendRecapEmail(_ context.Context, subject, html string) (int, error) {
	recipients := getRecipients(s.db)
	if len(recipients) == 0 {
		log.Printf("portfolio recap: no subscribers, nothing to send")
		return 0, nil
	}
	body := signByClaudia(ensureUnsubscribe(html), s.cfg.AnthropicModel)
	res := s.client.SendPersonalizedToList(s.cfg.EmailFrom, recipients, subject, body, unsubURLBuilder(s.cfg))
	log.Printf("portfolio recap: model-authored email sent to %d/%d subscribers", res.Succeeded, res.Total)
	if res.Failed > 0 {
		log.Printf("portfolio recap: email failures: %s", res.FailureDetail())
	}
	if res.Succeeded == 0 && res.Failed > 0 {
		return 0, fmt.Errorf("all %d recipient sends failed", res.Failed)
	}
	return res.Succeeded, nil
}

// ensureUnsubscribe guarantees the per-recipient unsubscribe placeholder is in
// the model-authored HTML (bulk-email compliance): if the model left it out,
// append a minimal footer carrying it just before </body>.
func ensureUnsubscribe(html string) string {
	if strings.Contains(html, "@@VT_UNSUBSCRIBE_URL@@") {
		return html
	}
	footer := `<p style="margin:16px 0 0;font-size:11px;color:#94a3b8;">VibeTradez is one real account run by an AI. Not financial advice. <a href="@@VT_UNSUBSCRIBE_URL@@" style="color:#94a3b8;text-decoration:underline;">Unsubscribe</a></p>`
	if i := strings.LastIndex(strings.ToLower(html), "</body>"); i >= 0 {
		return html[:i] + footer + html[i:]
	}
	return html + footer
}

/*
signByClaudia closes every model-authored trade email — the recaps sent on
closes and position changes — with Claudia's sign-off, plus the configured
model as small-print attribution (authoritative, not the model's own guess).
The prompt forbids the model from signing its own emails ("Do NOT sign with
a name or model version, the system stamps that" in prompt.md), so this
stamp is the single code-enforced signature: every position-change email
goes out signed by Claudia even when the model string is empty. Inserted
just before </body>.
*/
func signByClaudia(html, model string) string {
	attribution := "your autonomous portfolio manager"
	if model = strings.TrimSpace(model); model != "" {
		attribution += " &middot; written by " + model
	}
	sig := `<p style="margin:18px 0 0;font-size:14px;font-weight:700;color:#0f172a;">&mdash; Claudia</p>` +
		`<p style="margin:2px 0 0;font-size:11px;color:#94a3b8;">` + attribution + `</p>`
	if i := strings.LastIndex(strings.ToLower(html), "</body>"); i >= 0 {
		return html[:i] + sig + html[i:]
	}
	return html + sig
}

// analysisWindowKey is the sent_emails ledger key for the one-time product
// update announcing the longer session window. It is deliberately distinct
// from the retired launch_announcement_v2 key so the lingering launch row
// (left behind, never dropped) cannot gate this send. Bump the suffix only
// for a deliberate re-send.
const analysisWindowKey = "analysis_window_update_v1"

// liveTradingUpdateKey is the sent_emails ledger key for the one-time
// product update announcing indefinite live trading and the 2026-06-09
// change set (real-time UI, P&L decomposition, executions ledger,
// per-trade pages, and that day's risk framing).
const liveTradingUpdateKey = "live_trading_update_2026_06_09"

// optionsOnlyUpdateKey is the sent_emails ledger key for the one-time product
// update announcing the pivot to options-only trading: equity buys disabled,
// any leftover stock liquidated to cash, and per-contract / per-underlying
// sizing caps now in force. Self-gating like the other one-time updates.
const optionsOnlyUpdateKey = "options_only_update_v1"

// personaUpdateKey is the sent_emails ledger key for the one-time product
// update announcing the trading method: each session the agent researches a
// slate of ten candidate option plays, ranks them, and trades only its top
// three (aggressive but grounded, diversified across names, within the existing
// caps). Self-gating like the other one-time updates.
//
// Bumped to v2 on purpose: v1 already fired an earlier, since-superseded draft
// of this announcement, so this is a deliberate fresh send. The copy openly
// disowns the previous email, which is why a second send to the same list is
// the intended behavior, not a mistake.
const personaUpdateKey = "persona_update_v2"

/*
sendAnalysisWindowUpdate sends the one-time product-update email announcing
the longer daily session window (one hour, up from nine minutes), the
fail-safe no-action stance, and the richer transcripts. It links the current
day's transcript, whose session hit the old limit and ended early.

It runs on every boot but is self-gating via the sent_emails ledger: it
records its key after a successful send, so it goes out exactly once (the
first boot with subscribers) and never re-blasts on a later boot. Best-effort:
a render or send failure is logged, never fatal.
*/
func sendAnalysisWindowUpdate(cfg *config.Config, db *store.Store, emailClient *email.Client) {
	if sent, err := db.EmailAlreadySent(analysisWindowKey); err != nil {
		log.Printf("analysis-window update: could not read send ledger, skipping to be safe: %v", err)
		return
	} else if sent {
		log.Printf("analysis-window update: already sent (key=%s), nothing to do", analysisWindowKey)
		return
	}

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		log.Printf("analysis-window update: no subscribers, nothing to send")
		return
	}

	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	date := todayDate()
	data := templates.AnalysisWindowData{
		BaseURL:            base,
		TranscriptURL:      base + "/transcripts/" + date,
		TranscriptDateLong: longDate(date),
	}
	html, err := templates.RenderAnalysisWindow(data)
	if err != nil {
		log.Printf("analysis-window update: render email: %v", err)
		return
	}
	subject := "VibeTradez update: a longer analysis window"
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, html, unsubURLBuilder(cfg))
	log.Printf("analysis-window update: email sent to %d/%d subscribers", res.Succeeded, res.Total)
	if res.Failed > 0 {
		log.Printf("analysis-window update: email failures: %s", res.FailureDetail())
	}
	// Record the send so it never goes out again on a later boot. Only claim
	// the key once at least one recipient actually received it.
	if res.Succeeded > 0 {
		if err := db.MarkEmailSent(analysisWindowKey); err != nil {
			log.Printf("analysis-window update: WARNING sent but failed to record in ledger (could re-send on next boot): %v", err)
		}
	}
}

/*
sendLiveTradingUpdate sends the one-time product update announcing that
trading now runs indefinitely, with the full 2026-06-09 change list and
that day's risk framing. Same self-gating
shape as sendAnalysisWindowUpdate: runs on every boot, claims its
sent_emails key only after at least one recipient received it, and never
re-blasts. Best-effort: a render or send failure is logged, never fatal.
*/
func sendLiveTradingUpdate(cfg *config.Config, db *store.Store, emailClient *email.Client) {
	if sent, err := db.EmailAlreadySent(liveTradingUpdateKey); err != nil {
		log.Printf("live-trading update: could not read send ledger, skipping to be safe: %v", err)
		return
	} else if sent {
		log.Printf("live-trading update: already sent (key=%s), nothing to do", liveTradingUpdateKey)
		return
	}

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		log.Printf("live-trading update: no subscribers, nothing to send")
		return
	}

	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	data := templates.LiveTradingUpdateData{
		BaseURL:       base,
		TranscriptURL: base + "/transcripts/" + todayDate(),
	}
	html, err := templates.RenderLiveTradingUpdate(data)
	if err != nil {
		log.Printf("live-trading update: render email: %v", err)
		return
	}
	subject := "VibeTradez update: live trading, live everything"
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, html, unsubURLBuilder(cfg))
	log.Printf("live-trading update: email sent to %d/%d subscribers", res.Succeeded, res.Total)
	if res.Failed > 0 {
		log.Printf("live-trading update: email failures: %s", res.FailureDetail())
	}
	if res.Succeeded > 0 {
		if err := db.MarkEmailSent(liveTradingUpdateKey); err != nil {
			log.Printf("live-trading update: WARNING sent but failed to record in ledger (could re-send on next boot): %v", err)
		}
	}
}

/*
sendOptionsOnlyUpdate sends the one-time product update announcing the pivot
to options-only trading: equity buys are disabled, any leftover stock is
liquidated to cash, and single-contract / single-name sizing caps now apply.
Same self-gating shape as the other one-time updates: runs on every boot, claims
its sent_emails key only after at least one recipient received it, and never
re-blasts. Best-effort: a render or send failure is logged, never fatal.
*/
func sendOptionsOnlyUpdate(cfg *config.Config, db *store.Store, emailClient *email.Client) {
	if sent, err := db.EmailAlreadySent(optionsOnlyUpdateKey); err != nil {
		log.Printf("options-only update: could not read send ledger, skipping to be safe: %v", err)
		return
	} else if sent {
		log.Printf("options-only update: already sent (key=%s), nothing to do", optionsOnlyUpdateKey)
		return
	}

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		log.Printf("options-only update: no subscribers, nothing to send")
		return
	}

	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	data := templates.OptionsOnlyUpdateData{
		BaseURL:       base,
		TranscriptURL: base + "/transcripts/" + todayDate(),
	}
	html, err := templates.RenderOptionsOnlyUpdate(data)
	if err != nil {
		log.Printf("options-only update: render email: %v", err)
		return
	}
	subject := "VibeTradez update: options only (the stocks were boring)"
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, html, unsubURLBuilder(cfg))
	log.Printf("options-only update: email sent to %d/%d subscribers", res.Succeeded, res.Total)
	if res.Failed > 0 {
		log.Printf("options-only update: email failures: %s", res.FailureDetail())
	}
	if res.Succeeded > 0 {
		if err := db.MarkEmailSent(optionsOnlyUpdateKey); err != nil {
			log.Printf("options-only update: WARNING sent but failed to record in ledger (could re-send on next boot): %v", err)
		}
	}
}

/*
sendPersonaUpdate sends the one-time product update announcing the trading
method: each session the agent researches a slate of ten candidate option
plays, ranks them by realistic payoff, and trades only its top three
(aggressive but grounded in tool data, diversified across names, within the
same per-contract / per-underlying caps). Same self-gating shape as the other
one-time updates: runs on every boot, claims its sent_emails key only after at
least one recipient received it, and never re-blasts. Best-effort: a render or
send failure is logged, never fatal.
*/
func sendPersonaUpdate(cfg *config.Config, db *store.Store, emailClient *email.Client) {
	if sent, err := db.EmailAlreadySent(personaUpdateKey); err != nil {
		log.Printf("persona update: could not read send ledger, skipping to be safe: %v", err)
		return
	} else if sent {
		log.Printf("persona update: already sent (key=%s), nothing to do", personaUpdateKey)
		return
	}

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		log.Printf("persona update: no subscribers, nothing to send")
		return
	}

	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	data := templates.PersonaUpdateData{
		BaseURL:       base,
		TranscriptURL: base + "/transcripts/" + todayDate(),
	}
	html, err := templates.RenderPersonaUpdate(data)
	if err != nil {
		log.Printf("persona update: render email: %v", err)
		return
	}
	subject := "Forget my last email"
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, html, unsubURLBuilder(cfg))
	log.Printf("persona update: email sent to %d/%d subscribers", res.Succeeded, res.Total)
	if res.Failed > 0 {
		log.Printf("persona update: email failures: %s", res.FailureDetail())
	}
	if res.Succeeded > 0 {
		if err := db.MarkEmailSent(personaUpdateKey); err != nil {
			log.Printf("persona update: WARNING sent but failed to record in ledger (could re-send on next boot): %v", err)
		}
	}
}

// fableReturnUpdateKey is the sent_emails ledger key for the one-time product
// update announcing that the agent is back on Claude Fable 5 (after the
// mid-June retirement forced a temporary downgrade to Opus 4.8), with live
// performance numbers: the SPY gap, how much of it has been recovered, and
// a breakdown of recent winning closes. Self-gating like the other one-time
// updates.
const fableReturnUpdateKey = "fable_return_update_v1"

/*
sendFableReturnUpdate sends the one-time product update announcing the return
to Claude Fable 5. Unlike the other one-time updates it carries live numbers
(account equity, the SPY buy-and-hold gap, recent round-trip wins), so it has
an extra gate: when the store doesn't yet have enough curve/trade history to
render honest numbers, the send is deferred WITHOUT claiming the ledger key,
and it retries on a later boot. Otherwise the same self-gating shape as
sendPersonaUpdate: claims its sent_emails key only after at least one
recipient received it, and never re-blasts. Best-effort: a render or send
failure is logged, never fatal.
*/
func sendFableReturnUpdate(cfg *config.Config, db *store.Store, emailClient *email.Client) {
	if sent, err := db.EmailAlreadySent(fableReturnUpdateKey); err != nil {
		log.Printf("fable-return update: could not read send ledger, skipping to be safe: %v", err)
		return
	} else if sent {
		log.Printf("fable-return update: already sent (key=%s), nothing to do", fableReturnUpdateKey)
		return
	}

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		log.Printf("fable-return update: no subscribers, nothing to send")
		return
	}

	data, ok := buildFableReturnData(cfg, db)
	if !ok {
		log.Printf("fable-return update: not enough performance data yet, deferring send to a later boot")
		return
	}

	html, err := templates.RenderFableReturnUpdate(data)
	if err != nil {
		log.Printf("fable-return update: render email: %v", err)
		return
	}
	subject := "VibeTradez update: my brain is back (the money is following)"
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, html, unsubURLBuilder(cfg))
	log.Printf("fable-return update: email sent to %d/%d subscribers", res.Succeeded, res.Total)
	if res.Failed > 0 {
		log.Printf("fable-return update: email failures: %s", res.FailureDetail())
	}
	if res.Succeeded > 0 {
		if err := db.MarkEmailSent(fableReturnUpdateKey); err != nil {
			log.Printf("fable-return update: WARNING sent but failed to record in ledger (could re-send on next boot): %v", err)
		}
	}
}

/*
buildFableReturnData assembles the live numbers for the Fable-return email.
Returns ok=false when the equity curve can't support an honest gap statement
(no points with both an equity value and a SPY close), so the caller defers.

The benchmark is the same normalization the dashboard chart uses: the first
curve point's equity riding SPY closes (equity₀ × SPY_t / SPY₀). The gap is
account equity minus that benchmark; "gap recovered" is the improvement from
the widest deficit to today. Wins are recent completed round trips from the
shared trades derivation — the same source the /closed page and the agent's
get_track_record read — with losses netted into one honest line.
*/
func buildFableReturnData(cfg *config.Config, db *store.Store) (templates.FableReturnData, bool) {
	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	today := todayDate()
	data := templates.FableReturnData{
		BaseURL:       base,
		TranscriptURL: base + "/transcripts/" + today,
		AsOf:          longDate(today),
		WindowLabel:   "the last two weeks",
	}

	pts, err := db.GetEquityCurve("0000-00-00", today)
	if err != nil {
		return data, false
	}
	var (
		haveBase             bool
		baseEquity, baseSPY  float64
		gap, bench, worstGap float64
		havePoint            bool
	)
	for _, p := range pts {
		if p.SPYClose <= 0 || p.AccountEquity <= 0 {
			continue
		}
		if !haveBase {
			baseEquity, baseSPY = p.AccountEquity, p.SPYClose
			haveBase = true
		}
		bench = baseEquity * p.SPYClose / baseSPY
		gap = p.AccountEquity - bench
		if !havePoint || gap < worstGap {
			worstGap = gap
		}
		data.AccountEquity = p.AccountEquity
		havePoint = true
	}
	if !havePoint {
		return data, false
	}
	data.Benchmark = bench
	data.Gap = gap
	data.Behind = gap < 0
	if recovered := gap - worstGap; recovered > 0 {
		data.GapClosed = recovered
	}

	decisions, err := db.AllPortfolioDecisions()
	if err != nil {
		// The gap stats alone still make an honest email; an empty Wins
		// slice just drops the receipts section from the template.
		return data, true
	}
	trips := trades.Derive(decisions) // newest-close first
	cutoff := ""
	if t, perr := time.Parse("2006-01-02", today); perr == nil {
		cutoff = t.AddDate(0, 0, -14).Format("2006-01-02")
	}
	recent := trips[:0:0]
	for _, t := range trips {
		if t.ClosedDate >= cutoff {
			recent = append(recent, t)
		}
	}
	// A thin fortnight reads as cherry-picking, so fall back to the six most
	// recent closes overall and label the window accordingly.
	if len(recent) < 3 && len(trips) > len(recent) {
		n := min(6, len(trips))
		recent = trips[:n]
		data.WindowLabel = fmt.Sprintf("my last %d closes", n)
	}
	const maxWinRows = 8
	for _, t := range recent {
		switch {
		case t.RealizedPnl > 0:
			if len(data.Wins) < maxWinRows {
				data.Wins = append(data.Wins, templates.FableReturnWin{
					Label:      tripLabel(t),
					ClosedLong: longDate(t.ClosedDate),
					HoldDays:   t.HoldDays,
					Pnl:        t.RealizedPnl,
					Pct:        t.RealizedPct,
				})
			}
		case t.RealizedPnl < 0:
			data.LossCount++
			data.LossTotal += t.RealizedPnl
		}
		data.NetRealized += t.RealizedPnl
	}
	return data, true
}

// tripLabel renders a round trip as the short human label the win table
// shows: "NVDA $190 calls" for options, the bare ticker for legacy equity.
func tripLabel(t trades.RoundTrip) string {
	name := t.Underlying
	if name == "" {
		name = t.Symbol
	}
	if t.AssetType != "OPTION" {
		return name
	}
	side := "calls"
	if strings.EqualFold(t.ContractType, "PUT") || strings.EqualFold(t.ContractType, "P") {
		side = "puts"
	}
	if t.Strike != nil && *t.Strike > 0 {
		return fmt.Sprintf("%s $%s %s", name, strconv.FormatFloat(*t.Strike, 'f', -1, 64), side)
	}
	return name + " " + side
}

// farewellUpdateKey is the sent_emails ledger key for the one-time farewell
// letter announcing the platform's decommission: the operator has taken a
// research-intensive role and can no longer run the service, so the book
// closes at the next Monday open and the account winds down. Self-gating
// like the other one-time updates.
const farewellUpdateKey = "farewell_decommission_v1"

/*
sendFarewellUpdate sends the one-time farewell letter. Like
sendFableReturnUpdate it carries live numbers (the final start-vs-end
accounting, the SPY gap, the round-trip record), so when the store can't
support honest numbers the send is deferred WITHOUT claiming the ledger key
and retries on a later boot. Otherwise the same self-gating shape as the
other one-time updates: claims its sent_emails key only after at least one
recipient received it, and never re-blasts. Best-effort: a render or send
failure is logged, never fatal.
*/
func sendFarewellUpdate(cfg *config.Config, db *store.Store, emailClient *email.Client) {
	if sent, err := db.EmailAlreadySent(farewellUpdateKey); err != nil {
		log.Printf("farewell update: could not read send ledger, skipping to be safe: %v", err)
		return
	} else if sent {
		log.Printf("farewell update: already sent (key=%s), nothing to do", farewellUpdateKey)
		return
	}

	recipients := getRecipients(db)
	if len(recipients) == 0 {
		log.Printf("farewell update: no subscribers, nothing to send")
		return
	}

	data, ok := buildFarewellData(cfg, db)
	if !ok {
		log.Printf("farewell update: not enough performance data yet, deferring send to a later boot")
		return
	}

	html, err := templates.RenderFarewellUpdate(data)
	if err != nil {
		log.Printf("farewell update: render email: %v", err)
		return
	}
	subject := "A last letter from Claudia"
	res := emailClient.SendPersonalizedToList(cfg.EmailFrom, recipients, subject, html, unsubURLBuilder(cfg))
	log.Printf("farewell update: email sent to %d/%d subscribers", res.Succeeded, res.Total)
	if res.Failed > 0 {
		log.Printf("farewell update: email failures: %s", res.FailureDetail())
	}
	if res.Succeeded > 0 {
		if err := db.MarkEmailSent(farewellUpdateKey); err != nil {
			log.Printf("farewell update: WARNING sent but failed to record in ledger (could re-send on next boot): %v", err)
		}
	}
}

/*
buildFarewellData assembles the final accounting for the farewell letter.
Returns ok=false when the equity curve has no usable point (fresh or broken
store), so the caller defers rather than sending a letter with hollow
numbers.

Start/end equity are the first and last curve points with a positive equity
value. The SPY benchmark is the same normalization the dashboard chart and
the Fable-return email use: the first dual-valued point's equity riding SPY
closes (equity0 x SPY_t / SPY0). The round-trip record comes from the shared
trades derivation, the same source the /closed page reads.
*/
func buildFarewellData(cfg *config.Config, db *store.Store) (templates.FarewellData, bool) {
	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	today := todayDate()
	data := templates.FarewellData{
		BaseURL: base,
		AsOf:    longDate(today),
	}

	pts, err := db.GetEquityCurve("0000-00-00", today)
	if err != nil {
		return data, false
	}
	var (
		haveStart           bool
		haveBench           bool
		baseEquity, baseSPY float64
	)
	for _, p := range pts {
		if p.AccountEquity <= 0 {
			continue
		}
		data.TradingDays++
		if !haveStart {
			data.StartEquity = p.AccountEquity
			data.StartDate = longDate(p.Date)
			haveStart = true
		}
		data.CurrentEquity = p.AccountEquity
		if p.SPYClose > 0 {
			if !haveBench {
				baseEquity, baseSPY = p.AccountEquity, p.SPYClose
				haveBench = true
			}
			data.Benchmark = baseEquity * p.SPYClose / baseSPY
		}
	}
	if !haveStart {
		return data, false
	}
	data.TotalPnl = data.CurrentEquity - data.StartEquity
	data.AbsTotalPnl = absFloat(data.TotalPnl)
	data.Lost = data.TotalPnl < 0
	if haveBench {
		data.Gap = data.CurrentEquity - data.Benchmark
		data.AbsGap = absFloat(data.Gap)
		data.Behind = data.Gap < 0
	}

	decisions, err := db.AllPortfolioDecisions()
	if err != nil {
		// The equity accounting alone still makes an honest letter; a zero
		// Trips count just softens the record line.
		return data, true
	}
	trips := trades.Derive(decisions)
	data.Trips = len(trips)
	for _, t := range trips {
		switch {
		case t.RealizedPnl > 0:
			data.Wins++
		case t.RealizedPnl < 0:
			data.Losses++
		}
	}
	return data, true
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
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

/*
runPortfolioRisk is the intraday cron body (every 15 min, market hours).
Sizing limits are enforced at the tool layer when the model trades, not
intraday, so the job's remaining duty is reconciling the decision log's
order statuses against the broker so fills show up on the dashboard within
minutes.
*/
func runPortfolioRisk(db *store.Store, executor *exec.Service) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Keep the decision log honest: the log is insert-only at placement
	// time, so without this sweep a filled order reads "working" on the
	// dashboard until the end of time.
	reconcileOrderStatuses(ctx, db, executor)
}

/*
reconcileOrderStatuses asks the broker for the current state of every
order the decision log still considers open and writes the answer back
(status, and the average fill price once filled). Runs on the 15-minute
risk cadence and once more at EOD, so the dashboard's executions tape
shows FILLED within minutes of the broker filling instead of "working"
forever. Best-effort per order: one unreachable order must not stall the
rest.
*/
func reconcileOrderStatuses(ctx context.Context, db *store.Store, executor *exec.Service) {
	if db == nil || executor == nil {
		return
	}
	ids, err := db.OpenOrderIDs()
	if err != nil {
		log.Printf("order reconcile: open ids: %v", err)
		return
	}
	for _, id := range ids {
		st, err := executor.OrderStatusAgent(ctx, id)
		if err != nil {
			log.Printf("order reconcile: status %s: %v", id, err)
			continue
		}
		if st.RawStatus == "" {
			continue
		}
		if err := db.UpdateDecisionOrderStatus(id, st.RawStatus, st.FillPrice); err != nil {
			log.Printf("order reconcile: update %s: %v", id, err)
			continue
		}
		if st.Filled {
			log.Printf("order reconcile: %s FILLED @ %.2f", id, st.FillPrice)
		}
	}
}

/*
runPortfolioEODSnapshot is the 16:00 ET cron body. It records one
equity-curve point for the day: account equity, cash, the running
high-water mark, and the SPY close for the benchmark. Upserted by date, so
a re-run overwrites cleanly. The day's last order-status reconcile runs
here too, so overnight readers see the settled truth.
*/
func runPortfolioEODSnapshot(db *store.Store, reader *portfoliowire.Reader) {
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
	// No recap email is sent here anymore: the model writes and sends its own
	// recap at the end of each session it trades, via the send_recap_email tool.
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
