package portfolio

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vibetradez.com/internal/marketnews"
	"vibetradez.com/internal/schwab"
)

/*
fakeReader / fakeExec stub the two interfaces. fakeExec records how many
broker calls actually happened so a test can assert that a refusal NEVER
reaches the broker (the whole point of the tool layer being the security
boundary).
*/
type fakeReader struct {
}

func (f *fakeReader) Snapshot(context.Context) (Snapshot, error) { return Snapshot{}, nil }
func (f *fakeReader) GetQuotes([]string) (map[string]schwab.StockQuote, error) {
	return map[string]schwab.StockQuote{}, nil
}
func (f *fakeReader) GetOptionChain(string, string, string, string, float64) (*schwab.OptionChain, error) {
	return &schwab.OptionChain{}, nil
}
func (f *fakeReader) GetDailyPriceHistory(string) (*schwab.PriceHistory, error) {
	return &schwab.PriceHistory{Candles: []schwab.Candle{{Close: 100, High: 100, Low: 100}}}, nil
}
func (f *fakeReader) GetFundamentals(string) (*schwab.Fundamentals, error) {
	return &schwab.Fundamentals{}, nil
}
func (f *fakeReader) RecentDecisions(int) ([]HistoryEntry, error) { return nil, nil }
func (f *fakeReader) RecentStances(int) ([]StanceEntry, error)    { return nil, nil }
func (f *fakeReader) OrderStatus(context.Context, string) (OrderStatus, error) {
	return OrderStatus{}, nil
}
func (f *fakeReader) PriorSession() (string, string, bool, error) { return "", "", false, nil }
func (f *fakeReader) TrackRecord(int) (TrackRecord, error)        { return TrackRecord{}, nil }
func (f *fakeReader) TickerNews([]string, int) ([]marketnews.NewsItem, error) {
	return nil, nil
}
func (f *fakeReader) TrendingTickers(int) ([]marketnews.TrendingTicker, error) {
	return nil, nil
}
func (f *fakeReader) SocialSentiment(string) (marketnews.Sentiment, error) {
	return marketnews.Sentiment{}, nil
}

type fakeExec struct {
	buyEquityCalls int
}

func (f *fakeExec) BuyEquity(context.Context, string, float64, float64) (string, int, error) {
	f.buyEquityCalls++
	return "ord-1", 101, nil
}
func (f *fakeExec) SellEquity(context.Context, string, float64, float64) (string, int, error) {
	return "ord-s", 102, nil
}
func (f *fakeExec) BuyOption(context.Context, string, string, string, float64, string, int, float64) (string, int, error) {
	return "ord-o", 103, nil
}
func (f *fakeExec) SellOption(context.Context, string, int, float64) (string, int, error) {
	return "ord-so", 104, nil
}
func (f *fakeExec) CancelOrder(context.Context, string) error { return nil }

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func isErrResult(s string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return false
	}
	_, ok := m["error"]
	return ok
}

// A buy beyond the settled cash must be refused at the tool layer and must
// NOT reach the broker.
func TestDispatch_RefusalNeverHitsBroker(t *testing.T) {
	reader := &fakeReader{}
	ex := &fakeExec{}
	snap := baseSnapshot() // $6,000 settled cash
	d := NewToolDispatcher(reader, ex, snap)

	// $6,200 stock order > the $6,000 settled cash.
	res := d.Dispatch(context.Background(), "buy_equity", mustJSON(t, map[string]any{
		"symbol": "MDT", "quantity": 124.0, "limit_price": 50.0, "rationale": "test",
	}))
	if !isErrResult(res) {
		t.Fatalf("expected a refusal, got: %s", res)
	}
	if ex.buyEquityCalls != 0 {
		t.Fatalf("refused order must not reach the broker, got %d calls", ex.buyEquityCalls)
	}
	if len(d.Decisions()) != 0 {
		t.Fatalf("refused order must not record a decision, got %d", len(d.Decisions()))
	}
}

// Successful buys decrement the working snapshot's settled cash, so
// cumulative buys are gated against the cash actually left, not the pristine
// session state. With the allocation caps gone, settled cash is the only
// cumulative gate, and allocation across names is unconstrained.
func TestDispatch_CumulativeBuysRunToSettledCash(t *testing.T) {
	reader := &fakeReader{}
	ex := &fakeExec{}
	snap := baseSnapshot() // $6,000 settled cash

	d := NewToolDispatcher(reader, ex, snap)

	// $3,000 + $2,800 + $200 spends exactly the $6,000 settled cash: all pass
	// (the old allocation caps would have refused the very first $3,000
	// single-name order).
	for i, buy := range []map[string]any{
		{"symbol": "AAA", "quantity": 200.0, "limit_price": 15.0, "rationale": "first"},
		{"symbol": "BBB", "quantity": 200.0, "limit_price": 14.0, "rationale": "second"},
		{"symbol": "CCC", "quantity": 20.0, "limit_price": 10.0, "rationale": "third"},
	} {
		if r := d.Dispatch(context.Background(), "buy_equity", mustJSON(t, buy)); isErrResult(r) {
			t.Fatalf("buy %d should pass, got: %s", i+1, r)
		}
	}
	// One more dollar must be refused: there is no settled cash left.
	r := d.Dispatch(context.Background(), "buy_equity", mustJSON(t, map[string]any{
		"symbol": "DDD", "quantity": 1.0, "limit_price": 10.0, "rationale": "fourth",
	}))
	if !isErrResult(r) || !strings.Contains(r, "settled cash") {
		t.Fatalf("fourth buy should be refused on settled cash, got: %s", r)
	}
	if ex.buyEquityCalls != 3 {
		t.Fatalf("exactly three buys should have reached the broker, got %d", ex.buyEquityCalls)
	}
}

func TestPriceSummary(t *testing.T) {
	// Strictly rising series so the 1-month return is positive and the last
	// close sits at the 52-week high (0% from high).
	candles := make([]schwab.Candle, 60)
	for i := range candles {
		px := 100 + float64(i)
		candles[i] = schwab.Candle{Open: px, High: px, Low: px, Close: px}
	}
	s := priceSummary("TEST", &schwab.PriceHistory{Candles: candles})
	if s["last_close"].(float64) != 159 {
		t.Fatalf("expected last_close 159, got %v", s["last_close"])
	}
	if s["pct_from_52w_high"].(float64) != 0 {
		t.Fatalf("expected 0%% from high at the top, got %v", s["pct_from_52w_high"])
	}
	if s["return_1m_pct"].(float64) <= 0 {
		t.Fatalf("expected positive 1m return on a rising series, got %v", s["return_1m_pct"])
	}
}

// hold records a continuation decision (by symbol) and never touches the broker.
func TestDispatch_Hold(t *testing.T) {
	d := NewToolDispatcher(&fakeReader{}, &fakeExec{}, baseSnapshot())
	res := d.Dispatch(context.Background(), "hold", mustJSON(t, map[string]any{"symbol": "MDT", "rationale": "continuing to hold, thesis intact"}))
	if isErrResult(res) {
		t.Fatalf("hold should succeed, got: %s", res)
	}
	decs := d.Decisions()
	if len(decs) != 1 || decs[0].Action != ActionHold || decs[0].Symbol != "MDT" {
		t.Fatalf("expected one MDT hold decision, got %+v", decs)
	}
	// hold without a symbol is rejected (it's only for continuing a held name).
	if !isErrResult(d.Dispatch(context.Background(), "hold", mustJSON(t, map[string]any{"rationale": "x"}))) {
		t.Fatal("hold without symbol should error")
	}
}
