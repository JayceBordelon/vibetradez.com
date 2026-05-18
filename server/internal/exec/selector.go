package exec

import "vibetradez.com/internal/trades"

/*
MaxContractPremium is the per-contract premium ceiling. Premium is
quoted per-share, options are 100 shares, so $10 premium = $1,000 of
capital exposure on a single contract. Hard upper bound on any
single contract in the basket. The two-phase selector below honors
this on every contract it admits, including the guaranteed top 3.

Paper mode honors this cap too so paper P&L tracking mirrors what
live would have actually done.
*/
const MaxContractPremium = 10.00

/*
MaxDailyBasketUSD is the soft target the greedy-fill phase aims at.
Phase 1 (guaranteed top 3) ignores this entirely, so on a day where
ranks 1+2+3 cost more than $1,000 the basket overshoots by design.
Phase 2 only fills additional contracts when this many dollars of
exposure are still unused after phase 1.
*/
const MaxDailyBasketUSD = 1000.00

/*
GuaranteedBasketRank is the phase-1 cutoff: ranks 1..N each get a
single contract submitted unconditionally (modulo the per-contract
cap as a sanity gate) regardless of basket budget.
*/
const GuaranteedBasketRank = 3

/*
GreedyFillMaxRank is the phase-2 cutoff: after phase 1 lands the top
GuaranteedBasketRank picks, the greedy fill walks ranks 1..N adding
more contracts (duplicates allowed) until the remaining budget is
exhausted or every rank's contract is too big to fit. 10 matches the
full size of the daily picker output.
*/
const GreedyFillMaxRank = 10

/*
BasketEntry is one (trade, quantity) pair the executor will submit
as a single quantity=N order. Duplicates are not allowed in the
returned slice: if the selector wants two contracts of rank-1, that
shows up as one BasketEntry with Quantity=2, not as two entries.
*/
type BasketEntry struct {
	Trade    trades.Trade
	Quantity int
}

/*
QualifyingBasket returns the basket of (trade, quantity) entries the
auto-executor will submit today, in rank-ascending order.

Two phases:

Phase 1 (guaranteed top 3): for each rank in 1..GuaranteedBasketRank,
if the pick exists and 0 < EstimatedPrice <= MaxContractPremium, add
one contract to the basket. Phase 1 ignores MaxDailyBasketUSD on
purpose: the top three picks are conviction-driven and must fire.
The cap stays as a runaway-pick safety filter; a picker-side validator
or operator alert is the right tool for "Claude returned a $50/share
contract", not a silent basket skip.

Phase 2 (greedy fill toward MaxDailyBasketUSD): compute remaining =
MaxDailyBasketUSD - phase-1 exposure, clamped to 0. Walk ranks
1..GreedyFillMaxRank; for each rank, while another contract of that
rank's pick fits in the remaining budget AND is below the per-contract
cap, add one more contract and decrement remaining. The walk prefers
earlier ranks: rank-1 fills up first, then rank-2, etc. The same
trade can therefore end up with quantity > 1 in the returned slice.

Score is intentionally NOT a gate. Rank order is the only ordering.
*/
func QualifyingBasket(picks []trades.Trade) []BasketEntry {
	if len(picks) == 0 {
		return nil
	}

	byRank := make(map[int]*trades.Trade, len(picks))
	for i := range picks {
		t := &picks[i]
		if t.Rank < 1 || t.Rank > GreedyFillMaxRank {
			continue
		}
		// First-seen wins if the same rank shows up twice (defensive; the
		// upstream picker enforces unique ranks).
		if _, dup := byRank[t.Rank]; dup {
			continue
		}
		byRank[t.Rank] = t
	}

	qty := make(map[int]int, GreedyFillMaxRank)

	// Phase 1: unconditional top GuaranteedBasketRank.
	for r := 1; r <= GuaranteedBasketRank; r++ {
		t := byRank[r]
		if t == nil {
			continue
		}
		if !capEligible(t.EstimatedPrice) {
			continue
		}
		qty[r] = 1
	}

	// Phase 2: greedy fill toward MaxDailyBasketUSD.
	used := 0.0
	for r, n := range qty {
		used += float64(n) * byRank[r].EstimatedPrice * 100
	}
	remaining := MaxDailyBasketUSD - used
	if remaining < 0 {
		remaining = 0
	}

	for r := 1; r <= GreedyFillMaxRank; r++ {
		t := byRank[r]
		if t == nil {
			continue
		}
		if !capEligible(t.EstimatedPrice) {
			continue
		}
		costPerContract := t.EstimatedPrice * 100
		// Float comparison is fine here: costPerContract is always an
		// exact dollar value in the cents (premium * 100), and remaining
		// is decremented by the same exact value, so no drift accumulates.
		for costPerContract <= remaining {
			qty[r]++
			remaining -= costPerContract
		}
	}

	out := make([]BasketEntry, 0, len(qty))
	for r := 1; r <= GreedyFillMaxRank; r++ {
		n, ok := qty[r]
		if !ok || n <= 0 {
			continue
		}
		out = append(out, BasketEntry{Trade: *byRank[r], Quantity: n})
	}
	return out
}

func capEligible(estimatedPrice float64) bool {
	return estimatedPrice > 0 && estimatedPrice <= MaxContractPremium
}
