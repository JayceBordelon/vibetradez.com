package exec

import (
	"fmt"
	"math"
	"strings"
	"time"

	"vibetradez.com/internal/trades"
)

/*
MaxContractPremium is the per-contract premium ceiling enforced at the
broker-facing order-submission layer. Premium is quoted per share,
options are 100 shares, so $10/share = $1,000 of capital exposure on
a single contract. Hard upper bound on any single order this codebase
submits to Schwab.

The agent tool layer (internal/execagent/tools.go) enforces the same
cap as MaxToolPremiumPerShare so a buggy or compromised model can't
talk the broker out of an over-cap order; this constant is the
defense-in-depth re-check on the way out of Service.PlaceBuyToOpenAgent.
*/
const MaxContractPremium = 10.00

/*
LimitPriceMultiplier is the buffer the execagent's prompt recommends
on top of the live ask when constructing the open LIMIT (e.g.,
limit_price ≈ 1.10 × ask). The constant is no longer enforced in code
— the agent picks the limit_price directly via the place_options_order
tool — but it's kept here as a documented anchor the prompt references
so the wider system has a single source of truth for the convention.

The hard ceiling on any single LIMIT is MaxContractPremium
($10/share = $1000/contract), enforced by the tool layer in
internal/execagent/tools.go (MaxToolPremiumPerShare) and re-validated
in Service.PlaceBuyToOpenAgent below.
*/
const LimitPriceMultiplier = 1.10

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
and the contract quantity (always 1 under the top-3-only selector,
but kept as a parameter for the order/execution wiring and for
backwards compatibility with historical quantity > 1 rows).

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
