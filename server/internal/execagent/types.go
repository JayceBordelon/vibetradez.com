/*
Package execagent runs the 9:30:00 ET at-open execution agent.

The morning picker (internal/trades) selects 3 candidate tickers + contract
intent (target_otm_pct, min_dte) at 9:25 ET. This package replaces the
deterministic resolver+selector that used to fire orders at the open:
instead, Claude is re-invoked at 9:30:00 ET with the now-live option
chain and a dedicated `place_options_order` tool, and decides per-pick
whether to buy and at what strike/expiration/limit.

Design constraints (this package is the only place these are enforced):

  - The tool surface is the boundary. exec/ has zero awareness of
    Claude; trades/ has zero awareness of Schwab Trader. This file
    defines the structs that cross both boundaries.
  - Per-pick "skip with reason" is a first-class outcome, not an error.
    A pick where Claude decided not to buy gets persisted as an
    Execution with status="skipped" and the reason in ErrorMessage.
  - Hard caps live next to the tools (tools.go), NOT in the prompt.
    A malicious or buggy model output must not be able to bypass them.
*/
package execagent

import "vibetradez.com/internal/trades"

/*
DecisionAction is the per-pick outcome the agent reconciles in its
final JSON response. "buy" means the agent successfully called
place_options_order during the tool loop and the broker accepted the
order; "skip" means Claude explicitly chose not to trade this pick
with a reason. Any candidate without an explicit "buy" or "skip" in
the agent's final output is treated as "skip" with a synthetic reason
(see agent.go reconciliation).
*/
type DecisionAction string

const (
	ActionBuy  DecisionAction = "buy"
	ActionSkip DecisionAction = "skip"
)

/*
Decision is the per-pick outcome the agent runner persists. One Decision
per Candidate, regardless of whether Claude acted on it. For buys, the
fields after Action are the order Claude chose. For skips, SkipReason
is required and the order fields are zero.
*/
type Decision struct {
	Rank   int
	Symbol string
	Action DecisionAction

	// Populated on buy outcomes only.
	OrderID     string
	Strike      float64
	Expiration  string
	LimitPrice  float64
	Quantity    int
	ExecutionID int // executions.id row written for this buy

	// Populated on skip outcomes only.
	SkipReason string
	SkipExecID int // executions.id row written for this skip (status='skipped')
}

/*
Candidate is one morning-picker output the agent considers. Mirrors the
five-field intent contract: rank, symbol, contract_type, target_otm_pct,
min_dte. The agent is given the full trades.Trade so the prompt can
surface thesis/rationale/score for context, but only these five fields
gate execution decisions.
*/
type Candidate struct {
	Trade trades.Trade
}

/*
ExecagentResult is what the agent returns to its caller (the 9:30 cron
in cmd/scanner/main.go). One Decision per Candidate in candidate order,
plus a top-level FinalNote Claude wrote about the overall basket (e.g.,
"overnight gap on tech names dampened conviction across the board").
The cron logs the result and the 9:35 SendExecutionSummary email
picks up the persisted execution rows.
*/
type ExecagentResult struct {
	Decisions []Decision
	FinalNote string
}
