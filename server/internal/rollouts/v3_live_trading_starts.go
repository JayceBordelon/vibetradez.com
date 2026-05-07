package rollouts

import "vibetradez.com/internal/templates"

/*
liveTradingStartsV3 announces the flip from paper to live trading. The
runtime guard already had TRADING_MODE=live since 2026-05-05, but the
order pipeline was silently failing — the LIMIT-on-open fix in PR #62
is what makes it actually work. This email goes out once on first
startup after that PR ships, telling subscribers tomorrow's 9:30 ET
open is the first real-money trade.

Slug is permanent — never rename.
*/
var liveTradingStartsV3 = Rollout{
	Slug:    "live-trading-starts-v3",
	Subject: "Tomorrow: trades go LIVE — real money on a 1-contract leash",
	Render:  templates.RenderRolloutLiveTradingStarts,
}
