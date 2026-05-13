package exec

import "vibetradez.com/internal/trades"

/*
MaxContractPremium is the per-contract premium ceiling. Premium is
quoted per-share, options are 100 shares, so $5 premium = $500 of
capital exposure on a single contract. Hard upper bound on any
single basket pick; the daily basket cap stacks on top of this.

Paper mode honors this cap so paper P&L tracking mirrors what live
would have actually done.
*/
const MaxContractPremium = 5.00

/*
MaxDailyBasketUSD is the hard daily cap on cumulative open exposure
across the entire top-3 basket. Sized at 2× MaxContractPremium ×
100 so two fully-priced contracts always fit, three usually do, and
worst-case daily exposure is bounded at $1,000 regardless of how
the picks land. Rank-1 has first claim on the budget but plays by
the same per-contract and basket rules as ranks 2 and 3.
*/
const MaxDailyBasketUSD = 1000.00

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
    or is non-positive (no usable Claude estimate)
  - include each remaining pick whose estimated cost (premium × 100)
    fits in the remaining basket budget
  - skip-and-continue when a pick is too big — so a $700 rank-2
    that doesn't fit in the remaining budget after rank-1 lands does
    NOT block a smaller rank-3 from filling

Rank-1 always gets first crack at the budget. Returns an empty slice
when nothing qualifies. Cost accounting uses Claude's modeled
premium; the service does a second cash-availability check at order-
place time against Schwab's reported availableFundsNonMarginableTrade
and the actual ask-derived limit price.

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
