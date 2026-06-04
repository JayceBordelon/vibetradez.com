package exec

import (
	"fmt"
	"math"
	"strings"
	"time"
)

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
DecodeOCCSymbol is the exported inverse of OCCSymbol: it recovers the
(underlying, expiration, contractType, strike) a 21-char OCC option symbol
encodes. The v2 portfolio manager's read path uses it to turn a held
option's broker symbol back into its contract spec (strike / expiration /
DTE) for the snapshot it shows the agent.
*/
func DecodeOCCSymbol(occ string) (symbol, expiration, contractType string, strike float64, err error) {
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
		contractType = "CALL"
	case 'P':
		contractType = "PUT"
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
	return symbol, expiration, contractType, strike, nil
}

/*
BuildEquityOrder returns a single-leg equity LIMIT order. side is "BUY"
or "SELL" (Schwab's plain equity instructions, distinct from the options
BUY_TO_OPEN / SELL_TO_CLOSE). Used by the v2 portfolio manager; the v1
options path never calls it. Quantity is whole shares.

Returns ErrInvalidOrder-style errors on a bad side, non-positive limit,
or quantity < 1. LIMIT (not MARKET) so the portfolio agent's chosen price
is always honored and a fat-finger mark can't fill at any price.
*/
func BuildEquityOrder(symbol, side string, quantity int, limitPrice float64) (Order, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return Order{}, fmt.Errorf("missing equity symbol")
	}
	instruction := strings.ToUpper(strings.TrimSpace(side))
	if instruction != "BUY" && instruction != "SELL" {
		return Order{}, fmt.Errorf("invalid equity side %q (must be BUY or SELL)", side)
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
			Instruction: instruction,
			Quantity:    quantity,
			Instrument: Instrument{
				Symbol:    sym,
				AssetType: "EQUITY",
			},
		}},
	}, nil
}

/*
BuildOptionLimitOrder returns a single-leg option LIMIT order against a
pre-built OCC symbol, for either side. instruction is "BUY_TO_OPEN" or
"SELL_TO_CLOSE". Unlike BuildOpenOrderForTrade (which derives the OCC from
a trades.Trade) and BuildCloseOrderForPosition (which submits a MARKET
close), this is the symbol-in, LIMIT-out builder the v2 portfolio manager
uses, since the agent picks the limit price for both opens and closes.
*/
func BuildOptionLimitOrder(occSymbol, instruction string, quantity int, limitPrice float64) (Order, error) {
	occ := strings.ToUpper(strings.TrimSpace(occSymbol))
	if occ == "" {
		return Order{}, fmt.Errorf("missing OCC symbol")
	}
	ins := strings.ToUpper(strings.TrimSpace(instruction))
	if ins != "BUY_TO_OPEN" && ins != "SELL_TO_CLOSE" {
		return Order{}, fmt.Errorf("invalid option instruction %q (must be BUY_TO_OPEN or SELL_TO_CLOSE)", instruction)
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
			Instruction: ins,
			Quantity:    quantity,
			Instrument: Instrument{
				Symbol:    occ,
				AssetType: "OPTION",
			},
		}},
	}, nil
}
