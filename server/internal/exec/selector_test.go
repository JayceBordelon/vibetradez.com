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

func symbols(picks []trades.Trade) []string {
	out := make([]string, len(picks))
	for i, p := range picks {
		out[i] = p.Symbol
	}
	return out
}

func TestQualifyingPicks_AllThreeFit(t *testing.T) {
	in := []trades.Trade{
		mkTrade("AKAM", "CALL", 1, 1.25),
		mkTrade("RKLB", "CALL", 2, 1.69),
		mkTrade("IREN", "CALL", 3, 1.65),
		mkTrade("NET", "PUT", 4, 1.55), // ignored — beyond top-3
	}
	got := QualifyingPicks(in)
	if want := []string{"AKAM", "RKLB", "IREN"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_StopsAtMaxBasketRank(t *testing.T) {
	in := []trades.Trade{
		mkTrade("AKAM", "CALL", 1, 0.50), // $50
		mkTrade("RKLB", "CALL", 2, 0.50), // $50
		mkTrade("IREN", "CALL", 3, 0.50), // $50, total $150
		mkTrade("NET", "CALL", 4, 0.50),  // would fit budget but rank > 3
	}
	got := QualifyingPicks(in)
	if want := []string{"AKAM", "RKLB", "IREN"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_GreedyRankOrderSkipsTooBig(t *testing.T) {
	// Rank-1 ($350) + Rank-2 ($450) = $800 leaves $200 for rank-3.
	// Rank-3 at $4.99 ($499) doesn't fit → skipped. Drop in a smaller
	// pick at rank-3 and it would land. Confirms skip-and-continue
	// stops at rank-3 once the budget can't fit it.
	in := []trades.Trade{
		mkTrade("R1", "CALL", 1, 3.50),
		mkTrade("R2", "CALL", 2, 4.50),
		mkTrade("BIG3", "CALL", 3, 4.99),
	}
	got := QualifyingPicks(in)
	if want := []string{"R1", "R2"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_SkipsAndContinuesWhenMidPickTooBig(t *testing.T) {
	// Rank-1 ($350). Remaining $650. Rank-2 at $4.99 ($499) fits and
	// is taken. Rank-3 at $0.90 ($90) fits the remaining $151 and is
	// taken too. Confirms greedy walk takes all three when they fit.
	in := []trades.Trade{
		mkTrade("R1", "CALL", 1, 3.50),
		mkTrade("R2", "CALL", 2, 4.99),
		mkTrade("R3", "CALL", 3, 0.90),
	}
	got := QualifyingPicks(in)
	if want := []string{"R1", "R2", "R3"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_SkipsRank2ButKeepsRank3WhenRank3Fits(t *testing.T) {
	// Rank-1 at $5 cap eats $500 of the $1000 basket. Rank-2 at $5 cap
	// ($500) would just barely fit ($500 remaining), but if rank-2 is
	// $5.01 it busts the per-contract cap entirely — skipped at the
	// premium gate, not the budget gate. Rank-3 still fits.
	in := []trades.Trade{
		mkTrade("R1", "CALL", 1, MaxContractPremium),
		mkTrade("HUGE2", "CALL", 2, MaxContractPremium+0.01),
		mkTrade("R3", "CALL", 3, 1.00),
	}
	got := QualifyingPicks(in)
	if want := []string{"R1", "R3"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_SkipsAbovePerContractCap(t *testing.T) {
	in := []trades.Trade{
		mkTrade("HUGE", "CALL", 1, MaxContractPremium+0.01),
		mkTrade("OK1", "CALL", 2, 1.00),
		mkTrade("OK2", "CALL", 3, 1.00),
	}
	got := QualifyingPicks(in)
	if want := []string{"OK1", "OK2"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_BasketBudgetExhausted(t *testing.T) {
	// Rank-1 + Rank-2 at the per-contract cap eat the whole $1000
	// basket. Even a tiny rank-3 is skipped because the budget is
	// exactly zero.
	in := []trades.Trade{
		mkTrade("MAX1", "CALL", 1, MaxContractPremium),
		mkTrade("MAX2", "CALL", 2, MaxContractPremium),
		mkTrade("TINY3", "CALL", 3, 0.10),
	}
	got := QualifyingPicks(in)
	if want := []string{"MAX1", "MAX2"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_NoRank1OnlyRanks2And3(t *testing.T) {
	in := []trades.Trade{
		mkTrade("RKLB", "CALL", 2, 1.69),
		mkTrade("IREN", "CALL", 3, 1.65),
	}
	got := QualifyingPicks(in)
	if want := []string{"RKLB", "IREN"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_SkipsNonPositiveEstimate(t *testing.T) {
	// EstimatedPrice == 0 means Claude couldn't quote it — don't trust
	// the cost-accounting if we'd build the order off a zero estimate.
	in := []trades.Trade{
		mkTrade("ZERO", "CALL", 1, 0),
		mkTrade("OK1", "CALL", 2, 1.00),
		mkTrade("OK2", "CALL", 3, 1.00),
	}
	got := QualifyingPicks(in)
	if want := []string{"OK1", "OK2"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_EmptyInput(t *testing.T) {
	if got := QualifyingPicks(nil); len(got) != 0 {
		t.Fatalf("nil input: want empty, got %v", symbols(got))
	}
	if got := QualifyingPicks([]trades.Trade{}); len(got) != 0 {
		t.Fatalf("empty slice: want empty, got %v", symbols(got))
	}
}

func TestQualifyingPicks_PutAtRank1IsEligible(t *testing.T) {
	in := []trades.Trade{mkTrade("HUBS", "PUT", 1, 1.18)}
	got := QualifyingPicks(in)
	if len(got) != 1 || got[0].ContractType != "PUT" {
		t.Fatalf("expected PUT pick selectable, got %v", got)
	}
}
