package exec

import "time"

/*
Execution is one order lifecycle. Every weekday the cron fires one
Execution per BasketEntry the selector returns (typically 3 to 6
entries, sometimes fewer when a top-3 pick is over the per-contract
cap). Each carries the (trade, quantity) pair so the broker order
is one quantity=N submission per entry, not N separate single-
contract orders. For each open that fills, the 3:55pm cron creates a
matching Execution with side='close' at the same quantity.
PaperTrader fills are synthetic (no SchwabOrderID); LiveTrader fills
carry the Schwab order id. Each row references the trades.id row
that spawned it so dashboard queries can join back to the contract
spec.
*/
type Execution struct {
	ID                int
	TradeID           int
	Mode              string
	Side              string
	SchwabOrderID     *string
	Status            string
	FillPrice         *float64
	FilledQuantity    int
	RequestedQuantity int
	SubmittedAt       time.Time
	FilledAt          *time.Time
	ErrorMessage      string
	CreatedAt         time.Time
}

/*
OpenPosition pairs an Execution with the contract spec of the trade
that spawned it. The store builds it (via a join) so the close cron
can rebuild the OCC symbol and compute realized P&L without a second
DB hit. Lives in exec because it's an exec-time lifecycle struct, not
a persistence concern; if it lived in store we'd have a circular
import (store already imports exec for the Execution type).
*/
type OpenPosition struct {
	Execution     Execution
	Symbol        string
	ContractType  string
	StrikePrice   float64
	Expiration    string
	ContractPrice float64
}
