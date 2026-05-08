package exec

import "vibetradez.com/internal/trades"

/*
MaxContractPremium is the hard upper bound on a single contract's
premium for live-mode auto-execution. The cap is enforced uniformly
to keep the live failure mode contained: worst-case daily loss is
MaxContractPremium * 100 = $500 per filled contract. Premium quoted
in this codebase is per-share, options are 100 shares, so $5 premium
= $500 of capital exposure. Adjust both this constant and the prompt
language together if the cap ever changes.

Paper mode also enforces the cap so paper P&L tracking mirrors what
live would have actually done.
*/
const MaxContractPremium = 5.00

/*
MaxDailyBasketUSD is the hard daily cap on cumulative open exposure
across the basket of qualifying picks. Sized so the worst-case daily
loss across the full basket is bounded by this number, mirroring the
single-contract cap intent at the basket level. With MaxBasketRank=3
and MaxContractPremium=$5 per share, an unbounded basket could risk
3*$500=$1500/day; this constant reins that in to a single $500/day
ceiling regardless of how many of the top-3 fit.
*/
const MaxDailyBasketUSD = 500.00

/*
MaxBasketRank caps which ranks are eligible for the basket. Only
picks ranked 1..MaxBasketRank are considered, in rank order. Rank-4
and below are ignored even if budget remains, because the executor's
fairness story is "execute Claude's top conviction picks" — the tail
is for the published email, not for capital.
*/
const MaxBasketRank = 3

/*
QualifyingPicks returns the basket of trades to auto-execute today,
in submission order (rank-ascending). The selector walks ranks
1..MaxBasketRank greedily:

  - reject any pick whose modeled premium exceeds MaxContractPremium
    (per-contract cap unchanged from the single-pick era)
  - include each remaining pick whose estimated cost (premium × 100)
    fits in the remaining basket budget
  - skip-and-continue when a pick is too big — a $400 rank-1 plus a
    $90 rank-3 fills better than stopping at the first oversized pick

Returns an empty slice when nothing qualifies (no rank-1, rank-1
exceeds the per-contract cap and no eligible smaller pick remains,
etc). Cost accounting here uses Claude's modeled premium; the
service does a second cash-availability check at order-place time
against Schwab's reported availableFundsNonMarginableTrade and the
actual ask-derived limit price, so a quote drift between morning
analysis and 9:25 cron submission can still skip a contract.

Score is intentionally NOT a gate. Rank order is the only signal.
*/
func QualifyingPicks(picks []trades.Trade) []trades.Trade {
	if len(picks) == 0 {
		return nil
	}
	byRank := make(map[int]*trades.Trade, MaxBasketRank)
	for i := range picks {
		t := &picks[i]
		if t.Rank < 1 || t.Rank > MaxBasketRank {
			continue
		}
		byRank[t.Rank] = t
	}
	out := make([]trades.Trade, 0, MaxBasketRank)
	remainingUSD := MaxDailyBasketUSD
	for r := 1; r <= MaxBasketRank; r++ {
		t := byRank[r]
		if t == nil {
			continue
		}
		if t.EstimatedPrice <= 0 || t.EstimatedPrice > MaxContractPremium {
			continue
		}
		costUSD := t.EstimatedPrice * 100
		if costUSD > remainingUSD {
			continue
		}
		out = append(out, *t)
		remainingUSD -= costUSD
	}
	return out
}
