package sentiment

import (
	"testing"
)

/*
TestMergeSourcesAccumulatesMentions confirms a ticker appearing across
several sources sums its mention count. A name in Yahoo movers AND
Finviz unusual-volume AND EDGAR 8-K catalysts should read Mentions=3,
since mention count is the trending-strength signal the picker reads.
*/
func TestMergeSourcesAccumulatesMentions(t *testing.T) {
	yahooMover := []TickerMention{
		{Symbol: "TGT", Mentions: 1, Sources: []string{"yahoo-movers"}},
	}
	finvizUnusual := []TickerMention{
		{Symbol: "TGT", Mentions: 1, Sources: []string{"finviz-unusual-volume"}},
	}
	edgarHit := []TickerMention{
		{Symbol: "TGT", Mentions: 1, Sources: []string{"edgar-8k-catalyst"}},
	}

	result := mergeSources(yahooMover, finvizUnusual, edgarHit)

	if len(result) != 1 {
		t.Fatalf("expected 1 ticker, got %d", len(result))
	}
	got := result[0]
	if got.Symbol != "TGT" {
		t.Fatalf("expected TGT, got %s", got.Symbol)
	}
	if got.Mentions != 3 {
		t.Errorf("expected Mentions=3, got %d", got.Mentions)
	}
}

/*
TestMergeSourcesSortsByMentions confirms the result list is ordered by
mention count descending — the picker reads the top N, so order is a
load-bearing contract.
*/
func TestMergeSourcesSortsByMentions(t *testing.T) {
	src := []TickerMention{
		{Symbol: "AAA", Mentions: 1},
		{Symbol: "BBB", Mentions: 5},
		{Symbol: "CCC", Mentions: 3},
	}

	result := mergeSources(src)

	if len(result) != 3 {
		t.Fatalf("expected 3 tickers, got %d", len(result))
	}
	if result[0].Symbol != "BBB" || result[1].Symbol != "CCC" || result[2].Symbol != "AAA" {
		t.Errorf("expected order [BBB, CCC, AAA], got [%s, %s, %s]", result[0].Symbol, result[1].Symbol, result[2].Symbol)
	}
}

/*
TestMergeSourcesAccumulatesSources confirms the Sources slice gets the
union from every contributor — both the "stocktwits-trending" tag and
the "yahoo-movers" tag should survive on a ticker that hit both.
*/
func TestMergeSourcesAccumulatesSources(t *testing.T) {
	a := []TickerMention{
		{Symbol: "TGT", Mentions: 1, Sources: []string{"yahoo-movers"}},
	}
	b := []TickerMention{
		{Symbol: "TGT", Mentions: 1, Sources: []string{"stocktwits-trending"}},
	}

	result := mergeSources(a, b)

	if len(result) != 1 {
		t.Fatalf("expected 1 ticker, got %d", len(result))
	}
	if len(result[0].Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(result[0].Sources))
	}
	have := map[string]bool{}
	for _, s := range result[0].Sources {
		have[s] = true
	}
	if !have["yahoo-movers"] || !have["stocktwits-trending"] {
		t.Errorf("expected both yahoo-movers and stocktwits-trending in Sources, got %v", result[0].Sources)
	}
}
