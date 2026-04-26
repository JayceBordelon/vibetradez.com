package exec

import "time"

/*
Decision is one row of the daily go/no-go pipeline. There is at most
one Decision per trade_date by schema constraint. Decisions start as
'pending', then transition to 'execute' (user clicked Execute within
the 5-minute window), 'decline' (user clicked Don't Execute), or
'timeout' (window expired without a click).
*/
type Decision struct {
	ID            int
	TradeDate     string // YYYY-MM-DD ET
	Symbol        string
	ContractType  string // CALL | PUT
	StrikePrice   float64
	Expiration    string // YYYY-MM-DD
	OCCSymbol     string // 21-char OSI
	ContractPrice float64
	GPTScore      int
	ClaudeScore   int
	TradeID       int    // references trades.id
	TokenHash     string // sha256(execute-token); decline-token hash is derivable but unused
	Decision      string // pending | execute | decline | timeout
	DecidedAt     *time.Time
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

/*
Execution is one order lifecycle. A Decision with decision='execute'
has exactly one Execution with side='open'. If the open fills, the
3:55pm cron creates a second Execution with side='close'. PaperTrader
fills are synthetic (no SchwabOrderID); LiveTrader fills carry the
Schwab order id.
*/
type Execution struct {
	ID                int
	DecisionID        int
	Mode              string // paper | live
	Side              string // open | close
	SchwabOrderID     *string
	Status            string // pending | working | filled | canceled | rejected | failed
	FillPrice         *float64
	FilledQuantity    int
	RequestedQuantity int
	SubmittedAt       time.Time
	FilledAt          *time.Time
	ErrorMessage      string
	CreatedAt         time.Time
}
