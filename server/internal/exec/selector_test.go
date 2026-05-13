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
	// Rank-1 is privileged (own cap, exempt from basket). Test the
	// skip-and-continue logic on the rank-2/3 tail where the basket
	// budget applies: rank-2 = $450 eats $450 of the $500 basket, so
	// rank-3 at $90 ($60 left) is skipped.
	in := []trades.Trade{
		mkTrade("R1", "CALL", 1, 0.30),
		mkTrade("BIG2", "CALL", 2, 4.50),
		mkTrade("SML", "CALL", 3, 0.90),
	}
	got := QualifyingPicks(in)
	if want := []string{"R1", "BIG2"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_SkipsAboveRank1Cap(t *testing.T) {
	// Rank-1 above MaxRank1ContractPremium is dropped. Tail picks
	// (eligible at MaxContractPremium) still run.
	in := []trades.Trade{
		mkTrade("HUGE", "CALL", 1, MaxRank1ContractPremium+0.01),
		mkTrade("OK1", "CALL", 2, 1.00),
		mkTrade("OK2", "CALL", 3, 1.00),
	}
	got := QualifyingPicks(in)
	if want := []string{"OK1", "OK2"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_SkipsAboveTailCapButRank1Survives(t *testing.T) {
	// Ranks 2 and 3 above MaxContractPremium are skipped; rank-1
	// stays in even at an elevated premium that ranks 2-3 couldn't take.
	in := []trades.Trade{
		mkTrade("R1HI", "CALL", 1, 7.00), // over base cap, under rank-1 cap
		mkTrade("BIG2", "CALL", 2, MaxContractPremium+0.01),
		mkTrade("OK3", "CALL", 3, 1.00),
	}
	got := QualifyingPicks(in)
	if want := []string{"R1HI", "OK3"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_Rank1IsExemptFromBasketBudget(t *testing.T) {
	// Rank-1 at $8/share = $800 exposure: would blow the $500 basket
	// if it consumed it. Privileging means rank-2 and rank-3 still get
	// the full $500 basket after rank-1 lands.
	in := []trades.Trade{
		mkTrade("R1", "CALL", 1, 8.00),
		mkTrade("R2", "CALL", 2, 2.50),
		mkTrade("R3", "CALL", 3, 2.50),
	}
	got := QualifyingPicks(in)
	if want := []string{"R1", "R2", "R3"}; !reflect.DeepEqual(symbols(got), want) {
		t.Fatalf("want %v, got %v", want, symbols(got))
	}
}

func TestQualifyingPicks_BasketBudgetExhaustedByTail(t *testing.T) {
	// Rank-2 at the $5 cap eats the whole $500 basket. Rank-3 (even
	// at $0.10) is then skipped because the tail budget is gone.
	// Rank-1 still lands.
	in := []trades.Trade{
		mkTrade("R1", "CALL", 1, 1.00),
		mkTrade("MAX2", "CALL", 2, MaxContractPremium),
		mkTrade("TINY3", "CALL", 3, 0.10),
	}
	got := QualifyingPicks(in)
	if want := []string{"R1", "MAX2"}; !reflect.DeepEqual(symbols(got), want) {
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
