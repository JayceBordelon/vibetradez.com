package exec

import (
	"fmt"
	"math"
	"strings"
	"time"

	"vibetradez.com/internal/trades"
)

/*
LimitPriceMultiplier is the buffer applied to the live ask (or the
fallback estimate) when constructing the open LIMIT order. 1.10 =
10% above ask.

Bumped from 1.05 to 1.10 in the execute-at-open redesign: with the
9:35 reprice cron removed, a LIMIT × 1.05 that misses on a fast-
moving open (e.g. TJX gapping +5% in the first second) has no safety
net to re-quote. The extra 5% buys headroom for the at-open spread
to close at fill on volatile names. On calm names the LIMIT still
fills near the ask — the bigger buffer is a ceiling, not the
expected fill price, since Schwab routes to NBBO.

The buffer's only ceiling is MaxContractPremium ($10/share, =
$1000/contract): any LIMIT computed above that is clamped down,
which keeps the per-contract cap intact even on a runaway ask.
*/
const LimitPriceMultiplier = 1.10

/*
ComputeOpenLimitPrice picks the LIMIT price for a morning open order:
askPrice × LimitPriceMultiplier, rounded to nearest cent, clamped to
MaxContractPremium. Falls back to estimatedPrice (Claude's modeled
premium at pick time) when askPrice is unavailable or zero — typical
pre-open Schwab market data sometimes returns 0 ask on thin contracts.
Returns 0 if neither basis is usable; callers treat 0 as "do not
submit".
*/
func ComputeOpenLimitPrice(askPrice, estimatedPrice float64) float64 {
	basis := askPrice
	if basis <= 0 {
		basis = estimatedPrice
	}
	if basis <= 0 {
		return 0
	}
	limit := math.Round(basis*LimitPriceMultiplier*100) / 100
	if limit > MaxContractPremium {
		limit = MaxContractPremium
	}
	return limit
}

/*
Contract count is now per-entry: the selector returns BasketEntry
values carrying their own quantity, the open path passes it through
into the order leg, the close path reads it back off the open
execution row. There is no fixed contract-count constant any more.
The two governors are MaxContractPremium (per-share ceiling) and
MaxDailyBasketUSD (phase-2 fill target), both defined in selector.go.
*/

/*
OCCSymbol builds the 21-character OCC OSI symbol that Schwab's Trader
API expects for option instruments. Format:

	[6-char root, space-padded right][YYMMDD][C|P][8-digit strike × 1000]

Examples:

	AAPL  240119C00150000   AAPL Jan 19 2024 $150 call
	NVDA  260417P00875500   NVDA Apr 17 2026 $875.50 put

Returns an error on invalid inputs (bad date, negative strike,
root > 6 chars, contract type other than CALL/PUT). The 1000×
multiplier on strike encodes 3 decimal places; we round to nearest
cent first to avoid float drift turning $150.00 into "00149999".
*/
func OCCSymbol(symbol, expiration, contractType string, strike float64) (string, error) {
	root := strings.ToUpper(strings.TrimSpace(symbol))
	if root == "" || len(root) > 6 {
		return "", fmt.Errorf("invalid root %q (must be 1-6 chars)", symbol)
	}
	t, err := time.Parse("2006-01-02", expiration)
	if err != nil {
		return "", fmt.Errorf("invalid expiration %q: %w", expiration, err)
	}
	var letter string
	switch strings.ToUpper(strings.TrimSpace(contractType)) {
	case "CALL":
		letter = "C"
	case "PUT":
		letter = "P"
	default:
		return "", fmt.Errorf("invalid contract type %q (must be CALL or PUT)", contractType)
	}
	if strike <= 0 {
		return "", fmt.Errorf("strike must be positive (got %f)", strike)
	}

	// Round to nearest cent first so 150.00 doesn't drift to 149.999...
	cents := math.Round(strike * 100)
	// OCC encodes strike × 1000, so multiply rounded-cents by 10.
	strikeInt := int64(cents) * 10
	if strikeInt > 99999999 {
		return "", fmt.Errorf("strike %f exceeds OCC 8-digit limit", strike)
	}

	rootPadded := root + strings.Repeat(" ", 6-len(root))
	return fmt.Sprintf("%s%s%s%08d", rootPadded, t.Format("060102"), letter, strikeInt), nil
}

/*
BuildOpenOrderForTrade returns the LIMIT BUY_TO_OPEN order to submit
for the morning auto-execution. Caller passes the qualifying Trade,
its pre-built OCC symbol, the limit price (see ComputeOpenLimitPrice),
and the contract quantity (from the selector's BasketEntry, which can
be > 1 when phase-2 greedy-fills duplicate the same pick).

Returns ErrInvalidOrder when limitPrice is non-positive or quantity
is < 1 — Schwab rejects LIMIT orders without a price, and qty 0
orders are nonsensical.
*/
func BuildOpenOrderForTrade(t *trades.Trade, occSymbol string, limitPrice float64, quantity int) (Order, error) {
	if t == nil {
		return Order{}, ErrInvalidOrder
	}
	if occSymbol == "" {
		return Order{}, fmt.Errorf("missing OCC symbol")
	}
	if limitPrice <= 0 {
		return Order{}, fmt.Errorf("limit price must be positive (got %.2f)", limitPrice)
	}
	if quantity < 1 {
		return Order{}, fmt.Errorf("quantity must be >= 1 (got %d)", quantity)
	}
	return Order{
		OrderType:         "LIMIT",
		Session:           "NORMAL",
		Duration:          "DAY",
		OrderStrategyType: "SINGLE",
		Price:             limitPrice,
		OrderLegCollection: []OrderLeg{{
			Instruction: "BUY_TO_OPEN",
			Quantity:    quantity,
			Instrument: Instrument{
				Symbol:    occSymbol,
				AssetType: "OPTION",
			},
		}},
	}, nil
}

/*
BuildCloseOrderForPosition mirrors BuildOpenOrderForTrade for the
3:55pm mandatory close. SELL_TO_CLOSE the same number of contracts
that the open execution recorded, so callers pass the
Execution.RequestedQuantity (or FilledQuantity once available) from
the open row. Quantity < 1 is rejected to avoid leaving open
contracts at the broker after a malformed close attempt.
*/
func BuildCloseOrderForPosition(p *OpenPosition, quantity int) (Order, error) {
	if p == nil {
		return Order{}, ErrInvalidOrder
	}
	if quantity < 1 {
		return Order{}, fmt.Errorf("quantity must be >= 1 (got %d)", quantity)
	}
	occ, err := OCCSymbol(p.Symbol, p.Expiration, p.ContractType, p.StrikePrice)
	if err != nil {
		return Order{}, fmt.Errorf("rebuild OCC: %w", err)
	}
	return Order{
		OrderType:         "MARKET",
		Session:           "NORMAL",
		Duration:          "DAY",
		OrderStrategyType: "SINGLE",
		OrderLegCollection: []OrderLeg{{
			Instruction: "SELL_TO_CLOSE",
			Quantity:    quantity,
			Instrument: Instrument{
				Symbol:    occ,
				AssetType: "OPTION",
			},
		}},
	}, nil
}
