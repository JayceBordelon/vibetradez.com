package exec

import (
	"fmt"
	"math"
	"strings"
	"time"

	"vibetradez.com/internal/trades"
)

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
BuildOpenOrderForTrade returns the Order to submit for the morning
auto-execution. Hardcodes MaxContracts (1) and BUY_TO_OPEN, these are
invariants, not parameters. Caller passes the rank-1 Trade plus its
pre-built OCC symbol.
*/
func BuildOpenOrderForTrade(t *trades.Trade, occSymbol string) (Order, error) {
	if t == nil {
		return Order{}, ErrInvalidOrder
	}
	if occSymbol == "" {
		return Order{}, fmt.Errorf("missing OCC symbol")
	}
	return Order{
		OrderType:         "MARKET",
		Session:           "NORMAL",
		Duration:          "DAY",
		OrderStrategyType: "SINGLE",
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
