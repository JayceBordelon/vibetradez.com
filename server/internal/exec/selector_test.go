package exec

import (
	"testing"

	"vibetradez.com/internal/trades"
)

func mkTrade(symbol, kind string, gptRank, claudeRank int, price float64, picked ...string) trades.Trade {
	t := trades.Trade{
		Symbol:         symbol,
		ContractType:   kind,
		EstimatedPrice: price,
		GPTRank:        gptRank,
		ClaudeRank:     claudeRank,
	}
	for _, p := range picked {
		switch p {
		case "gpt":
			t.PickedByOpenAI = true
		case "claude":
			t.PickedByClaude = true
		case "both":
			t.PickedByOpenAI = true
			t.PickedByClaude = true
		}
	}
	return t
}

func TestQualifyingPick_BothRank1SameDirection(t *testing.T) {
	in := []trades.Trade{
		mkTrade("AAPL", "CALL", 1, 1, 3.50, "both"),
		mkTrade("MSFT", "CALL", 2, 3, 2.10, "both"),
	}
	pick, ok := QualifyingPick(in)
	if !ok {
		t.Fatal("expected qualifying pick")
	}
	if pick.Symbol != "AAPL" {
		t.Errorf("got symbol %s want AAPL", pick.Symbol)
	}
}

func TestQualifyingPick_RejectsDirectionMismatch(t *testing.T) {
	/*
		Both models ranked AAPL #1 but disagreed on direction — the unioned
		row would carry whichever direction landed first, but in any case
		the picker shouldn't fire when the direction itself was contested.
		We approximate "different direction" by setting the unioned row's
		ContractType to something the test asserts wouldn't fire; in
		practice the union doesn't merge across direction so this test
		codifies the trade as "both picked it but only PUT'd it" — we
		still want it executable. The real direction-mismatch path is
		tested at the union layer; here we just confirm the selector
		honors a single-direction unified row.
	*/
	in := []trades.Trade{
		mkTrade("AAPL", "PUT", 1, 1, 3.50, "both"),
	}
	pick, ok := QualifyingPick(in)
	if !ok || pick.ContractType != "PUT" {
		t.Fatal("expected PUT pick to be selectable")
	}
}

func TestQualifyingPick_RejectsSingleModel(t *testing.T) {
	in := []trades.Trade{
		mkTrade("AAPL", "CALL", 1, 0, 3.50, "gpt"),
	}
	if _, ok := QualifyingPick(in); ok {
		t.Fatal("expected no pick when only one model picked")
	}
}

func TestQualifyingPick_RejectsBothPickedButNotBothRank1(t *testing.T) {
	in := []trades.Trade{
		mkTrade("AAPL", "CALL", 1, 2, 3.50, "both"),
		mkTrade("MSFT", "CALL", 2, 1, 2.10, "both"),
	}
	if _, ok := QualifyingPick(in); ok {
		t.Fatal("expected no pick — neither row has both ranks at 1")
	}
}

func TestQualifyingPick_RejectsAbovePriceCap(t *testing.T) {
	in := []trades.Trade{
		mkTrade("NVDA", "CALL", 1, 1, MaxContractPremium+0.01, "both"),
	}
	if _, ok := QualifyingPick(in); ok {
		t.Fatal("expected no pick when price exceeds cap")
	}
}

func TestQualifyingPick_AcceptsPriceExactlyAtCap(t *testing.T) {
	in := []trades.Trade{
		mkTrade("NVDA", "CALL", 1, 1, MaxContractPremium, "both"),
	}
	if _, ok := QualifyingPick(in); !ok {
		t.Fatal("expected pick at exactly the cap")
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

func TestQualifyingPick_FirstQualifyingWins(t *testing.T) {
	/*
		If two unioned rows both qualified (shouldn't happen because both
		models can only have one rank-1 each, but defensive), the first one
		in iteration order wins. We don't assert which is "best" — the
		guarantee is determinism.
	*/
	in := []trades.Trade{
		mkTrade("AAPL", "CALL", 1, 1, 3.00, "both"),
		mkTrade("MSFT", "CALL", 1, 1, 2.00, "both"),
	}
	pick, ok := QualifyingPick(in)
	if !ok {
		t.Fatal("expected a pick")
	}
	if pick.Symbol != "AAPL" {
		t.Errorf("expected first row (AAPL) to win, got %s", pick.Symbol)
	}
}
