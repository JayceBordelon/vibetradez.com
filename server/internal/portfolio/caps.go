package portfolio

import (
	"fmt"
	"strings"
)

/*
Caps is the hard-cap risk policy: two sleeve limits, each a fraction of
LIVE account equity read at the start of the session. The operator's
standing policy is "the model does what it wants with all the money, split
no worse than half stock / half options": there is no per-name
concentration cap, no per-order cap, no session deployment pacing, no
drawdown breaker, and no liquidity floor. The only other gates anywhere
are the settled-cash rule below (broker T+1 compliance, not a risk
preference) and exec.MaxPortfolioOrderCostCeiling, a flat fat-finger
ceiling at the broker entry that backstops code bugs, not trading.

The sleeve caps are enforced at the tool layer (tools.go) via the Check*
methods below, and that is the SOLE enforcement point. The broker entry
point does NOT re-validate them, so this struct is the single source of
truth, and CapsSummary renders it into the agent prompt so the prose and
the enforced values never drift.
*/
type Caps struct {
	// MaxOptionsSleevePct caps the total option premium held at once as a
	// fraction of equity. Options can expire to zero; half the account is
	// the most that can be on the leverage sleeve.
	MaxOptionsSleevePct float64

	// MaxEquitySleevePct caps the total stock value held at once as a
	// fraction of equity, the other half of the 50/50 split policy.
	MaxEquitySleevePct float64
}

// DefaultCaps returns the operator policy: a 50/50 sleeve split.
func DefaultCaps() Caps {
	return Caps{
		MaxOptionsSleevePct: 0.50,
		MaxEquitySleevePct:  0.50,
	}
}

// floatTol absorbs floating-point noise so a move sized exactly to a cap
// (e.g. notional == remaining sleeve) is allowed rather than rejected by
// a rounding hair.
const floatTol = 1e-6

/*
CapError is a single rejected-by-policy outcome. Code is a stable
machine-readable tag (useful for the dashboard / tests), Message is the
human- and model-readable explanation the tool returns verbatim so the
model can react (size down, switch sleeves, or hold).
*/
type CapError struct {
	Code    string
	Message string
}

func (e *CapError) Error() string { return e.Message }

func capErr(code, format string, a ...any) *CapError {
	return &CapError{Code: code, Message: fmt.Sprintf(format, a...)}
}

// sleeveValue sums the current market value of every position of the
// given asset type, for the sleeve caps.
func sleeveValue(s Snapshot, t AssetType) float64 {
	var total float64
	for _, p := range s.Positions {
		if p.AssetType == t {
			total += p.MarkValue
		}
	}
	return total
}

/*
CheckBuy runs the buy-side policy against a proposed move and the current
snapshot, returning the first violation (or nil if the move is allowed):
the settled-cash rule first (broker compliance), then the sleeve the buy
lands in. The caller must have filled move.Notional with the committed
dollar amount. CheckBuy mutates nothing.
*/
func (c Caps) CheckBuy(s Snapshot, m Move) *CapError {
	if m.Action != ActionBuyEquity && m.Action != ActionBuyOption {
		return capErr("not_a_buy", "CheckBuy called with non-buy action %q", m.Action)
	}
	if m.Notional <= 0 {
		return capErr("bad_notional", "order notional must be > 0, got %.2f", m.Notional)
	}

	// 1. Settled-cash rule (cash account: cannot spend unsettled proceeds).
	if m.Notional > s.SettledCash+floatTol {
		return capErr("settled_cash",
			"order notional $%.2f exceeds settled cash $%.2f. Unsettled proceeds cannot be redeployed until they settle (T+1).",
			m.Notional, s.SettledCash)
	}

	// 2. Sleeve cap for the asset type being bought.
	if m.AssetType == AssetOption {
		sleeveCap := s.Equity * c.MaxOptionsSleevePct
		projected := sleeveValue(s, AssetOption) + m.Notional
		if projected > sleeveCap+floatTol {
			return capErr("options_sleeve",
				"this buy would put $%.2f of option premium at risk, over the options-sleeve cap $%.2f (%.0f%% of $%.2f equity). Use equity for this exposure or size down.",
				projected, sleeveCap, c.MaxOptionsSleevePct*100, s.Equity)
		}
	} else {
		sleeveCap := s.Equity * c.MaxEquitySleevePct
		projected := sleeveValue(s, AssetEquity) + m.Notional
		if projected > sleeveCap+floatTol {
			return capErr("equity_sleeve",
				"this buy would put $%.2f in stock, over the equity-sleeve cap $%.2f (%.0f%% of $%.2f equity). Use options for this exposure or size down.",
				projected, sleeveCap, c.MaxEquitySleevePct*100, s.Equity)
		}
	}

	return nil
}

/*
CheckSell validates a de-risking move: the position must exist and the
quantity to sell cannot exceed what is held. Sells are never blocked by
the sleeve caps (reducing exposure is always allowed). Returns nil if the
sell is valid.
*/
func (c Caps) CheckSell(s Snapshot, m Move) *CapError {
	if m.Action != ActionSellEquity && m.Action != ActionSellOption {
		return capErr("not_a_sell", "CheckSell called with non-sell action %q", m.Action)
	}
	if m.Quantity <= 0 {
		return capErr("bad_quantity", "sell quantity must be > 0, got %.4g", m.Quantity)
	}
	want := strings.ToUpper(strings.TrimSpace(m.Symbol))
	for _, p := range s.Positions {
		if strings.ToUpper(strings.TrimSpace(p.Symbol)) != want {
			continue
		}
		if m.Quantity > p.Quantity+floatTol {
			return capErr("oversell",
				"cannot sell %.4g of %s, only %.4g held.",
				m.Quantity, want, p.Quantity)
		}
		return nil
	}
	return capErr("no_position", "no open position in %s to sell.", want)
}
