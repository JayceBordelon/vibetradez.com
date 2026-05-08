package rollouts

import "vibetradez.com/internal/templates"

/*
basketAndCopyTradingV4 announces three things, in order:

 1. Today's first real-money winning trade (AKAM 145 CALL,
    entry $1.73, close $6.09, +$435.67 realized P&L).
 2. The shift from rank-1-only auto-execution to a basket of up
    to 3 picks/day capped at $500 total exposure with a Schwab
    cash check on every order. Lands the same week as this email.
 3. The intent to ship a copy-trading mode where subscribers link
    their own Schwab account and the cron mirrors trades against
    their balance, with their own per-day cap. Liability stays
    limited to the cap; not custody, not pooled. Reply-to-join
    waitlist.

Slug is permanent — never rename. The numbers in the template are
specific to today's trade — when next week's win comes in, write a
new rollout instead of recycling this slug.
*/
var basketAndCopyTradingV4 = Rollout{
	Slug:    "basket-and-copy-trading-v4",
	Subject: "+$435 today, and what's coming next on VibeTradez",
	Render:  templates.RenderRolloutBasketAndCopyTrading,
}
