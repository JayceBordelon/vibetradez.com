package portfolio

import (
	"fmt"
	"strings"
)

/*
The portfolio manager has full discretion over what it holds and how it
splits the account between stocks and options. There is no allocation
split, no per-name concentration cap, no per-order cap, no drawdown
breaker, no liquidity floor, and no session pacing: the model sizes and
picks instruments however it judges best.

Two gates remain, enforced here, and neither is a risk preference:

  - The settled-cash rule (CheckBuy): a cash account cannot spend
    unsettled sale proceeds until they settle at T+1. This is broker
    compliance, not a view on how the money should be allocated.
  - Sell validation (CheckSell): you cannot sell a position you do not
    hold, or sell more than you hold.

Both are enforced at the tool layer (tools.go), which is the SOLE
enforcement point. The broker entry point additionally re-checks one flat
absolute order-cost ceiling (exec.MaxPortfolioOrderCostCeiling), a
fat-finger backstop against code bugs, not a trading constraint.
*/

// floatTol absorbs floating-point noise so a buy sized to exactly the
// available settled cash is allowed rather than rejected by a rounding hair.
const floatTol = 1e-6

/*
GuardError is a single rejected-by-policy outcome. Code is a stable
machine-readable tag (useful for the dashboard / tests), Message is the
human- and model-readable explanation the tool returns verbatim so the
model can react (size down or hold).
*/
type GuardError struct {
	Code    string
	Message string
}

func (e *GuardError) Error() string { return e.Message }

func guardErr(code, format string, a ...any) *GuardError {
	return &GuardError{Code: code, Message: fmt.Sprintf(format, a...)}
}

/*
CheckBuy runs the buy-side gate against a proposed move and the current
snapshot, returning the first violation (or nil if the move is allowed).
The only buy-side gate is the settled-cash rule (broker compliance): how
much goes into which instrument is entirely the model's call. The caller
must have filled move.Notional with the committed dollar amount. CheckBuy
mutates nothing.
*/
func CheckBuy(s Snapshot, m Move) *GuardError {
	if m.Action != ActionBuyEquity && m.Action != ActionBuyOption {
		return guardErr("not_a_buy", "CheckBuy called with non-buy action %q", m.Action)
	}
	if m.Notional <= 0 {
		return guardErr("bad_notional", "order notional must be > 0, got %.2f", m.Notional)
	}

	// Settled-cash rule (cash account: cannot spend unsettled proceeds).
	if m.Notional > s.SettledCash+floatTol {
		return guardErr("settled_cash",
			"order notional $%.2f exceeds settled cash $%.2f. Unsettled proceeds cannot be redeployed until they settle (T+1).",
			m.Notional, s.SettledCash)
	}

	return nil
}

/*
CheckSell validates a de-risking move: the position must exist and the
quantity to sell cannot exceed what is held. Sells are never blocked on
allocation grounds (reducing exposure is always allowed). Returns nil if
the sell is valid.
*/
func CheckSell(s Snapshot, m Move) *GuardError {
	if m.Action != ActionSellEquity && m.Action != ActionSellOption {
		return guardErr("not_a_sell", "CheckSell called with non-sell action %q", m.Action)
	}
	if m.Quantity <= 0 {
		return guardErr("bad_quantity", "sell quantity must be > 0, got %.4g", m.Quantity)
	}
	want := strings.ToUpper(strings.TrimSpace(m.Symbol))
	for _, p := range s.Positions {
		if strings.ToUpper(strings.TrimSpace(p.Symbol)) != want {
			continue
		}
		if m.Quantity > p.Quantity+floatTol {
			return guardErr("oversell",
				"cannot sell %.4g of %s, only %.4g held.",
				m.Quantity, want, p.Quantity)
		}
		return nil
	}
	return guardErr("no_position", "no open position in %s to sell.", want)
}
