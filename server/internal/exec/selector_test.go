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
	// Rank-1 alone eats $300 — leaves $200 for ranks 2 and 3.
	// Rank-2 = $250 doesn't fit; rank-3 = $90 does. Skip-and-continue.
	in := []trades.Trade{
		mkTrade("BIG", "CALL", 1, 3.00),
		mkTrade("MID", "CALL", 2, 2.50),
		mkTrade("SML", "CALL", 3, 0.90),
	}
	got := QualifyingPicks(in)
	if want := []string{"BIG", "SML"}; !reflect.DeepEqual(symbols(got), want) {
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
	// Rank-1 = $5 cap = $500 exposure → eats the whole basket.
	// Ranks 2 and 3 cannot fit anything more, even tiny.
	in := []trades.Trade{
		mkTrade("MAX", "CALL", 1, MaxContractPremium),
		mkTrade("TINY1", "CALL", 2, 0.10),
		mkTrade("TINY2", "CALL", 3, 0.10),
	}
	got := QualifyingPicks(in)
	if want := []string{"MAX"}; !reflect.DeepEqual(symbols(got), want) {
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
