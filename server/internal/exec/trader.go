package exec

import (
	"context"
	"errors"
	"time"
)

/*
TraderClient abstracts the order-placement surface so the
auto-execution pipeline can be unit-tested with a fake and so the
paper / live mode switch is a single object swap at startup. Live
implementations live in the schwab package; the paper implementation
lives in this package next to the rest of the execution logic.
*/
type TraderClient interface {
	/*
		AccountHash returns the Schwab-issued account hash (NOT the raw
		account number). Almost all other endpoints take the hash. Live
		implementations cache; paper returns a synthetic constant.
	*/
	AccountHash(ctx context.Context) (string, error)

	/*
		PlaceOrder submits an order. Returns the order id from the broker
		(or a synthetic "paper-<uuid>" id in paper mode). Errors from the
		broker (validation, rejection, insufficient permissions) come back
		as a non-nil error; the caller decides whether to retry, escalate,
		or page the human.
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
}

/*
Order is the canonical wire shape submitted to the broker. The struct
mirrors Schwab's Trader API JSON 1:1 so the live implementation can
marshal directly. Fields that aren't relevant for single-leg options
are omitted entirely rather than left zero — the broker is strict
about extra fields.
*/
type Order struct {
	OrderType          string     `json:"orderType"`         // "MARKET"
	Session            string     `json:"session"`           // "NORMAL"
	Duration           string     `json:"duration"`          // "DAY"
	OrderStrategyType  string     `json:"orderStrategyType"` // "SINGLE"
	OrderLegCollection []OrderLeg `json:"orderLegCollection"`
}

type OrderLeg struct {
	Instruction string     `json:"instruction"` // "BUY_TO_OPEN" | "SELL_TO_CLOSE"
	Quantity    int        `json:"quantity"`
	Instrument  Instrument `json:"instrument"`
}

type Instrument struct {
	Symbol    string `json:"symbol"`    // 21-char OCC OSI
	AssetType string `json:"assetType"` // "OPTION"
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
	UpdatedAt      time.Time
	ErrorMessage   string // populated on REJECTED
}

/*
ErrInsufficientFunds, ErrUnsupportedStrategy, etc. could be added
here as the live trader matures and we learn which broker errors
warrant special handling. v1 just wraps errors generically.
*/
var ErrInvalidOrder = errors.New("invalid order shape")
