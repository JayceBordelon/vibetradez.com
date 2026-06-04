package exec

import (
	"context"
	"errors"
	"time"
)

/*
TraderClient abstracts the order-placement surface so the
auto-execution pipeline can be unit-tested with a fake. The live
implementation lives in the schwab package and is the only production
client; this account trades real money.
*/
type TraderClient interface {
	/*
		AccountHash returns the Schwab-issued account hash (NOT the raw
		account number). Almost all other endpoints take the hash. The live
		implementation caches it.
	*/
	AccountHash(ctx context.Context) (string, error)

	/*
		AvailableFunds returns the cash available to open a new position
		without putting margin at risk. The live implementation calls
		Schwab's account-balance endpoint and prefers
		availableFundsNonMarginableTrade (falls back to
		cashAvailableForTrading then availableFunds when the account type
		doesn't surface the preferred field).
	*/
	AvailableFunds(ctx context.Context, accountHash string) (float64, error)

	/*
		PlaceOrder submits an order. Returns the order id from the broker.
		Errors from the broker (validation, rejection, insufficient
		permissions) come back as a non-nil error; the caller decides
		whether to retry, escalate, or page the human.
	*/
	PlaceOrder(ctx context.Context, accountHash string, order Order) (orderID string, err error)

	/*
		GetOrder returns the current state of an order by id. Used by the
		3:55pm close cron to poll until filled or to decide whether to
		cancel-and-replace.
	*/
	GetOrder(ctx context.Context, accountHash, orderID string) (OrderStatus, error)

	/*
		CancelOrder requests cancellation. Whether the broker honors it
		depends on order state (already filled = error). Caller treats this
		as best-effort.
	*/
	CancelOrder(ctx context.Context, accountHash, orderID string) error

	/*
		GetPositions returns every open position in the account (equity and
		options). The v2 portfolio manager (internal/portfolio) reads this
		at the start of each session to build its Snapshot — the broker is
		the source of truth for what is held, not a local ledger. The
		options-only v1 path never calls this. The live implementation hits
		Schwab's account endpoint with fields=positions.
	*/
	GetPositions(ctx context.Context, accountHash string) ([]BrokerPosition, error)
}

/*
BrokerPosition is one open holding as the broker reports it, normalized
across equity and options. Quantity is net long (longQuantity minus
shortQuantity). MarketValue is the current mark of the whole position;
AverageCost is the per-unit cost basis (per share for equity, per share of
premium for options — multiply by 100 × contracts for option dollar cost).
Underlying is the equity ticker the position keys against for concentration
accounting: equal to Symbol for equity, the option's underlying for
options. Strike/Expiration/ContractType are populated for options only and
may be left zero when the broker payload doesn't carry them (the caller can
decode them from the OCC Symbol).
*/
type BrokerPosition struct {
	Symbol       string  // equity ticker or OCC option symbol
	AssetType    string  // "EQUITY" | "OPTION"
	Underlying   string  // equity ticker the exposure keys against
	Quantity     float64 // net long quantity (shares or contracts)
	MarketValue  float64 // current market value of the whole position, USD
	AverageCost  float64 // per-unit cost basis
	ContractType string  // "CALL" | "PUT" (options only)
	Strike       float64 // options only
	Expiration   string  // YYYY-MM-DD (options only)
}

/*
Order is the canonical wire shape submitted to the broker. The struct
mirrors Schwab's Trader API JSON 1:1 so the live implementation can
marshal directly. Fields that aren't relevant for single-leg options
are omitted entirely rather than left zero — the broker is strict
about extra fields.
*/
type Order struct {
	OrderType          string     `json:"orderType"`         // "MARKET" | "LIMIT"
	Session            string     `json:"session"`           // "NORMAL"
	Duration           string     `json:"duration"`          // "DAY"
	OrderStrategyType  string     `json:"orderStrategyType"` // "SINGLE"
	Price              float64    `json:"price,omitempty"`   // required for LIMIT, omitted for MARKET
	OrderLegCollection []OrderLeg `json:"orderLegCollection"`
}

type OrderLeg struct {
	Instruction string     `json:"instruction"` // options: "BUY_TO_OPEN" | "SELL_TO_CLOSE"; equity: "BUY" | "SELL"
	Quantity    int        `json:"quantity"`
	Instrument  Instrument `json:"instrument"`
}

type Instrument struct {
	Symbol    string `json:"symbol"`    // 21-char OCC OSI for options, plain ticker for equity
	AssetType string `json:"assetType"` // "OPTION" | "EQUITY"
}

/*
OrderStatus is the polled state of a submitted order. Schwab's
vocabulary is large; we collapse the broker-side strings to a small
set the close cron can reason about. RawStatus preserves the original
for logging.
*/
type OrderStatus struct {
	OrderID        string
	RawStatus      string // verbatim from broker
	Filled         bool   // true iff RawStatus == "FILLED" (no partial-fill state in Schwab — partial stays "WORKING")
	Working        bool   // RawStatus == "WORKING" or "QUEUED" or "ACCEPTED" etc.
	Terminal       bool   // RawStatus is one of CANCELED / EXPIRED / REJECTED / FILLED
	FilledQuantity int
	Quantity       int
	FillPrice      float64 // average fill price; 0 if not filled
	LimitPrice     float64 // the LIMIT price the order was placed at; 0 for MARKET orders or when the broker doesn't surface it. Used by the post-open reprice path to compare against a fresh-ask-derived limit before deciding to cancel+replace.
	UpdatedAt      time.Time
	ErrorMessage   string // populated on REJECTED
}

/*
ErrInsufficientFunds, ErrUnsupportedStrategy, etc. could be added
here as the live trader matures and we learn which broker errors
warrant special handling. v1 just wraps errors generically.
*/
var ErrInvalidOrder = errors.New("invalid order shape")
