package exec

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

/*
MarkLookup is the slice of schwab.Client that PaperTrader needs.
Defined as an interface so unit tests can fake it. The real schwab
package implements this via Client.GetOptionMark (added below) which
wraps the existing GetOptionChain.
*/
type MarkLookup interface {
	/*
		OptionMark returns the current mark price for the option contract
		matching (symbol, expiration, contractType, strike). Used to
		synthesize a paper fill at the prevailing market price.
	*/
	OptionMark(ctx context.Context, symbol, expiration, contractType string, strike float64) (float64, error)
}

/*
PaperTrader simulates order execution without touching the Schwab
Trader API. It is the default for TRADING_MODE=paper (which is the
default mode itself). PlaceOrder fills immediately at the current
mark price; GetOrder reads back the synthetic fill state from an
in-memory map keyed by the synthetic order id.

State is in-memory and does NOT survive process restart — but the
load-bearing state (decision row + executions row) lives in
Postgres, so a restart between PlaceOrder and GetOrder is fine: the
caller has already written 'filled' status to executions before
calling GetOrder.
*/
type PaperTrader struct {
	marks MarkLookup

	mu     sync.Mutex
	orders map[string]paperOrder
}

type paperOrder struct {
	occSymbol      string
	side           string // BUY_TO_OPEN | SELL_TO_CLOSE
	quantity       int
	fillPrice      float64
	filledAt       time.Time
	status         string // FILLED | CANCELED
	rejectedReason string
}

func NewPaperTrader(marks MarkLookup) *PaperTrader {
	return &PaperTrader{
		marks:  marks,
		orders: make(map[string]paperOrder),
	}
}

func (pt *PaperTrader) AccountHash(_ context.Context) (string, error) {
	return "PAPER-ACCOUNT", nil
}

func (pt *PaperTrader) PlaceOrder(ctx context.Context, _ string, order Order) (string, error) {
	if len(order.OrderLegCollection) != 1 {
		return "", fmt.Errorf("paper: expected single-leg order, got %d legs", len(order.OrderLegCollection))
	}
	leg := order.OrderLegCollection[0]
	occ := leg.Instrument.Symbol

	/*
		Decode the OCC symbol back to (symbol, expiration, kind, strike) so
		we can look up the mark.
	*/
	sym, exp, kind, strike, err := decodeOCCSymbol(occ)
	if err != nil {
		return "", fmt.Errorf("paper: decode OCC: %w", err)
	}

	mark, err := pt.marks.OptionMark(ctx, sym, exp, kind, strike)
	if err != nil {
		/*
			Paper-mode honesty: if the live mark lookup fails, we don't
			pretend to fill. The execution row gets recorded as 'failed'
			by the caller and the close cron simply has no position to
			close. Better than fabricating a fill price.
		*/
		id := newPaperOrderID()
		pt.mu.Lock()
		pt.orders[id] = paperOrder{
			occSymbol:      occ,
			side:           leg.Instruction,
			quantity:       leg.Quantity,
			status:         "REJECTED",
			rejectedReason: err.Error(),
		}
		pt.mu.Unlock()
		return id, nil
	}

	id := newPaperOrderID()
	pt.mu.Lock()
	pt.orders[id] = paperOrder{
		occSymbol: occ,
		side:      leg.Instruction,
		quantity:  leg.Quantity,
		fillPrice: mark,
		filledAt:  time.Now(),
		status:    "FILLED",
	}
	pt.mu.Unlock()
	return id, nil
}

func (pt *PaperTrader) GetOrder(_ context.Context, _, orderID string) (OrderStatus, error) {
	pt.mu.Lock()
	o, ok := pt.orders[orderID]
	pt.mu.Unlock()
	if !ok {
		return OrderStatus{}, fmt.Errorf("paper: unknown order %q", orderID)
	}
	st := OrderStatus{
		OrderID:        orderID,
		RawStatus:      o.status,
		Quantity:       o.quantity,
		FilledQuantity: 0,
		FillPrice:      o.fillPrice,
		UpdatedAt:      time.Now(),
		ErrorMessage:   o.rejectedReason,
	}
	if o.status == "FILLED" {
		st.Filled = true
		st.Terminal = true
		st.FilledQuantity = o.quantity
	}
	if o.status == "CANCELED" || o.status == "REJECTED" {
		st.Terminal = true
	}
	return st, nil
}

func (pt *PaperTrader) CancelOrder(_ context.Context, _, orderID string) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	o, ok := pt.orders[orderID]
	if !ok {
		return fmt.Errorf("paper: unknown order %q", orderID)
	}
	if o.status == "FILLED" {
		return errors.New("paper: cannot cancel filled order")
	}
	o.status = "CANCELED"
	pt.orders[orderID] = o
	return nil
}

func newPaperOrderID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "paper-" + hex.EncodeToString(b)
}

/*
decodeOCCSymbol is the inverse of OCCSymbol — used by the paper
trader to recover the (symbol, expiration, kind, strike) it needs to
look up a fill price. Live trader doesn't need this since it submits
the OCC string and Schwab handles everything.
*/
func decodeOCCSymbol(occ string) (symbol, expiration, kind string, strike float64, err error) {
	if len(occ) != 21 {
		return "", "", "", 0, fmt.Errorf("OCC symbol must be 21 chars, got %d", len(occ))
	}
	symbol = strings.TrimRight(occ[:6], " ")
	yymmdd := occ[6:12]
	t, perr := time.Parse("060102", yymmdd)
	if perr != nil {
		return "", "", "", 0, fmt.Errorf("OCC date %q: %w", yymmdd, perr)
	}
	expiration = t.Format("2006-01-02")
	switch occ[12] {
	case 'C':
		kind = "CALL"
	case 'P':
		kind = "PUT"
	default:
		return "", "", "", 0, fmt.Errorf("OCC direction byte %q invalid", string(occ[12]))
	}
	strikeStr := occ[13:21]
	var strikeInt int64
	for _, c := range strikeStr {
		if c < '0' || c > '9' {
			return "", "", "", 0, fmt.Errorf("OCC strike contains non-digit %q", string(c))
		}
		strikeInt = strikeInt*10 + int64(c-'0')
	}
	strike = float64(strikeInt) / 1000.0
	return symbol, expiration, kind, strike, nil
}
