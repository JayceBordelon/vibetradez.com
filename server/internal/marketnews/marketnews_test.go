package marketnews

import "testing"

func TestParseRSS(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><rss><channel>
	<item><title> Apple hits record </title><link>https://ex.com/a</link><pubDate>Mon, 15 Jun 2026 13:00:00 +0000</pubDate></item>
	<item><title>Empty link still ok</title><link></link><pubDate>not-a-date</pubDate></item>
	<item><title>   </title><link>https://ex.com/blank</link></item>
	</channel></rss>`)

	items := parseRSS(body, "AAPL", "Yahoo Finance")
	if len(items) != 2 { // the all-whitespace title is dropped
		t.Fatalf("items: got %d want 2", len(items))
	}
	if items[0].Headline != "Apple hits record" || items[0].Symbol != "AAPL" || items[0].Source != "Yahoo Finance" {
		t.Fatalf("item0: %+v", items[0])
	}
	if items[0].Published.IsZero() {
		t.Fatalf("item0 pubDate should parse")
	}
	if !items[1].Published.IsZero() {
		t.Fatalf("item1 bad pubDate should be the zero time")
	}
}

func TestDedupNews(t *testing.T) {
	in := []NewsItem{
		{Headline: "Nvidia jumps on AI demand"},
		{Headline: "NVIDIA JUMPS ON AI DEMAND"}, // case-insensitive duplicate
		{Headline: "A different story entirely"},
	}
	if out := dedupNews(in); len(out) != 2 {
		t.Fatalf("dedup: got %d want 2", len(out))
	}
}

func TestParseTrendingRespectsLimitAndUppercases(t *testing.T) {
	body := []byte(`{"symbols":[{"symbol":"gme","title":"GameStop","watchlist_count":100},{"symbol":"AMC","title":"AMC","watchlist_count":50}]}`)
	out, err := parseTrending(body, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("limit: got %d want 1", len(out))
	}
	if out[0].Symbol != "GME" || out[0].Rank != 1 || out[0].WatchlistCount != 100 {
		t.Fatalf("trending0: %+v", out[0])
	}
}

func TestParseSentimentCountsTags(t *testing.T) {
	body := []byte(`{"messages":[
	  {"entities":{"sentiment":{"basic":"Bullish"}}},
	  {"entities":{"sentiment":{"basic":"Bullish"}}},
	  {"entities":{"sentiment":{"basic":"Bearish"}}},
	  {"entities":{"sentiment":null}},
	  {"entities":{}}
	]}`)

	s, err := parseSentiment(body, "TSLA")
	if err != nil {
		t.Fatal(err)
	}
	if s.MessageCount != 5 || s.BullishCount != 2 || s.BearishCount != 1 {
		t.Fatalf("counts: %+v", s)
	}
	if s.BullishPct != 66.7 { // 2 of 3 tagged
		t.Fatalf("bullish pct: got %v want 66.7", s.BullishPct)
	}
}
