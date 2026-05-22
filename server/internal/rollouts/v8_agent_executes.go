package rollouts

import "vibetradez.com/internal/templates"

/*
agentExecutesV8 announces the agent-driven at-open execution model
that ships in this deploy. The previous v7 rollout described a
deterministic Go resolver that walked the live chain to snap each
picker intent to a contract. That whole code path is retired; the
9:25 ET morning picker output is now fed into a second Claude call at
9:30:00 ET that owns contract selection AND order placement via a
locked-down `place_options_order` tool.

The body walks subscribers through the new five-stage day:

 1. 9:25 ET picker chooses 3 candidate tickers (unchanged from v7).
 2. 9:25 ET morning email goes out with intent text (unchanged).
 3. 9:30:00 ET at-open agent decides + executes (NEW).
 4. 9:35 ET cancel-dangling + consolidated summary email.
 5. 3:55 PM ET mandatory close + EOD email.

It also surfaces the hard caps enforced in code (one order per
candidate, max 3 per day, $10/share per-contract premium ceiling,
symbol allowlist) so subscribers know the model can't go off-script.

Slug is permanent. v7's `at-open-resolution-v7` stays recorded in
sent_rollouts forever; this v8 slug is the new permission slip.
*/
var agentExecutesV8 = Rollout{
	Slug:    "agent-executes-v8",
	Subject: "Claudia now picks the contracts at the open, too",
	Render:  templates.RenderRolloutAgentExecutes,
}
