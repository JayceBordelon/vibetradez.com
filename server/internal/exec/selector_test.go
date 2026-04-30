package exec

import (
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

func TestQualifyingPick_Rank1UnderCap(t *testing.T) {
	in := []trades.Trade{
		mkTrade("AAPL", "CALL", 1, 3.50),
		mkTrade("MSFT", "CALL", 2, 2.10),
	}
	pick, ok := QualifyingPick(in)
	if !ok {
		t.Fatal("expected qualifying pick")
	}
	if pick.Symbol != "AAPL" {
		t.Errorf("got symbol %s want AAPL", pick.Symbol)
	}
}

func TestQualifyingPick_RejectsNonRank1(t *testing.T) {
	in := []trades.Trade{
		mkTrade("AAPL", "CALL", 2, 3.50),
		mkTrade("MSFT", "CALL", 3, 2.10),
	}
	if _, ok := QualifyingPick(in); ok {
		t.Fatal("expected no pick when no trade is rank 1")
	}
}

func TestQualifyingPick_RejectsAbovePriceCap(t *testing.T) {
	in := []trades.Trade{
		mkTrade("NVDA", "CALL", 1, MaxContractPremium+0.01),
	}
	if _, ok := QualifyingPick(in); ok {
		t.Fatal("expected no pick when price exceeds cap")
	}
}

func TestQualifyingPick_AcceptsPriceExactlyAtCap(t *testing.T) {
	in := []trades.Trade{
		mkTrade("NVDA", "CALL", 1, MaxContractPremium),
	}
	if _, ok := QualifyingPick(in); !ok {
		t.Fatal("expected pick at exactly the cap")
	}
}

func TestQualifyingPick_AcceptsPutAtRank1(t *testing.T) {
	in := []trades.Trade{
		mkTrade("AAPL", "PUT", 1, 3.50),
	}
	pick, ok := QualifyingPick(in)
	if !ok || pick.ContractType != "PUT" {
		t.Fatal("expected PUT pick to be selectable")
	}
}

func TestQualifyingPick_EmptyInput(t *testing.T) {
	if _, ok := QualifyingPick(nil); ok {
		t.Fatal("expected no pick from nil input")
	}
	if _, ok := QualifyingPick([]trades.Trade{}); ok {
		t.Fatal("expected no pick from empty input")
	}
}
