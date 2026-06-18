package trades

import (
	"sort"
	"strings"
	"time"

	"vibetradez.com/internal/store"
)

/*
Package trades derives completed round trips and performance summaries from
the portfolio decision log and the equity curve. It is the shared source of
truth for "what actually happened": the HTTP layer renders these for the
/closed page, and the portfolio agent reads them through get_track_record so
its decisions are informed by realized outcomes instead of its own prose.

There is no separate closed-trades table. The decision log is walked forward
and each buy is paired with the sells that flatten it (average-cost), one
RoundTrip per completed cycle.
*/

// RoundTrip is one completed round trip: an opening buy (or buys) fully
// flattened by sells, with realized P&L computed on the consumed cost.
type RoundTrip struct {
	Uuid           string
	Symbol         string
	Underlying     string
	AssetType      string // EQUITY | OPTION
	ContractType   string
	Strike         *float64
	Expiration     string
	Quantity       float64
	OpenedDate     string
	ClosedDate     string
	HoldDays       int
	EntryPrice     float64
	ExitPrice      float64
	CostBasis      float64
	Proceeds       float64
	RealizedPnl    float64
	RealizedPct    float64
	OpenRationale  string
	CloseRationale string
}

// DecisionPrice prefers the recorded fill, falling back to the limit price
// (the fire-and-forget LIMIT the order was submitted at).
func DecisionPrice(d store.PortfolioDecisionRow) float64 {
	if d.FillPrice != nil && *d.FillPrice > 0 {
		return *d.FillPrice
	}
	return d.LimitPrice
}

// AssetMult is the per-unit dollar multiplier: option premium is quoted
// per share with a 100-share contract.
func AssetMult(assetType string) float64 {
	if assetType == "OPTION" {
		return 100
	}
	return 1
}

const qtyEpsilon = 1e-6

/*
traded reports whether a decision actually executed and so should feed
round-trip P&L. An order that reached a terminal NON-filled state
(canceled / rejected / expired / replaced) never traded, and folding it in
would fabricate a round trip — at its unfilled limit price, since
DecisionPrice falls back to LimitPrice when there's no recorded fill.
Filled, still-working, and legacy-blank (pre-status-tracking) rows are
kept. Schwab has no partial-fill state — a partial stays WORKING and only
flips to FILLED once the whole order completes — so a kept row never
overstates quantity.
*/
func traded(d store.PortfolioDecisionRow) bool {
	switch strings.ToUpper(strings.TrimSpace(d.Status)) {
	case "CANCELED", "CANCELLED", "REJECTED", "EXPIRED", "REPLACED":
		return false
	}
	return true
}

// acc accumulates an open round trip until a sell flattens it.
type acc struct {
	openQty      float64
	openCost     float64
	soldQty      float64
	proceeds     float64
	costConsumed float64
	openDate     string
	closeDate    string
	openReason   string
	closeReason  string
	uuid         string
	assetType    string
	underlying   string
	contractType string
	expiration   string
	strike       *float64
}

/*
Derive walks the decision log forward, pairing buys with the sells that
flatten them (average-cost), and emits one RoundTrip per completed cycle.
Sells with no matching open lot are ignored (no entry on record to pair
against). Result is newest-close first.
*/
func Derive(decisions []store.PortfolioDecisionRow) []RoundTrip {
	open := map[string]*acc{}
	var out []RoundTrip

	for _, d := range decisions {
		if !traded(d) {
			continue // canceled / rejected / expired / replaced: never executed
		}
		switch d.Action {
		case "buy_equity", "buy_option":
			a := open[d.Symbol]
			if a == nil {
				a = &acc{}
				open[d.Symbol] = a
			}
			if a.openQty <= qtyEpsilon {
				// start of a fresh cycle: capture entry context
				a.openDate = d.Date
				a.openReason = d.Rationale
				a.uuid = d.Uuid
				a.assetType = d.AssetType
				a.underlying = d.Underlying
				a.contractType = d.ContractType
				a.expiration = d.Expiration
				a.strike = d.Strike
				a.soldQty, a.proceeds, a.costConsumed = 0, 0, 0
			}
			a.openQty += d.Quantity
			a.openCost += d.Quantity * DecisionPrice(d) * AssetMult(d.AssetType)

		case "sell_equity", "sell_option":
			a := open[d.Symbol]
			if a == nil || a.openQty <= qtyEpsilon {
				continue // no open lot to close against
			}
			sellQty := d.Quantity
			if sellQty > a.openQty {
				sellQty = a.openQty
			}
			avgCost := a.openCost / a.openQty
			costPortion := avgCost * sellQty
			a.openQty -= sellQty
			a.openCost -= costPortion
			a.soldQty += sellQty
			a.proceeds += sellQty * DecisionPrice(d) * AssetMult(d.AssetType)
			a.costConsumed += costPortion
			a.closeReason = d.Rationale
			a.closeDate = d.Date
			if a.openQty <= qtyEpsilon {
				out = append(out, a.build(d.Symbol))
				delete(open, d.Symbol)
			}
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].ClosedDate > out[j].ClosedDate })
	return out
}

