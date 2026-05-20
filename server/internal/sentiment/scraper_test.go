package sentiment

import (
	"math"
	"testing"
)

/*
TestMergeSourcesPreservesSignalAgainstZeroOpinionSources is the
regression test for the "Sentiment always reads 0.00" UI bug. A ticker
appears in Yahoo's day_losers list (-0.5 vote) AND in Finviz unusual-
volume AND in EDGAR 8-K catalysts. Pre-fix the cross-source merge did a
running average against the trailing zero-opinion sources and decayed
the final score to roughly -0.02, which the 2dp display rounded to
"0.00". Post-fix only the Yahoo source has sentimentSamples > 0, so the
final reads -0.5.
*/
func TestMergeSourcesPreservesSignalAgainstZeroOpinionSources(t *testing.T) {
	yahooMover := []TickerMention{
		{Symbol: "TGT", Mentions: 1, sentimentSum: -0.5, sentimentSamples: 1, Sources: []string{"yahoo-movers"}},
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
	if math.Abs(got.Sentiment-(-0.5)) > 1e-9 {
		t.Errorf("expected Sentiment=-0.5 (one negative vote, two no-opinion sources), got %f", got.Sentiment)
	}
}

/*
TestMergeSourcesAveragesAcrossMultipleSentimentVotes confirms the
mention-weighted average actually averages when multiple sources do
have an opinion: two Yahoo movers entries (-0.5 each, samples=2) plus
StockTwits trending (+0.3, samples=1). Final = (-1.0 + 0.3) / 3.
*/
func TestMergeSourcesAveragesAcrossMultipleSentimentVotes(t *testing.T) {
	yahoo := []TickerMention{
		{Symbol: "TGT", Mentions: 2, sentimentSum: -1.0, sentimentSamples: 2, Sources: []string{"yahoo-movers", "yahoo-movers"}},
	}
	stocktwits := []TickerMention{
		{Symbol: "TGT", Mentions: 2, sentimentSum: 0.3, sentimentSamples: 1, Sources: []string{"stocktwits-trending"}},
	}

	result := mergeSources(yahoo, stocktwits)

	if len(result) != 1 {
		t.Fatalf("expected 1 ticker, got %d", len(result))
	}
	expected := (-1.0 + 0.3) / 3.0
	if math.Abs(result[0].Sentiment-expected) > 1e-9 {
		t.Errorf("expected Sentiment=%f, got %f", expected, result[0].Sentiment)
	}
	if result[0].Mentions != 4 {
		t.Errorf("expected Mentions=4, got %d", result[0].Mentions)
	}
}

/*
TestMergeSourcesNoOpinionLeavesSentimentZero confirms that a ticker
appearing only in zero-opinion sources reads exactly 0.0 — those
sources don't synthesize a fake signal in either direction.
*/
func TestMergeSourcesNoOpinionLeavesSentimentZero(t *testing.T) {
	finviz := []TickerMention{
		{Symbol: "TGT", Mentions: 1, Sources: []string{"finviz-unusual-volume"}},
	}
	edgar := []TickerMention{
		{Symbol: "TGT", Mentions: 1, Sources: []string{"edgar-8k-catalyst"}},
	}

	result := mergeSources(finviz, edgar)

	if len(result) != 1 {
		t.Fatalf("expected 1 ticker, got %d", len(result))
	}
	if result[0].Sentiment != 0 {
		t.Errorf("expected Sentiment=0, got %f", result[0].Sentiment)
	}
	if result[0].Mentions != 2 {
		t.Errorf("expected Mentions=2, got %d", result[0].Mentions)
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
