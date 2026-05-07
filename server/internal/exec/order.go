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
fallback estimate) when constructing the open LIMIT order. 1.05 = 5%
above ask. Schwab rejects MARKET option orders submitted before the
9:30 ET open, so the morning cron uses LIMIT with this buffer to be
accepted pre-market and fill at or near the open. The buffer is
sized so a typical pre-open quote spread closes at fill without
exceeding MaxContractPremium for any contract that already passed the
selector cap.
*/
const LimitPriceMultiplier = 1.05

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
MaxContracts is the per-trade contract count cap. Hardcoded at 1 so
no caller can ever submit a multi-contract order — even if a future
bug computed N>1, BuildOpenOrder would panic. Modify here AND in the
task plan + email templates if this ever needs to change.
*/
const MaxContracts = 1

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
for the morning auto-execution. Hardcodes MaxContracts (1) and
BUY_TO_OPEN; caller passes the rank-1 Trade, its pre-built OCC symbol,
and the limit price (see ComputeOpenLimitPrice).

Returns ErrInvalidOrder when limitPrice is non-positive — Schwab
rejects LIMIT orders without a price, so the caller MUST resolve a
price (live ask or fallback estimate) before invoking this.
*/
func BuildOpenOrderForTrade(t *trades.Trade, occSymbol string, limitPrice float64) (Order, error) {
	if t == nil {
		return Order{}, ErrInvalidOrder
	}
	if occSymbol == "" {
		return Order{}, fmt.Errorf("missing OCC symbol")
	}
	if limitPrice <= 0 {
		return Order{}, fmt.Errorf("limit price must be positive (got %.2f)", limitPrice)
	}
	return Order{
		OrderType:         "LIMIT",
		Session:           "NORMAL",
		Duration:          "DAY",
		OrderStrategyType: "SINGLE",
		Price:             limitPrice,
		OrderLegCollection: []OrderLeg{{
			Instruction: "BUY_TO_OPEN",
			Quantity:    MaxContracts,
			Instrument: Instrument{
				Symbol:    occSymbol,
				AssetType: "OPTION",
			},
		}},
	}, nil
}

/*
BuildCloseOrderForPosition mirrors BuildOpenOrderForTrade for the
3:55pm mandatory close. Same hardcoded contract count + market order,
only the instruction differs (SELL_TO_CLOSE). Caller passes an
OpenPosition (joined trade + execution row) so the OCC symbol is
rebuilt from the trade's contract spec.
*/
func BuildCloseOrderForPosition(p *OpenPosition) (Order, error) {
	if p == nil {
		return Order{}, ErrInvalidOrder
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
			Quantity:    MaxContracts,
			Instrument: Instrument{
				Symbol:    occ,
				AssetType: "OPTION",
			},
		}},
	}, nil
}
