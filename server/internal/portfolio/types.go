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
HighWaterMark is the peak account equity observed historically, surfaced
for context. There is no session deployment budget: the agent may deploy
all settled cash, gated only by the settled-cash rule and the two sleeve
caps.
*/
type Snapshot struct {
	Equity        float64
	SettledCash   float64
	UnsettledCash float64
	HighWaterMark float64
	Positions     []Position
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
TrackRecord is the get_track_record payload: realized outcomes, not
narrative. Closed round trips with P&L, the win/loss stats split by
instrument, and the account-vs-SPY benchmark race. This is the agent's
only quantified feedback signal, so the JSON names are written for the
model to reason over directly.
*/
type TrackRecord struct {
	Overall      TrackRecordStats  `json:"overall"`
	EquityTrades TrackRecordStats  `json:"equity_trades"`
	OptionTrades TrackRecordStats  `json:"option_trades"`
	RecentTrips  []TrackRecordTrip `json:"recent_closed_trades"`
	Benchmark    BenchmarkRace     `json:"benchmark"`
}

// TrackRecordStats summarizes one bucket of closed round trips.
type TrackRecordStats struct {
	Trades           int     `json:"trades"`
	Wins             int     `json:"wins"`
	Losses           int     `json:"losses"`
	WinRatePct       float64 `json:"win_rate_pct"`
	TotalRealizedPnl float64 `json:"total_realized_pnl"`
	AvgWinPnl        float64 `json:"avg_win_pnl"`
	AvgLossPnl       float64 `json:"avg_loss_pnl"`
	AvgHoldDays      float64 `json:"avg_hold_days"`
	BiggestWin       float64 `json:"biggest_win"`
	BiggestLoss      float64 `json:"biggest_loss"`
}

// TrackRecordTrip is one completed round trip in the detail list.
type TrackRecordTrip struct {
	Symbol         string  `json:"symbol"`
	Underlying     string  `json:"underlying,omitempty"`
	AssetType      string  `json:"asset_type"`
	OpenedDate     string  `json:"opened_date"`
	ClosedDate     string  `json:"closed_date"`
	HoldDays       int     `json:"hold_days"`
	EntryPrice     float64 `json:"entry_price"`
	ExitPrice      float64 `json:"exit_price"`
	RealizedPnl    float64 `json:"realized_pnl"`
	RealizedPct    float64 `json:"realized_pct"`
	OpenRationale  string  `json:"open_rationale,omitempty"`
	CloseRationale string  `json:"close_rationale,omitempty"`
}

// BenchmarkRace is the account's return since inception against
// buy-and-hold SPY over the same window, with the deepest drawdown.
type BenchmarkRace struct {
	TradingDays      int     `json:"trading_days"`
	FirstDate        string  `json:"first_date,omitempty"`
	LastDate         string  `json:"last_date,omitempty"`
	StartEquity      float64 `json:"start_equity"`
	CurrentEquity    float64 `json:"current_equity"`
	AccountReturnPct float64 `json:"account_return_pct"`
	SpyReturnPct     float64 `json:"spy_return_pct"`
	VsSpyPct         float64 `json:"vs_spy_pct"`
	MaxDrawdownPct   float64 `json:"max_drawdown_pct"`
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
