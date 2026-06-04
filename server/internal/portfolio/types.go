package portfolio

import "vibetradez.com/internal/transcript"

/*
AssetType is the instrument class of a position or a proposed move.
The portfolio agent trades long equity and long single-leg options
only (no shorting, no spreads — see the design doc non-goals).
*/
type AssetType string

const (
	AssetEquity AssetType = "EQUITY"
	AssetOption AssetType = "OPTION"
)

/*
Action is the verb of a move the agent proposes through the tool layer.
buy_* opens or adds to a position, sell_* trims or closes an existing
holding, hold is the explicit no-op.
*/
type Action string

const (
	ActionBuyEquity  Action = "buy_equity"
	ActionSellEquity Action = "sell_equity"
	ActionBuyOption  Action = "buy_option"
	ActionSellOption Action = "sell_option"
	ActionHold       Action = "hold"
)

/*
Position is one holding in the account, as reported by the broker.
MarkValue is the current market value of the whole position (mark times
quantity times the contract multiplier for options). Underlying is the
equity ticker the position keys against for concentration accounting: for
an equity position it equals Symbol, for an option it is the underlying
stock symbol (not the OCC option symbol). This is what lets the
concentration cap treat MDT stock and MDT calls as exposure to the same
name.
*/
type Position struct {
	Symbol     string    // equity ticker, or OCC option symbol for options
	Underlying string    // equity ticker the exposure keys against
	AssetType  AssetType // EQUITY | OPTION
	Quantity   float64   // shares, or contract count for options
	MarkValue  float64   // current market value of the whole position, USD
	CostBasis  float64   // total cost basis of the whole position, USD

	// Option-specific. Zero/empty for equity positions.
	ContractType string  // "CALL" | "PUT"
	Strike       float64 // strike price
	Expiration   string  // YYYY-MM-DD
	DTE          int     // days to expiration (derived at snapshot time)
}

/*
Snapshot is the account state the cap checks and the agent reason over,
captured at the start of a session. Equity is the total account value
(positions mark plus cash). SettledCash is what new buys may spend in a
cash account (unsettled proceeds cannot be redeployed until T+1).
HighWaterMark is the peak account equity observed historically, used by
the drawdown breaker. DeploymentBudget is the dollar amount of NEW buys
allowed this session (settled cash at session start times the
daily-deployment cap); DeployedThisSession accrues as the agent buys.
*/
type Snapshot struct {
	Equity              float64
	SettledCash         float64
	UnsettledCash       float64
	HighWaterMark       float64
	DeploymentBudget    float64
	DeployedThisSession float64
	Positions           []Position
}

/*
LiquidityCtx carries the tradability facts a buy move must clear against
the liquidity floor. The dispatcher fills these from the live quote /
chain before calling the cap check, so the floor is enforced on real
market data rather than the model's say-so.
*/
type LiquidityCtx struct {
	StockPrice         float64 // underlying last/mark
	UnderlyingMktCap   float64 // underlying market capitalization, USD
	OptionOpenInterest int     // option contract open interest (options only)
	OptionVolume       int     // option contract day volume (options only)
	SpreadPct          float64 // (ask-bid)/mark on the instrument being bought
}

/*
Move is a single proposed action the agent issues through a tool. Notional
is the USD the move commits: price times shares for equity, limit times
contracts times 100 for options. For sell moves Notional is the proceeds
(informational) and Quantity is what to sell.
*/
type Move struct {
	Action     Action
	AssetType  AssetType
	Symbol     string // equity ticker or OCC option symbol
	Underlying string // equity ticker the exposure keys against
	Quantity   float64
	Notional   float64
	Liquidity  LiquidityCtx
}

/*
Decision is the recorded outcome of one move the agent committed (or a
hold). The dispatcher seeds these as tools succeed; they are the source of
truth for what happened, the agent's final JSON is consulted only for the
written rationale and the overall stance note (mirrors execagent's
buy-is-truth, JSON-is-rationale split).
*/
type Decision struct {
	Action      Action
	AssetType   AssetType
	Symbol      string
	Underlying  string
	Quantity    float64
	Notional    float64
	LimitPrice  float64
	OrderID     string
	ExecutionID int
	Rationale   string
}

/*
HistoryEntry is one past move the agent made, surfaced by the
get_recent_decisions tool so the manager can keep a coherent thesis across
days instead of re-deriving from scratch each session.
*/
type HistoryEntry struct {
	Date       string
	Action     string
	Symbol     string
	Underlying string
	Notional   float64
	Rationale  string
}

// StanceEntry is one past day's overall stance note.
type StanceEntry struct {
	Date   string
	Stance string
}

/*
Result is what Run returns to the caller (the daily cron). Decisions are
every move committed this session in commit order, Stance is the agent's
one-paragraph overall read of the book it wrote in its final JSON, and
Transcript is the captured conversation (non-nil even on a partial/error
run so the caller can persist whatever reasoning was produced).
*/
type Result struct {
	Decisions []Decision
	Stance    string
	// Summary is today's synopsis and ActionItems is the plan for the next
	// session, both recorded via the write_summary documentation tool. Empty
	// if the model didn't call it.
	Summary     string
	ActionItems string
	Transcript  *transcript.Transcript
}
