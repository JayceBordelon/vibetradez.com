package rollouts

import "vibetradez.com/internal/templates"

/*
equityWalkDayOneV5 announces three things, in order:

 1. The pre-May-8 trade history was wiped. Today, May 8 2026, is the
    official Day 1 of VibeTradez; the dashboard prior to today was a
    mix of paper-trade modeling and half-deployed feature flags and
    leaving it up would have misrepresented production behavior.

 2. The "Equity Walk Experiment" — top-3 picks/day inside a $500
    hard daily cap, real Schwab fills, intraday-only (9:30 → 3:55
    ET), no backfill or retroactive smoothing. Every dollar charted
    from here on is a dollar a real broker actually moved.

 3. Disclosure of the close-fill-price bug (limit price logged on
    MARKET close orders → realized P&L showed $0.00 on AKAM today)
    and the fix that's now live in the running container. Paper
    mode was never affected.

Slug is permanent — never rename. Numbers and dates in the template
are specific to May 8, 2026 (Day 1); future "clean-slate" emails
should mint a new slug rather than recycle this one.
*/
var equityWalkDayOneV5 = Rollout{
	Slug:    "equity-walk-day-one-v5",
	Subject: "Day 1: clean slate, real money, public equity walk",
	Render:  templates.RenderRolloutEquityWalkDayOne,
}