func (a *acc) build(symbol string) RoundTrip {
	units := a.soldQty * AssetMult(a.assetType)
	entry, exit := 0.0, 0.0
	if units > 0 {
		entry = a.costConsumed / units
		exit = a.proceeds / units
	}
	realized := a.proceeds - a.costConsumed
	pct := 0.0
	if a.costConsumed > 0 {
		pct = realized / a.costConsumed * 100
	}
	return RoundTrip{
		Uuid:           a.uuid,
		Symbol:         symbol,
		Underlying:     a.underlying,
		AssetType:      a.assetType,
		ContractType:   a.contractType,
		Strike:         a.strike,
		Expiration:     a.expiration,
		Quantity:       a.soldQty,
		OpenedDate:     a.openDate,
		ClosedDate:     a.closeDate,
		HoldDays:       daysBetween(a.openDate, a.closeDate),
		EntryPrice:     entry,
		ExitPrice:      exit,
		CostBasis:      a.costConsumed,
		Proceeds:       a.proceeds,
		RealizedPnl:    realized,
		RealizedPct:    pct,
		OpenRationale:  a.openReason,
		CloseRationale: a.closeReason,
	}
}

// daysBetween is whole calendar days from open to close; 0 when either
// date fails to parse or close precedes open.
func daysBetween(open, close string) int {
	o, err1 := time.Parse("2006-01-02", open)
	c, err2 := time.Parse("2006-01-02", close)
	if err1 != nil || err2 != nil {
		return 0
	}
	d := int(c.Sub(o).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

// Stats summarizes one bucket of round trips into the numbers a trader
// reviews: hit rate and the win/loss size asymmetry that decides expectancy.
type Stats struct {
	Trades      int
	Wins        int
	Losses      int
	WinRatePct  float64
	TotalPnl    float64
	AvgWinPnl   float64
	AvgLossPnl  float64
	AvgHoldDays float64
	BiggestWin  float64
	BiggestLoss float64
}

// Summarize computes Stats over a set of round trips. A zero-P&L trip
// counts as a loss (it paid the spread for nothing).
func Summarize(trips []RoundTrip) Stats {
	var s Stats
	s.Trades = len(trips)
	if s.Trades == 0 {
		return s
	}
	var winSum, lossSum, holdSum float64
	for _, t := range trips {
		s.TotalPnl += t.RealizedPnl
		holdSum += float64(t.HoldDays)
		if t.RealizedPnl > 0 {
			s.Wins++
			winSum += t.RealizedPnl
			if t.RealizedPnl > s.BiggestWin {
				s.BiggestWin = t.RealizedPnl
			}
		} else {
			s.Losses++
			lossSum += t.RealizedPnl
			if t.RealizedPnl < s.BiggestLoss {
				s.BiggestLoss = t.RealizedPnl
			}
		}
	}
	s.WinRatePct = float64(s.Wins) / float64(s.Trades) * 100
	if s.Wins > 0 {
		s.AvgWinPnl = winSum / float64(s.Wins)
	}
	if s.Losses > 0 {
		s.AvgLossPnl = lossSum / float64(s.Losses)
	}
	s.AvgHoldDays = holdSum / float64(s.Trades)
	return s
}

// CurveSummary is the benchmark race: the account's return since inception
// against buy-and-hold SPY over the same window, plus the deepest drawdown.
type CurveSummary struct {
	Days             int
	FirstDate        string
	LastDate         string
	StartEquity      float64
	CurrentEquity    float64
	AccountReturnPct float64
	SpyReturnPct     float64
	VsSpyPct         float64
	MaxDrawdownPct   float64
}

// SummarizeCurve computes the benchmark race from the equity-curve ledger.
// SPY gaps (a 0 close on days the quote failed) are skipped, so the SPY
// return spans the first and last days that actually have a benchmark mark.
func SummarizeCurve(points []store.EquityCurvePoint) CurveSummary {
	var c CurveSummary
	c.Days = len(points)
	if len(points) == 0 {
		return c
	}
	c.FirstDate = points[0].Date
	c.LastDate = points[len(points)-1].Date
	c.StartEquity = points[0].AccountEquity
	c.CurrentEquity = points[len(points)-1].AccountEquity
	if c.StartEquity > 0 {
		c.AccountReturnPct = (c.CurrentEquity/c.StartEquity - 1) * 100
	}

	var firstSpy, lastSpy float64
	peak, maxDD := 0.0, 0.0
	for _, p := range points {
		if p.SPYClose > 0 {
			if firstSpy == 0 {
				firstSpy = p.SPYClose
			}
			lastSpy = p.SPYClose
		}
		if p.AccountEquity > peak {
			peak = p.AccountEquity
		}
		if peak > 0 {
			dd := (peak - p.AccountEquity) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	if firstSpy > 0 && lastSpy > 0 {
		c.SpyReturnPct = (lastSpy/firstSpy - 1) * 100
	}
	c.VsSpyPct = c.AccountReturnPct - c.SpyReturnPct
	c.MaxDrawdownPct = maxDD
	return c
}
