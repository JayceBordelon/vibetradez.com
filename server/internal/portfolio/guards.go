package portfolio

import (
	"fmt"
	"strings"
)

/*
The portfolio manager trades OPTIONS ONLY and sizes methodically rather
than concentrating the account in a single bet. Equity buys are disabled
entirely (see tools.go); legacy shares are liquidated through sell_equity.

The buy-side gates enforced here, in order:

  - Settled-cash rule (CheckBuy): a cash account cannot spend unsettled
    sale proceeds until they settle at T+1. Broker compliance.
  - Per-position cap (CheckOptionSize): a single option position may not
    exceed MaxSingleOptionFrac of account equity.
  - Per-underlying cap (CheckUnderlyingExposure): total exposure to one
    underlying may not exceed MaxPerUnderlyingFrac of account equity.

Sell validation (CheckSell): you cannot sell a position you do not hold,
or sell more than you hold. Sells and de-risking are never blocked.

All are enforced at the tool layer (tools.go), the SOLE enforcement point.
The broker entry point additionally re-checks one flat absolute order-cost
ceiling (exec.MaxPortfolioOrderCostCeiling), a fat-finger backstop against
code bugs, not a trading constraint.
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

/*
Options-only sizing discipline. The mandate is to trade options only and to
diversify rather than over-expose the account to one bet. Two soft risk
limits, expressed as fractions of account equity, are made hard at the tool
layer (the model is told the same limits in the prompt):

  - MaxSingleOptionFrac: one option position may not exceed this share of
    equity (per-position concentration).
  - MaxPerUnderlyingFrac: total exposure to one underlying (all option
    positions on that name plus the proposed buy) may not exceed this share.

Both apply to option BUYS only; sells/de-risking are never blocked, and
equity buys are disabled entirely. When equity is unknown (<= 0) the caps
are skipped rather than blocking all trading.
*/
const (
	MaxSingleOptionFrac  = 0.10
	MaxPerUnderlyingFrac = 0.25
)

/*
CheckOptionSize caps a single option position at MaxSingleOptionFrac of
account equity, measured as the resulting exposure (the current mark of any
existing position in the same contract plus the proposed buy's notional).
Applies to option buys only; returns nil otherwise.
*/
func CheckOptionSize(s Snapshot, m Move) *GuardError {
	if m.Action != ActionBuyOption || s.Equity <= 0 {
		return nil
	}
	limit := MaxSingleOptionFrac * s.Equity
	want := strings.ToUpper(strings.TrimSpace(m.Symbol))
	existing := 0.0
	for _, p := range s.Positions {
		if p.AssetType == AssetOption && strings.ToUpper(strings.TrimSpace(p.Symbol)) == want {
			existing += p.MarkValue
		}
	}
	if existing+m.Notional > limit+floatTol {
		return guardErr("position_too_large",
			"this buy would put $%.2f into a single option position, over the per-position limit of $%.2f (about %.0f%% of $%.2f equity). Size down or spread across names.",
			existing+m.Notional, limit, MaxSingleOptionFrac*100, s.Equity)
	}
	return nil
}

/*
CheckUnderlyingExposure caps total exposure to one underlying at
MaxPerUnderlyingFrac of account equity (every position keyed to that
underlying plus the proposed buy). Applies to option buys only.
*/
func CheckUnderlyingExposure(s Snapshot, m Move) *GuardError {
	if m.Action != ActionBuyOption || s.Equity <= 0 {
		return nil
	}
	limit := MaxPerUnderlyingFrac * s.Equity
	want := strings.ToUpper(strings.TrimSpace(m.Underlying))
	existing := 0.0
	for _, p := range s.Positions {
		if strings.ToUpper(strings.TrimSpace(p.Underlying)) == want {
			existing += p.MarkValue
		}
	}
	if existing+m.Notional > limit+floatTol {
		return guardErr("underlying_concentration",
			"this buy would put $%.2f of total exposure into %s, over the per-underlying limit of $%.2f (about %.0f%% of $%.2f equity). Diversify into another name.",
			existing+m.Notional, want, limit, MaxPerUnderlyingFrac*100, s.Equity)
	}
	return nil
}
