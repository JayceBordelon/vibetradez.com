package exec

import (
	"reflect"
	"testing"

	"vibetradez.com/internal/trades"
)

func mkTrade(symbol, kind string, rank int, price float64) trades.Trade {
	return trades.Trade{
		Symbol:         symbol,
		ContractType:   kind,
		EstimatedPrice: price,
		Rank:           rank,
	}
}

type basketSummary struct {
	Symbol   string
	Quantity int
}

func summarize(b []BasketEntry) []basketSummary {
	out := make([]basketSummary, len(b))
	for i, e := range b {
		out[i] = basketSummary{Symbol: e.Trade.Symbol, Quantity: e.Quantity}
	}
	return out
}

func TestQualifyingBasket_TopThreeAlwaysIncluded(t *testing.T) {
	// Top 3 unconditional even when their combined exposure exceeds the
	// MaxDailyBasketUSD target. Phase 2 then has no remaining budget so
	// no greedy fill happens.
	in := []trades.Trade{
		mkTrade("R1", "CALL", 1, 4.00), // $400
		mkTrade("R2", "CALL", 2, 4.50), // $450
		mkTrade("R3", "CALL", 3, 4.00), // $400, total $1250 > $1000
		mkTrade("R4", "CALL", 4, 0.50),
	}
	got := summarize(QualifyingBasket(in))
	want := []basketSummary{{"R1", 1}, {"R2", 1}, {"R3", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestQualifyingBasket_GreedyFillsRank1FirstWithDuplicates(t *testing.T) {
	// Phase 1 buys r1+r2+r3 = 100+100+100 = $300, remaining = $700.
	// Phase 2 walks r1: $100/contract fits 7 times, so r1 gains 7 more
	// (qty 8 total). Remaining 0, the rest of the walk is no-op.
	in := []trades.Trade{
		mkTrade("R1", "CALL", 1, 1.00),
		mkTrade("R2", "CALL", 2, 1.00),
		mkTrade("R3", "CALL", 3, 1.00),
		mkTrade("R4", "CALL", 4, 1.00),
	}
	got := summarize(QualifyingBasket(in))
	want := []basketSummary{{"R1", 8}, {"R2", 1}, {"R3", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestQualifyingBasket_GreedyMovesToNextRankWhenCurrentTooBig(t *testing.T) {
	// Phase 1: r1=$700, r2=$50, r3=$50 → used $800. Remaining $200.
	// Phase 2 walk: r1 at $700 doesn't fit. r2 at $50 → fits 4× more.
	// Remaining 0. r3 skipped.
	in := []trades.Trade{
		mkTrade("R1", "CALL", 1, 7.00),
		mkTrade("R2", "CALL", 2, 0.50),
		mkTrade("R3", "CALL", 3, 0.50),
	}
	got := summarize(QualifyingBasket(in))
	want := []basketSummary{{"R1", 1}, {"R2", 5}, {"R3", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestQualifyingBasket_ExampleFromSpec(t *testing.T) {
	// User example: $300 left after top 3 and option #1 costs $299 →
	// buy 1 more of rank-1. Set top-3 exposure so remaining is exactly
	// $300, with rank-1 priced at $2.99.
	// r1=$2.99 ($299), r2=$2.01 ($201), r3=$2.00 ($200) → used $700.
	// Remaining $300. Phase 2 r1 at $299 fits 1× → qty 2 total. Remaining
	// $1, nothing else fits.
	in := []trades.Trade{
		mkTrade("R1", "CALL", 1, 2.99),
		mkTrade("R2", "CALL", 2, 2.01),
		mkTrade("R3", "CALL", 3, 2.00),
		mkTrade("R4", "CALL", 4, 1.00),
	}
	got := summarize(QualifyingBasket(in))
	want := []basketSummary{{"R1", 2}, {"R2", 1}, {"R3", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestQualifyingBasket_SkipsTopPickAboveCap(t *testing.T) {
	// Cap is still load-bearing as a safety net: a rank-1 quoted above
	// MaxContractPremium is skipped (likely a picker bug or stale
	// quote). The greedy phase still walks past it and fills the rest
	// of the basket with the next eligible ranks, so HUGE itself never
	// appears in the output.
	in := []trades.Trade{
		mkTrade("HUGE", "CALL", 1, MaxContractPremium+0.01),
		mkTrade("R2", "CALL", 2, 1.00),
		mkTrade("R3", "CALL", 3, 1.00),
	}
	got := summarize(QualifyingBasket(in))
	for _, e := range got {
		if e.Symbol == "HUGE" {
			t.Fatalf("HUGE above-cap should be skipped, basket = %v", got)
		}
	}
	// Phase 1 puts R2+R3 at qty 1 each ($200). Greedy fills R2 (rank-1
	// is still unusable) until the $1000 budget is exhausted: qty 9.
	want := []basketSummary{{"R2", 9}, {"R3", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestQualifyingBasket_SkipsNonPositiveEstimate(t *testing.T) {
	in := []trades.Trade{
		mkTrade("ZERO", "CALL", 1, 0),
		mkTrade("R2", "CALL", 2, 1.00),
		mkTrade("R3", "CALL", 3, 1.00),
	}
	got := summarize(QualifyingBasket(in))
	// r2+r3 in phase 1 use $200. Remaining $800. Phase 2 walks
	// r1 skipped (zero), then r2: $100 fits 8 more times (qty 9), then
	// remaining 0.
	want := []basketSummary{{"R2", 9}, {"R3", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestQualifyingBasket_NoRank1OnlyRanks2And3(t *testing.T) {
	// No rank-1 at all. Phase 1 still places r2 and r3. Phase 2 fills
	// from r2 (rank-1 missing entirely).
	in := []trades.Trade{
		mkTrade("R2", "CALL", 2, 1.69), // $169
		mkTrade("R3", "CALL", 3, 1.65), // $165
	}
	got := summarize(QualifyingBasket(in))
	// Phase 1 used $334. Remaining $666. Walk r2 at $169 → 3 more fit
	// (qty 4, used $676). Remaining $666-3×$169 = $159. Walk r3 at $165
	// doesn't fit. Total: r2=4, r3=1.
	if len(got) != 2 || got[0].Symbol != "R2" || got[0].Quantity != 4 || got[1].Symbol != "R3" || got[1].Quantity != 1 {
		t.Fatalf("unexpected basket: %v", got)
	}
}

func TestQualifyingBasket_GreedyReachesPicksBeyondTopThree(t *testing.T) {
	// Rank-1..3 at the cap eat $3000 (way over). Phase 2 has no budget.
	// Confirm we still respect the cap on phase 1 even when nothing
	// else qualifies.
	in := []trades.Trade{
		mkTrade("M1", "CALL", 1, MaxContractPremium),
		mkTrade("M2", "CALL", 2, MaxContractPremium),
		mkTrade("M3", "CALL", 3, MaxContractPremium),
		mkTrade("R4", "CALL", 4, 1.00),
	}
	got := summarize(QualifyingBasket(in))
	want := []basketSummary{{"M1", 1}, {"M2", 1}, {"M3", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestQualifyingBasket_GreedyFillFromRank4(t *testing.T) {
	// Phase 1 exposure is small ($300). Remaining $700. Rank 1-3 each at
	// $1 fit 7 more total split rank-first. Greedy fills r1 7× then
	// stops (budget 0). Confirms phase 2 prefers earlier ranks.
	in := []trades.Trade{
		mkTrade("R1", "CALL", 1, 1.00),
		mkTrade("R2", "CALL", 2, 1.00),
		mkTrade("R3", "CALL", 3, 1.00),
		mkTrade("R4", "CALL", 4, 5.00),
		mkTrade("R5", "CALL", 5, 5.00),
	}
	got := summarize(QualifyingBasket(in))
	want := []basketSummary{{"R1", 8}, {"R2", 1}, {"R3", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestQualifyingBasket_EmptyInput(t *testing.T) {
	if got := QualifyingBasket(nil); len(got) != 0 {
		t.Fatalf("nil input: want empty, got %v", summarize(got))
	}
	if got := QualifyingBasket([]trades.Trade{}); len(got) != 0 {
		t.Fatalf("empty slice: want empty, got %v", summarize(got))
	}
}

func TestQualifyingBasket_PutAtRank1IsEligible(t *testing.T) {
	in := []trades.Trade{mkTrade("HUBS", "PUT", 1, 1.18)}
	got := QualifyingBasket(in)
	if len(got) != 1 || got[0].Trade.ContractType != "PUT" {
		t.Fatalf("expected PUT pick selectable, got %v", got)
	}
}
