package rollouts

import "vibetradez.com/internal/templates"

/*
executionRewriteV6 announces the trade execution rewrite that ships
in this deploy. Three pieces:

 1. Acknowledge the prior "equity walk" basket logic was leaving
    high-conviction picks unfilled. The $5/share per-contract cap and
    the $500 daily basket budget combined to skip rank-1 picks
    whenever post-open option premiums ran above either gate, which
    produced hyper-conservative execution on otherwise valid batches.

 2. The new two-phase selector: top 3 picks always fire (cap-eligible
    only), then greedy fill of additional contracts from picks 1-10
    until cumulative exposure approaches $1,000. Duplicates allowed;
    the selector merges them into one quantity=N order per pick.

 3. Per-contract premium ceiling lifted from $5 to $10. Worst-case
    daily exposure is bounded by Schwab cash and by the picker's
    rank-1-through-10 universe; phase 1 can overshoot the $1,000
    target on a high-premium morning by design.

Slug is permanent. Future execution-logic changes should mint a new
slug rather than reuse this one.
*/
var executionRewriteV6 = Rollout{
	Slug:    "execution-rewrite-v6",
	Subject: "Selector rewrite: top 3 always fire, basket fills to $1k",
	Render:  templates.RenderRolloutExecutionRewrite,
}
